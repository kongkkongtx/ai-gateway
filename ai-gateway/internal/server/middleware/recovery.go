package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recovery catches panics in downstream handlers and returns a structured error response.
// Prevents the gateway process from crashing due to unexpected panics.
type Recovery struct {
	logger *slog.Logger
}

// NewRecovery creates a Recovery middleware instance.
func NewRecovery(logger *slog.Logger) *Recovery {
	return &Recovery{logger: logger}
}

// Middleware returns an HTTP handler that recovers from panics.
// Logs the panic details and stack trace, then returns a 500 error with an OpenAI-compatible body.
func (r *Recovery) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				r.logger.Error("panic recovered",
					"panic", rec,
					"stack", string(debug.Stack()),
					"path", req.URL.Path,
				)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error": map[string]string{
						"message": "Internal server error",
						"type":    "server_error",
						"code":    "internal_error",
					},
				})
			}
		}()
		next.ServeHTTP(w, req)
	})
}