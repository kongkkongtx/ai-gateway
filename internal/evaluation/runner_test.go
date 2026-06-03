package evaluation

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kongkkongtx/ai-gateway/internal/provider"
	"github.com/kongkkongtx/ai-gateway/internal/provider/openai"
)

// mockAdapter is a simple mock that returns a fixed response.
type mockAdapter struct {
	response *openai.ChatCompletionResponse
}

func (m *mockAdapter) ChatCompletion(req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	return m.response, nil
}

func (m *mockAdapter) ChatCompletionStream(req *openai.ChatCompletionRequest) (*http.Response, error) {
	return nil, nil
}

func (m *mockAdapter) Embedding(req *openai.EmbeddingRequest) (*openai.EmbeddingResponse, error) {
	return nil, nil
}

func TestExecuteEmptySuite(t *testing.T) {
	logger := slog.Default()
	adapters := make(map[string]provider.ProviderAdapter)
	runner := NewRunner(Config{}, adapters, nil, logger, nil)

	suite := EvalSuite{ID: "suite-1", Name: "Empty"}
	_, err := runner.Execute(suite)
	if err == nil {
		t.Fatal("expected error for empty suite")
	}
}

func TestExecuteNoModels(t *testing.T) {
	logger := slog.Default()
	adapters := make(map[string]provider.ProviderAdapter)
	runner := NewRunner(Config{}, adapters, nil, logger, nil)

	suite := EvalSuite{
		ID: "suite-1", Name: "No Models",
		TestPrompts: []TestPrompt{{ID: "p1", Prompt: "hello"}},
	}
	_, err := runner.Execute(suite)
	if err == nil {
		t.Fatal("expected error for no target models")
	}
}

func TestExecuteAndGetResults(t *testing.T) {
	// Setup mock upstream server
	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"test","object":"chat.completion","created":1234567890,"model":"test","choices":[{"index":0,"message":{"role":"assistant","content":"hello world"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`))
	}))
	defer mockSrv.Close()

	adapters := map[string]provider.ProviderAdapter{
		"mock-up": openai.NewAdapter(mockSrv.URL, "sk-test", "test-model", 0),
	}
	runner := NewRunner(Config{}, adapters, nil, slog.Default(), nil)

	suite := EvalSuite{
		ID:   "suite-1",
		Name: "Test Suite",
		TestPrompts: []TestPrompt{
			{ID: "p1", Prompt: "say hello", ReferenceAnswer: "hello world"},
		},
		TargetModels: []TargetModel{
			{Name: "MockModel", Upstream: "mock-up", Model: "test-model"},
		},
		Judge: JudgeConfig{
			Type: JudgeExactMatch,
		},
	}

	runID, err := runner.Execute(suite)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Wait for async run to complete
	summary := runner.Store().GetRun(runID)
	if summary == nil {
		t.Fatal("expected run summary")
	}

	// Poll for completion
	for i := 0; i < 50; i++ {
		summary = runner.Store().GetRun(runID)
		if summary.Status == RunCompleted || summary.Status == RunFailed {
			break
		}
	}
	if summary.Status != RunCompleted {
		t.Fatalf("expected completed, got %s", summary.Status)
	}
	if summary.Progress != 1 {
		t.Fatalf("expected progress 1, got %d", summary.Progress)
	}
	if len(summary.Scores) != 1 {
		t.Fatalf("expected 1 score, got %d", len(summary.Scores))
	}
	if summary.Scores[0].Score != 1.0 {
		t.Fatalf("expected score 1.0 for exact match, got %f", summary.Scores[0].Score)
	}
	if summary.Scores[0].ModelName != "MockModel" {
		t.Fatalf("expected MockModel, got %s", summary.Scores[0].ModelName)
	}
	if summary.AggregatedScores["MockModel"] != 1.0 {
		t.Fatalf("expected MockModel aggregated score 1.0, got %f", summary.AggregatedScores["MockModel"])
	}
}

func TestExecuteCancel(t *testing.T) {
	// Create a slow mock server
	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"test","object":"chat.completion","created":1234567890,"model":"test","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer mockSrv.Close()

	adapters := map[string]provider.ProviderAdapter{
		"mock-up": openai.NewAdapter(mockSrv.URL, "sk-test", "test-model", 0),
	}
	runner := NewRunner(Config{}, adapters, nil, slog.Default(), nil)

	suite := EvalSuite{
		ID:   "suite-cancel",
		Name: "Cancel Test",
		TestPrompts: []TestPrompt{
			{ID: "p1", Prompt: "test1"},
			{ID: "p2", Prompt: "test2"},
			{ID: "p3", Prompt: "test3"},
		},
		TargetModels: []TargetModel{
			{Name: "Model", Upstream: "mock-up", Model: "test"},
		},
		Judge: JudgeConfig{Type: JudgeExactMatch},
	}

	runID, err := runner.Execute(suite)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Cancel immediately
	ok := runner.Cancel(runID)
	if !ok {
		t.Fatal("Cancel returned false")
	}

	summary := runner.Store().GetRun(runID)
	if summary.Status != RunCancelled {
		t.Fatalf("expected cancelled, got %s", summary.Status)
	}
}

func TestCancelNonExistent(t *testing.T) {
	runner := NewRunner(Config{}, nil, nil, slog.Default(), nil)
	ok := runner.Cancel("nonexistent")
	if ok {
		t.Fatal("expected false for non-existent run")
	}
}
