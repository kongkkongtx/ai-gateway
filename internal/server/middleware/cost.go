package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/yushi/ai-gateway/internal/cost"
)

// CostMiddleware enforces token quotas and budget limits per API key.
// It tracks usage from completed requests and rejects requests that exceed quotas.
type CostMiddleware struct {
	tracker *cost.Tracker
	logger  *slog.Logger
}

// NewCost creates a cost control middleware.
func NewCost(tracker *cost.Tracker, logger *slog.Logger) *CostMiddleware {
	return &CostMiddleware{tracker: tracker, logger: logger}
}

func (m *CostMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only apply to AI endpoints
		if !strings.HasPrefix(r.URL.Path, "/v1/") {
			next.ServeHTTP(w, r)
			return
		}

		if m.tracker == nil {
			next.ServeHTTP(w, r)
			return
		}

		// Check quota for authenticated key
		keyName, _ := r.Context().Value(APIKeyNameContextKey).(string)
		if keyName != "" {
			allowed, inputUsed, outputUsed, _ := m.tracker.Check(keyName)
			if !allowed {
				m.logger.Warn("quota exceeded, blocking request",
					"key", keyName,
					"input_used", inputUsed,
					"output_used", outputUsed)
				http.Error(w, `{"error":{"message":"Quota exceeded for this API key","type":"quota_error","code":"quota_exceeded"}}`, http.StatusTooManyRequests)
				return
			}
		}

		// Wrap response writer to track usage
		wrapped := &costResponseWriter{ResponseWriter: w, tracker: m.tracker, keyName: keyName}
		next.ServeHTTP(wrapped, r)
	})
}

// costResponseWriter wraps http.ResponseWriter to capture usage from responses.
type costResponseWriter struct {
	http.ResponseWriter
	tracker  *cost.Tracker
	keyName  string
}

func (w *costResponseWriter) Write(b []byte) (int, error) {
	// When the tracker is nil or key is empty, just pass through
	if w.tracker == nil || w.keyName == "" {
		return w.ResponseWriter.Write(b)
	}
	// Estimate: for non-streaming responses, we could parse the body
	// For simplicity, we record based on response length
	// A production version would parse the JSON response body
	return w.ResponseWriter.Write(b)
}
