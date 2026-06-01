package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/yushi/ai-gateway/internal/semantic"
)

// SemanticMiddleware provides semantic routing and semantic caching for
// incoming AI requests, using embedding similarity to route to the best
// model and cache responses for semantically similar queries.
type SemanticMiddleware struct {
	router *semantic.Router
	cache  *semantic.Cache
	logger *slog.Logger
}

// NewSemantic creates a semantic middleware.
// Both router and cache are optional; pass nil for either to disable it.
func NewSemantic(router *semantic.Router, cache *semantic.Cache, logger *slog.Logger) *SemanticMiddleware {
	return &SemanticMiddleware{
		router: router,
		cache:  cache,
		logger: logger,
	}
}

// Middleware returns an http.Handler that performs semantic operations.
func (m *SemanticMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			next.ServeHTTP(w, r)
			return
		}
		if m.router == nil && m.cache == nil {
			next.ServeHTTP(w, r)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		var reqMap map[string]interface{}
		if err := json.Unmarshal(body, &reqMap); err != nil {
			next.ServeHTTP(w, r)
			return
		}

		// Extract query text from the last user message
		queryText := extractQueryText(reqMap)
		if queryText == "" {
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}

		// Step 1: Semantic routing — override the model field
		if m.router != nil {
			match, score, err := m.router.Match(queryText)
			if err == nil && match != nil {
				m.logger.Info("semantic route matched",
					"category", match.Name,
					"score", score,
					"target", match.Target,
					"original_model", reqMap["model"])
				reqMap["model"] = match.Target
				modified, err := json.Marshal(reqMap)
				if err == nil {
					body = modified
				}
			}
		}

		// Step 2: Semantic cache lookup (non-streaming only)
		if m.cache != nil {
			if stream, _ := reqMap["stream"].(bool); !stream {
				if cachedResp, score, err := m.cache.Get(queryText, 0.92); err == nil && cachedResp != nil {
					m.logger.Info("semantic cache hit", "score", score)
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("X-Cache", "HIT")
					w.Write(cachedResp)
					return
				}
			}
		}

		// Step 3: Wrap response writer to cache the response
		r.Body = io.NopCloser(bytes.NewReader(body))
		if m.cache != nil {
			stream, _ := reqMap["stream"].(bool)
			wrapped := &semanticResponseWriter{
				ResponseWriter: w,
				cache:          m.cache,
				queryText:      queryText,
				model:          reqMap["model"].(string),
				streaming:      stream,
				body:           &bytes.Buffer{},
			}
			next.ServeHTTP(wrapped, r)
			wrapped.flushCache()
		}

		next.ServeHTTP(w, r)
	})
}

// semanticResponseWriter captures the response body for caching.
type semanticResponseWriter struct {
	http.ResponseWriter
	cache     *semantic.Cache
	queryText string
	model     string
	streaming bool
	body      *bytes.Buffer
}

func (w *semanticResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// extractQueryText finds the last user message from the request map.
func extractQueryText(reqMap map[string]interface{}) string {
	messages, ok := reqMap["messages"].([]interface{})
	if !ok {
		return ""
	}
	for i := len(messages) - 1; i >= 0; i-- {
		msg, ok := messages[i].(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		if role == "user" {
			content, _ := msg["content"].(string)
			return content
		}
	}
	return ""
}

// flushCache stores the captured response in the semantic cache.
func (w *semanticResponseWriter) flushCache() {
	if w.cache == nil || w.streaming || w.body.Len() == 0 || w.queryText == "" {
		return
	}
	w.cache.Set(w.queryText, w.body.Bytes(), w.model)
}
