package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"path/filepath"

	"github.com/kongkkongtx/ai-gateway/internal/rag"
)

// RAGMiddleware intercepts chat requests and injects retrieved knowledge
// base context into the prompt before it reaches the LLM.
type RAGMiddleware struct {
	cfg     rag.Config
	store   rag.VectorStore
	embedder rag.Embedder
	logger  *slog.Logger
}

// NewRAG creates a new RAG middleware.
func NewRAG(cfg rag.Config, store rag.VectorStore, embedder rag.Embedder, logger *slog.Logger) *RAGMiddleware {
	if cfg.TopK <= 0 {
		cfg.TopK = 5
	}
	if cfg.ScoreThreshold <= 0 {
		cfg.ScoreThreshold = 0.75
	}
	if cfg.MaxContextTokens <= 0 {
		cfg.MaxContextTokens = 2000
	}
	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = "You are a helpful AI assistant. Answer questions using ONLY the provided context."
	}
	return &RAGMiddleware{
		cfg:      cfg,
		store:    store,
		embedder: embedder,
		logger:   logger,
	}
}

// Middleware returns an http.Handler that performs RAG retrieval and injection.
func (m *RAGMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
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

		queryText := extractQueryText(reqMap)
		if queryText == "" {
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}

		// Determine which KB to use based on route match
		model, _ := reqMap["model"].(string)
		kbID := m.findKBID(model)
		if kbID == "" {
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}

		// Get overrides for this route
		topK, scoreThreshold := m.getRouteOverrides(model)

		// Embed the query
		queryVector, err := m.embedder.Embed(queryText)
		if err != nil {
			m.logger.Warn("rag: embedding failed", "error", err)
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}

		// Search vector store
		results, err := m.store.Search(r.Context(), queryVector, topK, kbID)
		if err != nil {
			m.logger.Warn("rag: search failed", "error", err)
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}

		// Filter by score threshold
		var filtered []rag.SearchResult
		for _, res := range results {
			if res.Score >= scoreThreshold {
				filtered = append(filtered, res)
			}
		}

		if len(filtered) == 0 {
			m.logger.Debug("rag: no relevant results", "query", truncate(queryText, 50))
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}

		m.logger.Info("rag: context injected",
			"results", len(filtered),
			"top_score", fmt.Sprintf("%.4f", filtered[0].Score),
			"model", model,
			"kb", kbID,
		)

		// Build context block and inject into messages
		contextBlock := buildContextBlock(filtered)
		modifiedMessages := injectContext(reqMap, contextBlock, queryText)

		reqMap["messages"] = modifiedMessages
		reqMap["system_prompt"] = m.cfg.SystemPrompt

		newBody, err := json.Marshal(reqMap)
		if err != nil {
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}

		r.Body = io.NopCloser(bytes.NewReader(newBody))
		next.ServeHTTP(w, r)
	})
}

// findKBID matches the model against route overrides to find the KB ID.
// Falls back to the first configured KB if no route-specific match.
func (m *RAGMiddleware) findKBID(model string) string {
	for _, ro := range m.cfg.RouteOverrides {
		if !ro.Enabled {
			continue
		}
		matched, err := filepath.Match(ro.RouteMatch, model)
		if err == nil && matched {
			// Return the first KB's ID as default for this route
			if len(m.cfg.KnowledgeBases) > 0 {
				return m.cfg.KnowledgeBases[0].ID
			}
			return ""
		}
	}
	// No route match: return first KB if there are no overrides, or empty
	if len(m.cfg.RouteOverrides) == 0 && len(m.cfg.KnowledgeBases) > 0 {
		return m.cfg.KnowledgeBases[0].ID
	}
	return ""
}

// getRouteOverrides returns RAG parameters for the given model.
func (m *RAGMiddleware) getRouteOverrides(model string) (topK int, scoreThreshold float64) {
	topK = m.cfg.TopK
	scoreThreshold = m.cfg.ScoreThreshold
	for _, ro := range m.cfg.RouteOverrides {
		if !ro.Enabled {
			continue
		}
		matched, err := filepath.Match(ro.RouteMatch, model)
		if err == nil && matched {
			if ro.TopK > 0 {
				topK = ro.TopK
			}
			if ro.ScoreThreshold > 0 {
				scoreThreshold = ro.ScoreThreshold
			}
			return
		}
	}
	return
}

// buildContextBlock builds an XML-wrapped context string from search results.
func buildContextBlock(results []rag.SearchResult) string {
	var b strings.Builder
	b.WriteString("<context>\n")
	for i, r := range results {
		source := ""
		if title, ok := r.Chunk.Metadata["title"]; ok {
			source = fmt.Sprintf(` title="%s"`, title)
		}
		b.WriteString(fmt.Sprintf(`<source id="%d"%s>`, i+1, source))
		b.WriteString("\n")
		b.WriteString(strings.TrimSpace(r.Chunk.Content))
		b.WriteString("\n")
		b.WriteString("</source>\n")
	}
	b.WriteString("</context>")
	return b.String()
}

// injectContext replaces the last user message with context + query.
func injectContext(reqMap map[string]interface{}, contextBlock, queryText string) []interface{} {
	messages, _ := reqMap["messages"].([]interface{})
	if messages == nil {
		return nil
	}

	// Create the augmented user message
	augmented := fmt.Sprintf("%s\n\n<query>\n%s\n</query>\n\nReminder: Only answer questions based on the information available within the <context> tags.", contextBlock, queryText)

	result := make([]interface{}, len(messages))
	copy(result, messages)

	// Replace the last user message with the augmented one
	for i := len(result) - 1; i >= 0; i-- {
		msg, ok := result[i].(map[string]interface{})
		if ok && msg["role"] == "user" {
			result[i] = map[string]interface{}{
				"role":    "user",
				"content": augmented,
			}
			break
		}
	}

	return result
}

// truncate shortens a string for logging.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
