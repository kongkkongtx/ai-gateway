// Package anthropic implements the ProviderAdapter for Anthropic's Claude API.
// It translates between the internal OpenAI-compatible format and Anthropic's
// Messages API format.
package anthropic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kongkkongtx/ai-gateway/internal/provider/openai"
)

// Adapter proxies requests to Anthropic's Claude API, converting between
// OpenAI-compatible format and Anthropic's Messages API format.
type Adapter struct {
	endpoint   string
	apiKey     string
	model      string
	apiVersion string
	httpClient *http.Client
}

// NewAdapter creates an Adapter for Anthropic's Claude API.
// The apiKey is the Anthropic API key (x-api-key header).
func NewAdapter(endpoint, apiKey, model string, timeout time.Duration) *Adapter {
	if endpoint == "" {
		endpoint = "https://api.anthropic.com"
	}
	return &Adapter{
		endpoint:   endpoint,
		apiKey:     apiKey,
		model:      model,
		apiVersion: "2023-06-01",
		httpClient: &http.Client{Timeout: timeout},
	}
}

// --- Anthropic Messages API types ---

type anthropicMessage struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content any    `json:"content"` // string or []contentBlock
}

type contentBlock struct {
	Type string `json:"type"` // "text" or "tool_use"
	Text string `json:"text,omitempty"`
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
	Temperature *float64           `json:"temperature,omitempty"`
	TopP        *float64           `json:"top_p,omitempty"`
	Stop        []string           `json:"stop_sequences,omitempty"`
}

type anthropicResponse struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Role       string            `json:"role"`
	Content    []contentBlock    `json:"content"`
	Model      string            `json:"model"`
	StopReason string            `json:"stop_reason"`
	Usage      anthropicUsage    `json:"usage"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// ChatCompletion sends a non-streaming chat completion to Anthropic and returns
// the result in OpenAI-compatible format.
func (a *Adapter) ChatCompletion(req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	anthropicReq, err := a.toAnthropicRequest(req)
	if err != nil {
		return nil, fmt.Errorf("convert to anthropic format: %w", err)
	}

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, a.endpoint+"/v1/messages", bytes.NewReader(body))
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
		return nil, a.parseError(httpResp)
	}

	var anthropicResp anthropicResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&anthropicResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return a.toOpenAIResponse(&anthropicResp, req), nil
}

// ChatCompletionStream sends a streaming request. Returns the raw HTTP response
// with SSE stream from Anthropic. Note: Anthropic's SSE format differs from OpenAI's,
// so the raw stream is passed through. Clients that expect OpenAI SSE format may
// need client-side handling.
func (a *Adapter) ChatCompletionStream(req *openai.ChatCompletionRequest) (*http.Response, error) {
	anthropicReq, err := a.toAnthropicRequest(req)
	if err != nil {
		return nil, fmt.Errorf("convert to anthropic format: %w", err)
	}
	anthropicReq.Stream = true

	// Set max_tokens to a default if not specified (Anthropic requires it)
	if anthropicReq.MaxTokens <= 0 {
		anthropicReq.MaxTokens = 4096
	}

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, a.endpoint+"/v1/messages", bytes.NewReader(body))
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
		return nil, a.parseError(httpResp)
	}

	return httpResp, nil
}

// Embedding is not supported by Anthropic's Claude API.
func (a *Adapter) Embedding(req *openai.EmbeddingRequest) (*openai.EmbeddingResponse, error) {
	return nil, fmt.Errorf("embeddings not supported by Anthropic Claude API")
}

// --- Conversion helpers ---

func (a *Adapter) toAnthropicRequest(req *openai.ChatCompletionRequest) (*anthropicRequest, error) {
	model := req.Model
	if model == "" {
		model = a.model
	}

	anthropicReq := &anthropicRequest{
		Model:       model,
		MaxTokens:   4096,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stop:        req.Stop,
	}

	if req.MaxTokens != nil {
		anthropicReq.MaxTokens = *req.MaxTokens
	}

	// Separate system message from other messages
	var systemPrompt string
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			if systemPrompt != "" {
				systemPrompt += "\n" + openai.ExtractText(msg.Content)
			} else {
				systemPrompt = openai.ExtractText(msg.Content)
			}
			continue
		}
		// Map OpenAI roles to Anthropic roles
		role := msg.Role
		if role == "assistant" {
			role = "assistant"
		}
		anthropicReq.Messages = append(anthropicReq.Messages, anthropicMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	if systemPrompt != "" {
		anthropicReq.System = systemPrompt
	}

	return anthropicReq, nil
}

func (a *Adapter) toOpenAIResponse(anthropicResp *anthropicResponse, req *openai.ChatCompletionRequest) *openai.ChatCompletionResponse {
	// Extract text from content blocks
	var contentText string
	for _, block := range anthropicResp.Content {
		if block.Type == "text" {
			contentText = block.Text
			break
		}
	}

	// Map Anthropic stop_reason to OpenAI finish_reason
	finishReason := mapStopReason(anthropicResp.StopReason)

	// Map usage
	var usage *openai.Usage
	if anthropicResp.Usage.InputTokens > 0 || anthropicResp.Usage.OutputTokens > 0 {
		usage = &openai.Usage{
			PromptTokens:     anthropicResp.Usage.InputTokens,
			CompletionTokens: anthropicResp.Usage.OutputTokens,
			TotalTokens:      anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
		}
	}

	return &openai.ChatCompletionResponse{
		ID:      anthropicResp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   anthropicResp.Model,
		Choices: []openai.Choice{
			{
				Index: 0,
				Message: openai.Message{
					Role:    "assistant",
					Content: contentText,
				},
				FinishReason: finishReason,
			},
		},
		Usage: usage,
	}
}

func mapStopReason(anthropicReason string) string {
	switch anthropicReason {
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		return anthropicReason
	}
}

// parseError reads an Anthropic error response and returns a descriptive error.
func (a *Adapter) parseError(resp *http.Response) error {
	var errResp anthropicError
	if decodeErr := json.NewDecoder(resp.Body).Decode(&errResp); decodeErr == nil && errResp.Error.Message != "" {
		return fmt.Errorf("anthropic error (HTTP %d): %s [type=%s]", resp.StatusCode, errResp.Error.Message, errResp.Error.Type)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("anthropic HTTP %d: %s", resp.StatusCode, string(bodyBytes))
}

func (a *Adapter) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", a.apiVersion)
}