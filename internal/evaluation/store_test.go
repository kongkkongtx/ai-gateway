package evaluation

import "testing"

func TestCreateAndGetRun(t *testing.T) {
	s := NewStore()
	summary := s.CreateRun("run-1", "suite-1", 5, 2)
	if summary == nil {
		t.Fatal("expected non-nil summary")
	}
	if summary.RunID != "run-1" {
		t.Fatalf("expected run-1, got %s", summary.RunID)
	}
	if summary.Status != RunPending {
		t.Fatalf("expected pending, got %s", summary.Status)
	}
	if summary.TotalPrompts != 5 {
		t.Fatalf("expected 5 prompts, got %d", summary.TotalPrompts)
	}
	if summary.TotalModels != 2 {
		t.Fatalf("expected 2 models, got %d", summary.TotalModels)
	}

	got := s.GetRun("run-1")
	if got == nil {
		t.Fatal("expected to find run")
	}
	if got.RunID != "run-1" {
		t.Fatalf("expected run-1, got %s", got.RunID)
	}
}

func TestGetRunNotFound(t *testing.T) {
	s := NewStore()
	got := s.GetRun("nonexistent")
	if got != nil {
		t.Fatal("expected nil for nonexistent run")
	}
}

func TestUpdateStatus(t *testing.T) {
	s := NewStore()
	s.CreateRun("run-1", "suite-1", 1, 1)

	s.UpdateStatus("run-1", RunRunning)
	got := s.GetRun("run-1")
	if got.Status != RunRunning {
		t.Fatalf("expected running, got %s", got.Status)
	}

	s.UpdateStatus("run-1", RunCompleted)
	got = s.GetRun("run-1")
	if got.Status != RunCompleted {
		t.Fatalf("expected completed, got %s", got.Status)
	}
	if got.CompletedAt == nil {
		t.Fatal("expected completed_at to be set")
	}
}

func TestAddScoreAndAggregation(t *testing.T) {
	s := NewStore()
	s.CreateRun("run-1", "suite-1", 2, 2)

	s.AddScore("run-1", Score{
		PromptID:  "p1",
		ModelName: "model-a",
		Score:     0.8,
	})
	got := s.GetRun("run-1")
	if got.Progress != 1 {
		t.Fatalf("expected progress 1, got %d", got.Progress)
	}
	if got.AggregatedScores["model-a"] != 0.8 {
		t.Fatalf("expected model-a avg 0.8, got %f", got.AggregatedScores["model-a"])
	}

	s.AddScore("run-1", Score{
		PromptID:  "p2",
		ModelName: "model-a",
		Score:     0.6,
	})
	got = s.GetRun("run-1")
	if got.Progress != 2 {
		t.Fatalf("expected progress 2, got %d", got.Progress)
	}
	if got.AggregatedScores["model-a"] != 0.7 {
		t.Fatalf("expected model-a avg 0.7, got %f", got.AggregatedScores["model-a"])
	}

	// Add scores for another model
	s.AddScore("run-1", Score{
		PromptID:  "p1",
		ModelName: "model-b",
		Score:     0.9,
	})
	got = s.GetRun("run-1")
	if got.Progress != 3 {
		t.Fatalf("expected progress 3, got %d", got.Progress)
	}
	if got.AggregatedScores["model-b"] != 0.9 {
		t.Fatalf("expected model-b avg 0.9, got %f", got.AggregatedScores["model-b"])
	}
}

func TestAddScoreErrorExcluded(t *testing.T) {
	s := NewStore()
	s.CreateRun("run-1", "suite-1", 2, 1)

	// Error score should not contribute to aggregation
	s.AddScore("run-1", Score{
		PromptID:  "p1",
		ModelName: "model-a",
		Score:     0,
		Error:     "upstream error",
	})
	got := s.GetRun("run-1")
	if got.Progress != 1 {
		t.Fatalf("expected progress 1, got %d", got.Progress)
	}
	if _, ok := got.AggregatedScores["model-a"]; ok {
		t.Fatal("expected model-a to not have aggregated score due to all errors")
	}
}

func TestListRuns(t *testing.T) {
	s := NewStore()
	s.CreateRun("run-1", "suite-1", 1, 1)
	s.CreateRun("run-2", "suite-1", 1, 1)
	s.CreateRun("run-3", "suite-2", 1, 1)

	runs := s.ListRuns("suite-1", 0)
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs for suite-1, got %d", len(runs))
	}

	runs = s.ListRuns("suite-2", 0)
	if len(runs) != 1 {
		t.Fatalf("expected 1 run for suite-2, got %d", len(runs))
	}

	runs = s.ListRuns("suite-1", 1)
	if len(runs) != 1 {
		t.Fatalf("expected 1 run when limit=1, got %d", len(runs))
	}
}

func TestAddScoreNonExistentRun(t *testing.T) {
	s := NewStore()
	s.AddScore("nonexistent", Score{PromptID: "p1", ModelName: "m1", Score: 0.5})
	// Should not panic
}
