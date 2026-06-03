package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/kongkkongtx/ai-gateway/internal/cost"
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
		if !strings.HasPrefix(r.URL.Path, "/v1/") {
			next.ServeHTTP(w, r)
			return
		}
		if m.tracker == nil {
			next.ServeHTTP(w, r)
			return
		}

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

		wrapped := &costResponseWriter{
			ResponseWriter: w,
			tracker:        m.tracker,
			keyName:        keyName,
			path:           r.URL.Path,
			body:           &bytes.Buffer{},
		}
		next.ServeHTTP(wrapped, r)
		wrapped.flushUsage()
	})
}

// costResponseWriter wraps http.ResponseWriter to capture usage from responses,
// buffering the body so it can be parsed for token usage metadata.
type costResponseWriter struct {
	http.ResponseWriter
	tracker *cost.Tracker
	keyName string
	path    string
	body    *bytes.Buffer
}

func (w *costResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// flushUsage parses the buffered response body and records token usage.
func (w *costResponseWriter) flushUsage() {
	if w.tracker == nil || w.keyName == "" || w.body.Len() == 0 {
		return
	}
	if !strings.HasSuffix(w.path, "/chat/completions") {
		return
	}

	var resp struct {
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(w.body.Bytes(), &resp); err != nil {
		return
	}
	if resp.Usage == nil || (resp.Usage.PromptTokens == 0 && resp.Usage.CompletionTokens == 0) {
		return
	}
	w.tracker.Record(w.keyName, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
}