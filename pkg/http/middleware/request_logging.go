package middleware

import (
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/yurykabanov/nomad-deployer/pkg"
)

type statusWriter struct {
	http.ResponseWriter
	status int
	length int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.length += n
	return n, err
}

func WithRequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startAt := time.Now()
		sw := statusWriter{ResponseWriter: w, status: http.StatusOK}

		defer func() {
			ctx := r.Context()

			logger := ctx.Value(pkg.ContextLoggerKey).(log.FieldLogger)

			logger.WithFields(log.Fields{
				"host":           r.Host,
				"remote_addr":    r.RemoteAddr,
				"method":         r.Method,
				"request_uri":    r.RequestURI,
				"status":         sw.status,
				"content_length": sw.length,
				"user_agent":     r.UserAgent(),
				"duration_ns":    time.Now().Sub(startAt).Nanoseconds(),
			}).Info("request")
		}()

		next.ServeHTTP(&sw, r)
	})
}
