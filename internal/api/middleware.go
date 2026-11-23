package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
	"runtime/debug"
)

type requestIDContextKey struct{}

func requestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDContextKey{}).(string)
	return id
}

type statusWriter struct {
	http.ResponseWriter
	status int
}
func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}
func (w *statusWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(p)
}

func requestID(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 12)
		if _, err := rand.Read(b); err != nil {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		id := hex.EncodeToString(b)
		w.Header().Set("X-Request-ID", id)
		start := time.Now()
		tracked := &statusWriter{ResponseWriter: w}
		r = r.WithContext(context.WithValue(r.Context(), requestIDContextKey{}, id))
		next.ServeHTTP(tracked, r)
		if tracked.status == 0 {
			tracked.status = http.StatusOK
		}
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		logger.Info("http request", "request_id", id, "method", r.Method,
			"route", route, "status", tracked.status, "duration_ms", time.Since(start).Milliseconds())
	})
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}
func recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if v := recover(); v != nil {
					logger.Error("panic", "request_id", requestIDFromContext(r.Context()), "panic", v, "stack", string(debug.Stack()))
					fail(w, 500, "internal error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
