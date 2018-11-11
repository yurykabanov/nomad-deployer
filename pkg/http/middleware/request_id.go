package middleware

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/yurykabanov/nomad-deployer/pkg"
)

func WithRequestId(next http.Handler, nextRequestId func() string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestId := r.Header.Get("X-Request-Id")

		if requestId == "" {
			requestId = nextRequestId()
		}

		ctx := r.Context()
		if requestId != "" {
			ctx = context.WithValue(r.Context(), pkg.ContextRequestIdKey, requestId)
		}

		w.Header().Set("X-Request-Id", requestId)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func DefaultRequestIdProvider() string {
	var buf = make([]byte, 16)
	_, err := rand.Read(buf)
	if err != nil {
		log.WithError(err).Error("Unable to generate request id")
		return ""
	}
	return fmt.Sprintf("%02x", buf)
}
