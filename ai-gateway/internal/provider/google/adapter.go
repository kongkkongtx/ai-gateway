// Package google implements the ProviderAdapter for Google's Gemini API.
// It translates between the internal OpenAI-compatible format and Google's
// Generative Language API format.
package google

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/yushi/ai-gateway/internal/provider/openai"
)

// Adapter proxies requests to Google's Gemini API, converting between
// OpenAI-compatible format and Gemini's API format.
// Uses the v1beta API endpoint for the latest features.
type Adapter struct {
	endpoint   string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewAdapter creates an Adapter for Google's Gemini API.
// The apiKey is the Google AI Studio API key.
func NewAdapter(endpoint, apiKey, model string, timeout time.Duration) *Adapter {
	if endpoint == "" {
		endpoint = "https://generativelanguage.googleapis.com"
	}
	return &Adapter{
		endpoint:   endpoint,
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// --- Gemini API types ---

type geminiContent struct {
	Role  string        `json:"role,omitempty"` // "user" or "model"
	Parts []geminiPart  `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text,omitempty"`
}

type geminiRequest struct {
	Contents         []geminiContent  `json:"contents"`
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
	GenerationConfig geminiGenConfig  `json:"generationConfig,omitempty"`
}

type geminiGenConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type geminiResponse struct {
	Candidates    []geminiCandidate `json:"candidates"`
	UsageMetadata *geminiUsage      `json:"usageMetadata"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"` // "STOP", "MAX_TOKENS", "SAFETY", "RECITATION", "OTHER"
}

type geminiUsage struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type geminiError struct {
	Error struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
		Status  string `json:"status"`
	} `json:"error"`
}

// ChatCompletion sends a non-streaming chat completion to Gemini and returns
// the result in OpenAI-compatible format.
func (a *Adapter) ChatCompletion(req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = a.model
	}

	geminiReq := a.toGeminiRequest(req)
	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	endpointURL := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s",
		a.endpoint, url.PathEscape(model), url.QueryEscape(a.apiKey))

	httpReq, err := http.NewRequest(http.MethodPost, endpointURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return nil, a.parseError(httpResp)
	}

	var geminiResp geminiResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&geminiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return a.toOpenAIResponse(&geminiResp, model), nil
}

// ChatCompletionStream sends a streaming request using Gemini's SSE endpoint.
func (a *Adapter) ChatCompletionStream(req *openai.ChatCompletionRequest) (*http.Response, error) {
	model := req.Model
	if model == "" {
		model = a.model
	}

	geminiReq := a.toGeminiRequest(req)
	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	endpointURL := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?key=%s&alt=sse",
		a.endpoint, url.PathEscape(model), url.QueryEscape(a.apiKey))

	httpReq, err := http.NewRequest(http.MethodPost, endpointURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

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

// Embedding sends a text embedding request to Gemini.
func (a *Adapter) Embedding(req *openai.EmbeddingRequest) (*openai.EmbeddingResponse, error) {
	model := req.Model
	if model == "" {
		model = a.model
	}

	// Extract text input
	var inputText string
	switch v := req.Input.(type) {
	case string:
		inputText = v
	default:
		inputBytes, err := json.Marshal(req.Input)
		if err != nil {
			return nil, fmt.Errorf("marshal input: %w", err)
		}
		inputText = string(inputBytes)
	}

	geminiEmbedReq := map[string]interface{}{
		"model": model,
		"content": geminiContent{
			Parts: []geminiPart{{Text: inputText}},
		},
	}

	body, err := json.Marshal(geminiEmbedReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	endpointURL := fmt.Sprintf("%s/v1beta/models/%s:embedContent?key=%s",
		a.endpoint, url.PathEscape(model), url.QueryEscape(a.apiKey))

	httpReq, err := http.NewRequest(http.MethodPost, endpointURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return nil, a.parseError(httpResp)
	}

	var geminiEmbedResp struct {
		Embedding struct {
			Values []float64 `json:"values"`
		} `json:"embedding"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&geminiEmbedResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &openai.EmbeddingResponse{
		Object: "list",
		Data: []openai.EmbeddingData{
			{
				Object:    "embedding",
				Index:     0,
				Embedding: geminiEmbedResp.Embedding.Values,
			},
		},
		Model: model,
		Usage: &openai.Usage{
			PromptTokens:     0,
			CompletionTokens: 0,
			TotalTokens:      0,
		},
	}, nil
}

// --- Conversion helpers ---

func (a *Adapter) toGeminiRequest(req *openai.ChatCompletionRequest) *geminiRequest {
	geminiReq := &geminiRequest{
		GenerationConfig: geminiGenConfig{
			Temperature: req.Temperature,
			TopP:        req.TopP,
		},
	}

	if req.MaxTokens != nil {
		geminiReq.GenerationConfig.MaxOutputTokens = req.MaxTokens
	}
	if len(req.Stop) > 0 {
		geminiReq.GenerationConfig.StopSequences = req.Stop
	}

	// Split system message from conversation messages
	var systemContent string
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			if systemContent != "" {
				systemContent += "\n" + msg.Content
			} else {
				systemContent = msg.Content
			}
			continue
		}

		// Map OpenAI roles to Gemini roles
		role := msg.Role
		if role == "assistant" || role == "model" {
			role = "model"
		} else if role == "user" || role == "system" {
			role = "user"
		}

		geminiReq.Contents = append(geminiReq.Contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: msg.Content}},
		})
	}

	if systemContent != "" {
		geminiReq.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: systemContent}},
		}
	}

	return geminiReq
}

func (a *Adapter) toOpenAIResponse(geminiResp *geminiResponse, model string) *openai.ChatCompletionResponse {
	resp := &openai.ChatCompletionResponse{
		ID:      fmt.Sprintf("gemini-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
	}

	if len(geminiResp.Candidates) > 0 {
		candidate := geminiResp.Candidates[0]
		resp.Choices = []openai.Choice{
			{
				Index: 0,
				Message: openai.Message{
					Role:    "assistant",
					Content: extractText(candidate.Content.Parts),
				},
				FinishReason: mapFinishReason(candidate.FinishReason),
			},
		}
	}

	if geminiResp.UsageMetadata != nil {
		resp.Usage = &openai.Usage{
			PromptTokens:     geminiResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      geminiResp.UsageMetadata.TotalTokenCount,
		}
	}

	return resp
}

func extractText(parts []geminiPart) string {
	var text string
	for _, p := range parts {
		text += p.Text
	}
	return text
}

func mapFinishReason(geminiReason string) string {
	switch geminiReason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	case "RECITATION":
		return "content_filter"
	default:
		return geminiReason
	}
}

func (a *Adapter) parseError(resp *http.Response) error {
	var errResp geminiError
	if decodeErr := json.NewDecoder(resp.Body).Decode(&errResp); decodeErr == nil && errResp.Error.Message != "" {
		return fmt.Errorf("gemini error (HTTP %d): %s [status=%s]", resp.StatusCode, errResp.Error.Message, errResp.Error.Status)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("gemini HTTP %d: %s", resp.StatusCode, string(bodyBytes))
}