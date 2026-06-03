package evaluation

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/kongkkongtx/ai-gateway/internal/provider"
	"github.com/kongkkongtx/ai-gateway/internal/provider/openai"
	"github.com/kongkkongtx/ai-gateway/internal/semantic"
	"github.com/kongkkongtx/ai-gateway/internal/webhook"
)

// Runner manages asynchronous evaluation runs.
type Runner struct {
	mu            sync.Mutex
	cfg           Config
	store         *Store
	adapters      map[string]provider.ProviderAdapter
	embedder      *semantic.EmbeddingClient
	runs          map[string]context.CancelFunc // runID -> cancel function
	logger        *slog.Logger
	webhookNotifier *webhook.Notifier
}

// NewRunner creates an evaluation runner.
// embedder may be nil (required only for semantic similarity judge).
// webhookNotifier may be nil.
func NewRunner(cfg Config, adapters map[string]provider.ProviderAdapter, embedder *semantic.EmbeddingClient, logger *slog.Logger, webhookNotifier *webhook.Notifier) *Runner {
	store := NewStore()
	return &Runner{
		cfg:             cfg,
		store:           store,
		adapters:        adapters,
		embedder:        embedder,
		runs:            make(map[string]context.CancelFunc),
		logger:          logger,
		webhookNotifier: webhookNotifier,
	}
}

// Store returns the runner's store for querying results.
func (r *Runner) Store() *Store {
	return r.store
}

// Execute runs an evaluation suite asynchronously.
// Returns the run ID immediately.
func (r *Runner) Execute(suite EvalSuite) (string, error) {
	if len(suite.TestPrompts) == 0 {
		return "", fmt.Errorf("evaluation suite %q has no test prompts", suite.ID)
	}
	if len(suite.TargetModels) == 0 {
		return "", fmt.Errorf("evaluation suite %q has no target models", suite.ID)
	}

	// Validate judge configuration
	judge, err := NewJudge(suite.Judge, r.adapters, r.embedder)
	if err != nil {
		return "", fmt.Errorf("invalid judge config: %w", err)
	}

	runID := fmt.Sprintf("run-%d", time.Now().UnixNano())
	totalPrompts := len(suite.TestPrompts)
	totalModels := len(suite.TargetModels)

	r.store.CreateRun(runID, suite.ID, totalPrompts, totalModels)

	ctx, cancel := context.WithCancel(context.Background())

	r.mu.Lock()
	r.runs[runID] = cancel
	r.mu.Unlock()

	go r.execute(ctx, runID, suite, judge)

	r.logger.Info("evaluation run started",
		"run_id", runID,
		"suite", suite.Name,
		"prompts", totalPrompts,
		"models", totalModels,
	)
	return runID, nil
}

// Cancel signals a running evaluation to stop.
func (r *Runner) Cancel(runID string) bool {
	r.mu.Lock()
	cancel, ok := r.runs[runID]
	r.mu.Unlock()
	if !ok {
		return false
	}
	cancel()
	r.store.UpdateStatus(runID, RunCancelled)
	r.logger.Info("evaluation run cancelled", "run_id", runID)
	return true
}

func (r *Runner) execute(ctx context.Context, runID string, suite EvalSuite, judge Judge) {
	r.store.UpdateStatus(runID, RunRunning)

	for _, model := range suite.TargetModels {
		adapter, ok := r.adapters[model.Upstream]
		if !ok {
			r.logger.Error("evaluation target upstream not found",
				"run_id", runID, "model", model.Name, "upstream", model.Upstream)
			r.recordError(runID, model.Name, fmt.Sprintf("upstream %q not found", model.Upstream), "")
			continue
		}

		for _, prompt := range suite.TestPrompts {
			select {
			case <-ctx.Done():
				r.logger.Info("evaluation run cancelled mid-execution", "run_id", runID)
				return
			default:
			}

			score := r.evaluateSingle(ctx, runID, adapter, model, prompt, judge)
			r.store.AddScore(runID, score)
		}
	}

	summary := r.store.GetRun(runID)
	if summary == nil {
		return
	}

	// Determine final status
	allErrors := true
	for _, s := range summary.Scores {
		if s.Error == "" {
			allErrors = false
			break
		}
	}

	if allErrors && summary.Progress > 0 {
		r.store.UpdateStatus(runID, RunFailed)
		r.fireWebhook(webhook.EventEvalRunFailed, runID, suite.Name)
	} else {
		r.store.UpdateStatus(runID, RunCompleted)
		r.fireWebhook(webhook.EventEvalRunCompleted, runID, suite.Name)
	}

	r.mu.Lock()
	delete(r.runs, runID)
	r.mu.Unlock()

	r.logger.Info("evaluation run completed",
		"run_id", runID,
		"suite", suite.Name,
		"progress", r.store.GetRun(runID).Progress,
		"total", r.store.GetRun(runID).TotalPrompts*r.store.GetRun(runID).TotalModels,
	)
}

func (r *Runner) evaluateSingle(ctx context.Context, runID string, adapter provider.ProviderAdapter, model TargetModel, prompt TestPrompt, judge Judge) Score {
	start := time.Now()

	req := &openai.ChatCompletionRequest{
		Model: model.Model,
		Messages: []openai.Message{
			{Role: "user", Content: prompt.Prompt},
		},
	}

	resp, err := adapter.ChatCompletion(req)
	latencyMs := float64(time.Since(start)) / float64(time.Millisecond)

	if err != nil {
		return Score{
			PromptID: prompt.ID,
			ModelName: model.Name,
			Score:    0,
			LatencyMs: latencyMs,
			Error:    err.Error(),
		}
	}

	if len(resp.Choices) == 0 {
		return Score{
			PromptID: prompt.ID,
			ModelName: model.Name,
			Score:    0,
			LatencyMs: latencyMs,
			Error:    "no choices in response",
		}
	}

	responseText := openai.ExtractText(resp.Choices[0].Message.Content)

	var score float64
	var feedback string
	if prompt.ReferenceAnswer != "" {
		score, feedback, err = judge.Score(responseText, prompt.ReferenceAnswer)
		if err != nil {
			r.logger.Warn("judge scoring failed",
				"run_id", runID,
				"prompt", prompt.ID,
				"model", model.Name,
				"error", err,
			)
		}
	}

	promptTokens := 0
	completionTokens := 0
	if resp.Usage != nil {
		promptTokens = resp.Usage.PromptTokens
		completionTokens = resp.Usage.CompletionTokens
	}

	return Score{
		PromptID:         prompt.ID,
		ModelName:        model.Name,
		Score:            score,
		JudgeFeedback:    feedback,
		ResponseText:     responseText,
		LatencyMs:        latencyMs,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
	}
}

func (r *Runner) recordError(runID, modelName, errMsg, promptID string) {
	r.store.AddScore(runID, Score{
		PromptID:  promptID,
		ModelName: modelName,
		Score:     0,
		Error:     errMsg,
	})
}

func (r *Runner) fireWebhook(eventType webhook.EventType, runID, suiteName string) {
	if r.webhookNotifier == nil {
		return
	}
	r.webhookNotifier.Send(webhook.Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Severity:  "info",
		Title:     fmt.Sprintf("Evaluation %s: %s", eventType, suiteName),
		Message:   fmt.Sprintf("Evaluation run %s for suite %s", runID, suiteName),
		Details: map[string]interface{}{
			"run_id":  runID,
			"suite":   suiteName,
			"status":  eventType,
		},
	})
}
