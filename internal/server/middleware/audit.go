package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// Context keys for passing AI-specific metadata to the audit middleware.
const (
	ModelContextKey     contextKey = "audit_model"
	TokensInContextKey  contextKey = "audit_tokens_in"
	TokensOutContextKey contextKey = "audit_tokens_out"
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
type Audit struct {
	logger *slog.Logger
	store  AuditStore
}

// NewAudit creates an Audit middleware instance.
func NewAudit(logger *slog.Logger, store AuditStore) *Audit {
	return &Audit{logger: logger, store: store}
}

// Middleware returns an HTTP handler that logs request details after the response is written.
func (a *Audit) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		duration := time.Since(start)

		var level string
		if rw.statusCode >= 500 {
			level = "error"
		} else if rw.statusCode >= 400 {
			level = "warn"
		} else {
			level = "info"
		}

		keyName, _ := r.Context().Value(APIKeyNameContextKey).(string)

		// Read AI-specific metadata set by handlers
		model, _ := r.Context().Value(ModelContextKey).(string)
		tokensIn, _ := r.Context().Value(TokensInContextKey).(int)
		tokensOut, _ := r.Context().Value(TokensOutContextKey).(int)

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
		if model != "" {
			attrs = append(attrs, slog.String("model", model))
		}

		if a.store != nil {
			a.store.Push(AuditEntry{
				Timestamp:  start,
				Method:     r.Method,
				Path:       r.URL.Path,
				Status:     rw.statusCode,
				Duration:   duration.Round(time.Microsecond).String(),
				RemoteAddr: r.RemoteAddr,
				APIKeyName: keyName,
				Level:      level,
				Model:      model,
				TokensIn:   tokensIn,
				TokensOut:  tokensOut,
			})
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
