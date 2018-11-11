package middleware

import (
	"context"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/yurykabanov/nomad-deployer/pkg"
)

func WithLogger(next http.Handler, logger log.FieldLogger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		requestId, ok := ctx.Value(pkg.ContextRequestIdKey).(string)
		if !ok {
			ctx = context.WithValue(r.Context(), pkg.ContextLoggerKey, logger.WithField("request_id", nil))
		} else {
			ctx = context.WithValue(r.Context(), pkg.ContextLoggerKey, logger.WithField("request_id", requestId))
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
