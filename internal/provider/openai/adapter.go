// Package openai implements the OpenAI-compatible provider adapter.
// Since DeepSeek and many other providers offer OpenAI-compatible APIs,
// this adapter works with them by simply changing the endpoint URL.
package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Adapter proxies chat completion and embedding requests to an OpenAI-compatible API endpoint.
// It handles both streaming (SSE) and non-streaming responses.
type Adapter struct {
	endpoint   string       // Base URL of the upstream API (e.g. "https://api.openai.com")
	apiToken   string       // Bearer token for authorization
	model      string       // Default model to use if request doesn't specify one
	httpClient *http.Client
}

// NewAdapter creates an Adapter targeting the given endpoint with the provided auth token.
func NewAdapter(endpoint, apiToken, model string, timeout time.Duration) *Adapter {
	if endpoint == "" {
		endpoint = "https://api.openai.com"
	}
	return &Adapter{
		endpoint: endpoint,
		apiToken: apiToken,
		model:    model,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// ChatCompletionRequest mirrors the OpenAI chat completion request schema.
// Only commonly used fields are included; additional fields are ignored during marshaling.
type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature *float64  `json:"temperature,omitempty"`
	TopP        *float64  `json:"top_p,omitempty"`
	N           *int      `json:"n,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
	PresencePenalty  *float64 `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`
	User        string    `json:"user,omitempty"`
}

// ContentPart represents an element of a multimodal content array.
// When a message contains images or audio, Content is an array of ContentPart
// instead of a plain string.
type ContentPart struct {
	Type       string      `json:"type"`                 // "text" | "image_url" | "input_audio"
	Text       string      `json:"text,omitempty"`
	ImageURL   *ImageURL   `json:"image_url,omitempty"`
	InputAudio *InputAudio `json:"input_audio,omitempty"`
}

// ImageURL represents an image in a multimodal message.
type ImageURL struct {
	URL    string `json:"url"`              // HTTP URL or data:image/...;base64,...
	Detail string `json:"detail,omitempty"` // "auto" | "low" | "high"
}

// InputAudio represents audio input in a multimodal message.
type InputAudio struct {
	Data   string `json:"data"`   // base64 encoded audio
	Format string `json:"format"` // "wav" | "mp3" | "flac" | "opus" | "pcm16"
}

// Message represents a single turn in a chat conversation.
// Content can be a plain string (for text-only messages) or a []ContentPart
// (for multimodal messages containing images or audio).
type Message struct {
	Role    string `json:"role"`    // "system", "user", or "assistant"
	Content any    `json:"content"` // string or []ContentPart
}

// ExtractText extracts the first text content from a Message's Content field.
// It handles both plain string and content parts array formats.
func ExtractText(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []ContentPart:
		for _, p := range v {
			if p.Type == "text" && p.Text != "" {
				return p.Text
			}
		}
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				if t, ok2 := m["type"].(string); ok2 && t == "text" {
					if text, ok3 := m["text"].(string); ok3 {
						return text
					}
				}
			}
		}
	}
	return ""
}

// ChatCompletionResponse mirrors the OpenAI chat completion response schema.
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

// Choice represents one of the returned completion candidates.
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"` // "stop", "length", or "content_filter"
}

// Usage tracks token consumption for billing and monitoring.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// EmbeddingRequest mirrors the OpenAI embedding request schema.
type EmbeddingRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"` // Accepts string or []string
	User  string `json:"user,omitempty"`
}

// EmbeddingResponse mirrors the OpenAI embedding response schema.
type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  *Usage          `json:"usage,omitempty"`
}

// EmbeddingData contains a single embedding vector and its index.
type EmbeddingData struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

// ErrorResponse represents a structured error returned by the upstream API.
type ErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code,omitempty"`
	} `json:"error"`
}

// ChatCompletion sends a non-streaming chat completion request.
// Returns the parsed response or an error that includes the upstream's error details.
func (a *Adapter) ChatCompletion(req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	// Apply default model if the request didn't specify one
	if a.model != "" && req.Model == "" {
		req.Model = a.model
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequest(http.MethodPost, a.endpoint+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	a.setHeaders(httpReq)
	httpResp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if decodeErr := json.NewDecoder(httpResp.Body).Decode(&errResp); decodeErr == nil {
			return nil, fmt.Errorf("upstream error: %s (type=%s, code=%s)", errResp.Error.Message, errResp.Error.Type, errResp.Error.Code)
		}
		return nil, fmt.Errorf("upstream HTTP %d", httpResp.StatusCode)
	}
	var result ChatCompletionResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// ChatCompletionStream sends a streaming request and returns the raw HTTP response.
// The caller must read the SSE stream from resp.Body and close it when done.
func (a *Adapter) ChatCompletionStream(req *ChatCompletionRequest) (*http.Response, error) {
	if a.model != "" && req.Model == "" {
		req.Model = a.model
	}
	req.Stream = true // Force streaming on the upstream request
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequest(http.MethodPost, a.endpoint+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	a.setHeaders(httpReq)
	httpResp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		defer httpResp.Body.Close()
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("upstream error HTTP %d: %s", httpResp.StatusCode, string(bodyBytes))
	}
	return httpResp, nil
}

// Embedding sends a text embedding request and returns the vector result.
func (a *Adapter) Embedding(req *EmbeddingRequest) (*EmbeddingResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequest(http.MethodPost, a.endpoint+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	a.setHeaders(httpReq)
	httpResp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if decodeErr := json.NewDecoder(httpResp.Body).Decode(&errResp); decodeErr == nil {
			return nil, fmt.Errorf("upstream error: %s", errResp.Error.Message)
		}
		return nil, fmt.Errorf("upstream HTTP %d", httpResp.StatusCode)
	}
	var result EmbeddingResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// setHeaders applies the standard authorization and content-type headers.
func (a *Adapter) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiToken)
}