package server

import (
	"context"
	"crypto/rand"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type requestLoggerKey struct{}

func requestLogger(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(requestLoggerKey{}).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

type requestLogWriter struct {
	middleware.WrapResponseWriter
	errorCode string
}

func (w *requestLogWriter) setErrorCode(code string) {
	w.errorCode = code
}

// requestLogging records metadata only, never request/response bodies or headers.
func requestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := rand.Text()
		logger := slog.Default().With("request_id", id)
		r = r.WithContext(context.WithValue(r.Context(), requestLoggerKey{}, logger))
		w.Header().Set("X-Request-ID", id)
		ww := &requestLogWriter{WrapResponseWriter: middleware.NewWrapResponseWriter(w, r.ProtoMajor)}
		panicked := false
		defer func() {
			if recovered := recover(); recovered != nil {
				panicked = true
				ww.errorCode = "handler_panic"
				if recovered == http.ErrAbortHandler {
					ww.errorCode = "request_aborted"
					defer func() { panic(http.ErrAbortHandler) }()
				} else if ww.Status() == 0 {
					writeAPIError(ww, http.StatusInternalServerError, "handler_panic", "Internal server error")
				} else {
					// A response has already begun; abort rather than send a second response.
					defer func() { panic(http.ErrAbortHandler) }()
				}
			}
			status := ww.Status()
			if status == 0 && !panicked {
				status = http.StatusOK
			}
			route := "unmatched"
			if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
				route = rc.RoutePattern()
			}
			level := slog.LevelInfo
			switch {
			case panicked || status >= 500:
				level = slog.LevelError
			case status >= 400:
				level = slog.LevelWarn
			}
			attrs := []any{"method", r.Method, "route", route, "status", status,
				"duration_ms", float64(time.Since(start).Microseconds()) / 1000,
				"response_bytes", ww.BytesWritten()}
			if ww.errorCode != "" {
				attrs = append(attrs, "error_code", ww.errorCode)
			}
			logger.Log(r.Context(), level, "http_request", attrs...)
		}()
		next.ServeHTTP(ww, r)
	})
}
