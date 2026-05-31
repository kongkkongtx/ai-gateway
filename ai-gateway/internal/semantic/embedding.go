package semantic

import (
	"github.com/yushi/ai-gateway/internal/provider/openai"
)

// EmbeddingClient implements the Embedder interface using a provider's
// OpenAI-compatible embedding API.
type EmbeddingClient struct {
	adapter *openai.Adapter
	model   string
}

// NewEmbeddingClient creates an embedding client that calls the given adapter.
func NewEmbeddingClient(adapter *openai.Adapter, model string) *EmbeddingClient {
	if model == "" {
		model = "text-embedding-3-small"
	}
	return &EmbeddingClient{adapter: adapter, model: model}
}

// Embed computes the embedding for a text string.
// If the adapter provides usage info (token count), it''s available in the response.
func (e *EmbeddingClient) Embed(text string) (*openai.EmbeddingResponse, error) {
	return e.adapter.Embedding(&openai.EmbeddingRequest{
		Model: e.model,
		Input: text,
	})
}
