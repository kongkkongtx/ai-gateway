package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture the status code for audit logging.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.statusCode = http.StatusOK
		rw.written = true
	}
	return rw.ResponseWriter.Write(b)
}

// Audit logs structured request/response information for every request.
// Includes method, path, status code, duration, remote address, and authenticated key name.
// Error responses (5xx) are logged at ERROR level, client errors (4xx) at WARN level.
type Audit struct {
	logger *slog.Logger
}

// NewAudit creates an Audit middleware instance.
func NewAudit(logger *slog.Logger) *Audit {
	return &Audit{logger: logger}
}

// Middleware returns an HTTP handler that logs request details after the response is written.
func (a *Audit) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		duration := time.Since(start)
		keyName, _ := r.Context().Value(APIKeyNameContextKey).(string)
		attrs := []slog.Attr{
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rw.statusCode),
			slog.Duration("duration", duration),
			slog.String("remote", r.RemoteAddr),
		}
		if keyName != "" {
			attrs = append(attrs, slog.String("api_key", keyName))
		}
		if rw.statusCode >= 500 {
			a.logger.LogAttrs(nil, slog.LevelError, "request failed", attrs...)
		} else if rw.statusCode >= 400 {
			a.logger.LogAttrs(nil, slog.LevelWarn, "request warning", attrs...)
		} else {
			a.logger.LogAttrs(nil, slog.LevelInfo, "request completed", attrs...)
		}
	})
}