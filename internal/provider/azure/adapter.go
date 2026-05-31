// Package azure implements the ProviderAdapter for Azure OpenAI Service.
// Azure OpenAI uses the same API schema as OpenAI but with a different
// authentication mechanism (api-key header) and URL format that includes
// the deployment name and API version.
package azure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/yushi/ai-gateway/internal/provider/openai"
)

// Adapter proxies requests to Azure OpenAI Service.
// The endpoint format is:
//
//	https://{resource}.openai.azure.com/openai/deployments/{deployment}/{path}?api-version={apiVersion}
type Adapter struct {
	endpoint   string // Base URL like "https://my-resource.openai.azure.com"
	apiKey     string
	deployment string // Azure deployment name (maps to model)
	apiVersion string // e.g. "2024-02-15-preview"
	httpClient *http.Client
}

// NewAdapter creates an Adapter for Azure OpenAI Service.
// The apiKey is the Azure OpenAI API key (api-key header, not Bearer).
// The model parameter is used as the deployment name.
func NewAdapter(endpoint, apiKey, deployment string, timeout time.Duration, apiVersion string) *Adapter {
	if endpoint == "" {
		endpoint = "https://api.openai.azure.com"
	}
	if apiVersion == "" {
		apiVersion = "2024-02-15-preview"
	}
	return &Adapter{
		endpoint:   endpoint,
		apiKey:     apiKey,
		deployment: deployment,
		apiVersion: apiVersion,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// chatURL builds the Azure OpenAI chat completions URL.
func (a *Adapter) chatURL() string {
	return fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		a.endpoint, a.deployment, a.apiVersion)
}

// embeddingURL builds the Azure OpenAI embeddings URL.
func (a *Adapter) embeddingURL() string {
	return fmt.Sprintf("%s/openai/deployments/%s/embeddings?api-version=%s",
		a.endpoint, a.deployment, a.apiVersion)
}

// ChatCompletion sends a non-streaming chat completion to Azure OpenAI.
func (a *Adapter) ChatCompletion(req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	// Apply default deployment if request doesn't specify model
	if a.deployment != "" && req.Model == "" {
		req.Model = a.deployment
	}
	// Use the deployment name as the model in the request body (Azure ignores it)
	if req.Model == "" {
		req.Model = a.deployment
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, a.chatURL(), bytes.NewReader(body))
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

	var result openai.ChatCompletionResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// ChatCompletionStream sends a streaming request to Azure OpenAI.
func (a *Adapter) ChatCompletionStream(req *openai.ChatCompletionRequest) (*http.Response, error) {
	if a.deployment != "" && req.Model == "" {
		req.Model = a.deployment
	}
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, a.chatURL(), bytes.NewReader(body))
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
		return nil, fmt.Errorf("azure error HTTP %d: %s", httpResp.StatusCode, string(bodyBytes))
	}

	return httpResp, nil
}

// Embedding sends a text embedding request to Azure OpenAI.
func (a *Adapter) Embedding(req *openai.EmbeddingRequest) (*openai.EmbeddingResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, a.embeddingURL(), bytes.NewReader(body))
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

	var result openai.EmbeddingResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

func (a *Adapter) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", a.apiKey)
}

func (a *Adapter) parseError(resp *http.Response) error {
	var errResp struct {
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&errResp); decodeErr == nil && errResp.Error.Message != "" {
		return fmt.Errorf("azure error (HTTP %d): %s [code=%s]", resp.StatusCode, errResp.Error.Message, errResp.Error.Code)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("azure HTTP %d: %s", resp.StatusCode, string(bodyBytes))
}