// Package middleware holds the HTTP middleware chain: structured request
// logging, JWT auth, and the append-only activity audit wrapper.
package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// Logging emits a structured slog access log per request, including the chi
// request id, method, path, status, bytes, and latency.
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			defer func() {
				logger.Info("request",
					"id", middleware.GetReqID(r.Context()),
					"method", r.Method,
					"path", r.URL.Path,
					"status", ww.Status(),
					"bytes", ww.BytesWritten(),
					"latency_ms", time.Since(start).Milliseconds(),
				)
			}()
			next.ServeHTTP(ww, r)
		})
	}
}
