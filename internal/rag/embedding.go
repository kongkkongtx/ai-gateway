package rag

import (
	"github.com/kongkkongtx/ai-gateway/internal/provider/openai"
)

// EmbeddingAdapter wraps an OpenAI-compatible adapter to implement the Embedder interface.
type EmbeddingAdapter struct {
	adapter *openai.Adapter
	model   string
}

// NewEmbeddingAdapter creates a new EmbeddingAdapter.
func NewEmbeddingAdapter(adapter *openai.Adapter, model string) *EmbeddingAdapter {
	if model == "" {
		model = "text-embedding-3-small"
	}
	return &EmbeddingAdapter{adapter: adapter, model: model}
}

// Embed converts text to an embedding vector.
func (e *EmbeddingAdapter) Embed(text string) ([]float64, error) {
	resp, err := e.adapter.Embedding(&openai.EmbeddingRequest{
		Model: e.model,
		Input: text,
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 {
		return nil, nil
	}
	return resp.Data[0].Embedding, nil
}
