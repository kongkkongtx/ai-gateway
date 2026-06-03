// Package provider defines the ProviderAdapter interface that all AI provider
// adapters must implement. The internal unified format is OpenAI-compatible,
// so all adapters translate between their native API and this format.
package provider

import (
	"net/http"

	"github.com/kongkkongtx/ai-gateway/internal/provider/openai"
)

// ProviderAdapter is the interface that all provider adapters must implement.
// All methods accept and return OpenAI-compatible types as the internal unified format.
// Each adapter is responsible for translating between the OpenAI format and the
// provider's native API format.
type ProviderAdapter interface {
	// ChatCompletion sends a non-streaming chat completion request.
	ChatCompletion(req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error)

	// ChatCompletionStream sends a streaming request and returns the raw HTTP response.
	// The caller must read the SSE stream from resp.Body and close it when done.
	ChatCompletionStream(req *openai.ChatCompletionRequest) (*http.Response, error)

	// Embedding sends a text embedding request and returns the vector result.
	Embedding(req *openai.EmbeddingRequest) (*openai.EmbeddingResponse, error)
}