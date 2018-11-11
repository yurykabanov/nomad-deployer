package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/yurykabanov/nomad-deployer/pkg/domain"
	"github.com/yurykabanov/nomad-deployer/pkg/domain/redeploy"
	"github.com/yurykabanov/nomad-deployer/pkg/http/handler"
	"github.com/yurykabanov/nomad-deployer/pkg/http/middleware"
	"github.com/yurykabanov/nomad-deployer/pkg/nomad"
	"github.com/yurykabanov/nomad-deployer/pkg/storage/memory"
)

const (
	configLogLevel  = "log.level"
	configLogFormat = "log.format"

	configServerAddress         = "server.address"
	configServerTimeoutRead     = "server.timeout.read"
	configServerTimeoutWrite    = "server.timeout.write"
	configServerShutdownTimeout = "server.shutdown.timeout"
	configServerLogRequests     = "server.log.requests"

	configNomadUrl = "nomad.url"

	configJobs = "jobs"
)

var (
	Build   = "unknown"
	Version = "unknown"
)

func loadConfiguration() {
	// Verbose is a shortcut for `log.level = debug`
	viper.SetDefault("verbose", false)
	pflag.BoolP("verbose", "v", false, "Shortcut for verbose logs (debug level)")

	// Config file flag
	pflag.StringP("config", "c", "", "Config file")

	pflag.String(configLogLevel, "info", "Log level")
	pflag.String(configLogFormat, "json", "Log output format")

	// HTTP server config
	pflag.String(configServerAddress, "0.0.0.0:8000", "HTTP server bind address")
	pflag.Duration("server.timeout.read", 5*time.Second, "HTTP server read timeout")
	pflag.Duration("server.timeout.write", 10*time.Second, "HTTP server write timeout")
	pflag.Duration("server.shutdown.timeout", 30*time.Second, "HTTP server graceful shutdown timeout")
	pflag.Bool("server.log.requests", true, "HTTP server request logging")

	// Nomad config
	pflag.String(configNomadUrl, "http://127.0.0.1:4646", "Nomad API base URL")

	pflag.Parse()

	// NOTE: we don't have logger configured yet as we haven't read all sources of configuration
	// so we're using default logrus logger as fallback
	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		logrus.WithError(err).Fatal("Couldn't bind flags")
	}

	// Read config from environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("deployer")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read config from config file
	if configFile := viper.GetString("config"); configFile != "" {
		// If user do specify config file, then this file MUST exist and be valid
		// so missing file is a fatal error

		viper.SetConfigFile(configFile)

		if err := viper.ReadInConfig(); err != nil {
			logrus.WithError(err).Fatal("Couldn't read config file")
		}
	} else {
		// If user does not specify config file, then we'll still try to find appropriate config,
		// but missing file is not an error

		viper.SetConfigName("deployer")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")
		viper.AddConfigPath("/etc/nomad-deployer")

		if err := viper.ReadInConfig(); err != nil {
			logrus.WithError(err).Warn("Couldn't read config file")
		}
	}
}

func mustCreateLoggers() (logrus.FieldLogger, *log.Logger) {
	// logrus logger is used anywhere throughout the app
	logrusLogger := logrus.StandardLogger()

	level, err := logrus.ParseLevel(viper.GetString(configLogLevel))
	if err != nil {
		level = logrus.InfoLevel
	}

	logrusLogger.SetLevel(level)

	switch viper.GetString(configLogFormat) {
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{})
	default:
	case "text":
		logrus.SetFormatter(&logrus.TextFormatter{})
	}

	loggerWriter := logrusLogger.Writer()
	// NOTE: loggerWriter is never closed, but logger is supposed to live until application is closed, so this is fine

	// default logger writer is used as sink for `http.Server`'s `ErrorLog`
	defaultLogger := log.New(loggerWriter, "", 0)

	return logrusLogger, defaultLogger
}

func main() {
	loadConfiguration()

	// we have to create both logrus logger and adapter of default golang logger specially for http.Server
	logger, httpErrorLogger := mustCreateLoggers()

	logger.WithFields(logrus.Fields{
		"build":   Build,
		"version": Version,
	}).Info("Nomad Deployer is starting...")

	// As for now there is no storage for repository <-> jobs mapping, they are read from config
	var mapping = make(map[string][]domain.Job)
	for repository, jobNames := range viper.GetStringMapStringSlice(configJobs) {
		for _, jobName := range jobNames {
			mapping[repository] = append(mapping[repository], domain.Job{Name: jobName})
		}
	}

	// Services and dependencies
	jobsRepository := memory.NewJobsRepository(mapping)
	nomadClient := nomad.NewClient(viper.GetString(configNomadUrl))
	redeploySvc := redeploy.NewRedeployService(nomadClient)

	// HTTP router, handlers and middleware
	router := mux.NewRouter()

	healthHandler := handler.NewHealthHandler()
	registryCallbackHandler := handler.NewRegistryCallbackHandler(jobsRepository, redeploySvc)

	router.Handle("/health", healthHandler)
	router.Handle("/registry/callback", registryCallbackHandler)

	var httpHandler http.Handler = router

	if viper.GetBool(configServerLogRequests) {
		httpHandler = middleware.WithRequestLogging(httpHandler)
	}
	httpHandler = middleware.WithLogger(httpHandler, logger)
	httpHandler = middleware.WithRequestId(httpHandler, middleware.DefaultRequestIdProvider)

	addr := viper.GetString(configServerAddress)

	// HTTP server
	server := &http.Server{
		Addr:         addr,
		Handler:      httpHandler,
		ErrorLog:     httpErrorLogger,
		ReadTimeout:  viper.GetDuration(configServerTimeoutRead),
		WriteTimeout: viper.GetDuration(configServerTimeoutWrite),
	}

	// Shutdown notification channels
	done := make(chan struct{})
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	// Graceful shutdown
	go func() {
		<-quit
		logger.Info("Nomad Deployer is shutting down...")
		healthHandler.SetHealth(false)

		ctx, cancel := context.WithTimeout(context.Background(), viper.GetDuration(configServerShutdownTimeout))
		defer cancel()

		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(ctx); err != nil {
			logger.WithError(err).Fatal("Could not gracefully shutdown the server")
		}
		close(done)
	}()

	// Run HTTP server
	logger.Infof("Server is ready to handle requests at %s", addr)
	healthHandler.SetHealth(true)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.WithError(err).Fatalf("Could not listen on %s", addr)
	}

	// Wait for graceful shutdown
	<-done
	logger.Info("Nomad Deployer stopped")
}
