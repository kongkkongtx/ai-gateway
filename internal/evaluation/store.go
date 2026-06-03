package evaluation

import (
	"sync"
	"time"
)

// RunStatus represents the current state of an evaluation run.
type RunStatus string

const (
	RunPending   RunStatus = "pending"
	RunRunning   RunStatus = "running"
	RunCompleted RunStatus = "completed"
	RunFailed    RunStatus = "failed"
	RunCancelled RunStatus = "cancelled"
)

// Score holds a single evaluation score for one prompt-model pair.
type Score struct {
	PromptID         string  `json:"prompt_id"`
	ModelName        string  `json:"model_name"`
	Score            float64 `json:"score"`
	JudgeFeedback    string  `json:"judge_feedback,omitempty"`
	ResponseText     string  `json:"response_text,omitempty"`
	LatencyMs        float64 `json:"latency_ms"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	Error            string  `json:"error,omitempty"`
}

// RunSummary holds the status and aggregated results of an evaluation run.
type RunSummary struct {
	RunID            string             `json:"run_id"`
	SuiteID          string             `json:"suite_id"`
	Status           RunStatus          `json:"status"`
	StartedAt        time.Time          `json:"started_at"`
	CompletedAt      *time.Time         `json:"completed_at,omitempty"`
	Progress         int                `json:"progress"`
	TotalPrompts     int                `json:"total_prompts"`
	TotalModels      int                `json:"total_models"`
	AggregatedScores map[string]float64 `json:"aggregated_scores"` // model_name -> avg score
	Scores           []Score            `json:"scores,omitempty"`
}

// Store provides thread-safe in-memory storage for evaluation runs.
type Store struct {
	mu   sync.RWMutex
	runs map[string]*RunSummary
}

// NewStore creates a new evaluation run store.
func NewStore() *Store {
	return &Store{
		runs: make(map[string]*RunSummary),
	}
}

// CreateRun creates a new run summary and stores it.
func (s *Store) CreateRun(runID, suiteID string, totalPrompts, totalModels int) *RunSummary {
	summary := &RunSummary{
		RunID:            runID,
		SuiteID:          suiteID,
		Status:           RunPending,
		StartedAt:        time.Now(),
		TotalPrompts:     totalPrompts,
		TotalModels:      totalModels,
		AggregatedScores: make(map[string]float64),
		Scores:           make([]Score, 0),
	}
	s.mu.Lock()
	s.runs[runID] = summary
	s.mu.Unlock()
	return summary
}

// GetRun retrieves a run summary by run ID.
func (s *Store) GetRun(runID string) *RunSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	summary, ok := s.runs[runID]
	if !ok {
		return nil
	}
	return summary
}

// ListRuns returns all runs for a given suite, newest first.
func (s *Store) ListRuns(suiteID string, limit int) []RunSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []RunSummary
	for _, r := range s.runs {
		if r.SuiteID == suiteID {
			result = append(result, *r)
		}
	}

	// Sort by start time descending
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].StartedAt.After(result[i].StartedAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

// UpdateStatus changes the status of a run.
func (s *Store) UpdateStatus(runID string, status RunStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.runs[runID]; ok {
		r.Status = status
		if status == RunCompleted || status == RunFailed || status == RunCancelled {
			now := time.Now()
			r.CompletedAt = &now
		}
	}
}

// AddScore adds a score to a run and recalculates aggregated scores.
func (s *Store) AddScore(runID string, score Score) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[runID]
	if !ok {
		return
	}
	r.Scores = append(r.Scores, score)
	r.Progress++

	// Recalculate aggregated scores per model
	modelScores := make(map[string][]float64)
	for _, sc := range r.Scores {
		if sc.Error == "" {
			modelScores[sc.ModelName] = append(modelScores[sc.ModelName], sc.Score)
		}
	}
	for model, scores := range modelScores {
		sum := 0.0
		for _, s := range scores {
			sum += s
		}
		r.AggregatedScores[model] = sum / float64(len(scores))
	}
}
