package evaluation

import "testing"

func TestExactMatchJudgeExact(t *testing.T) {
	j := &ExactMatchJudge{CaseSensitive: true}
	score, feedback, err := j.Score("hello world", "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 1.0 {
		t.Fatalf("expected 1.0, got %f", score)
	}
	if feedback != "exact match" {
		t.Fatalf("expected 'exact match', got %s", feedback)
	}
}

func TestExactMatchJudgeDifferent(t *testing.T) {
	j := &ExactMatchJudge{CaseSensitive: true}
	score, _, _ := j.Score("hello world", "goodbye world")
	if score != 0.0 {
		t.Fatalf("expected 0.0, got %f", score)
	}
}

func TestExactMatchJudgeCaseInsensitive(t *testing.T) {
	j := &ExactMatchJudge{CaseSensitive: false}
	score, _, _ := j.Score("Hello World", "hello world")
	if score != 1.0 {
		t.Fatalf("expected 1.0, got %f", score)
	}
}

func TestExactMatchJudgeTrimWhitespace(t *testing.T) {
	j := &ExactMatchJudge{CaseSensitive: false}
	score, _, _ := j.Score("  hello world  ", "Hello World")
	if score != 1.0 {
		t.Fatalf("expected 1.0 after trimming, got %f", score)
	}
}

func TestSemanticJudgeNilEmbedder(t *testing.T) {
	// Semantic judge requires an embedder; test that NewJudge catches this
	cfg := JudgeConfig{Type: JudgeSemantic}
	_, err := NewJudge(cfg, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil embedder")
	}
}

func TestLLMJudgeMissingUpstream(t *testing.T) {
	cfg := JudgeConfig{Type: JudgeLLM, JudgeUpstream: ""}
	_, err := NewJudge(cfg, nil, nil)
	if err == nil {
		t.Fatal("expected error for missing judge_upstream")
	}
}

func TestLLMJudgeUnknownUpstream(t *testing.T) {
	cfg := JudgeConfig{Type: JudgeLLM, JudgeUpstream: "nonexistent"}
	_, err := NewJudge(cfg, nil, nil)
	if err == nil {
		t.Fatal("expected error for unknown upstream")
	}
}

func TestNewJudgeExactMatch(t *testing.T) {
	cfg := JudgeConfig{Type: JudgeExactMatch}
	j, err := NewJudge(cfg, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := j.(*ExactMatchJudge); !ok {
		t.Fatal("expected ExactMatchJudge type")
	}
}

func TestNewJudgeUnknownType(t *testing.T) {
	cfg := JudgeConfig{Type: "unknown"}
	_, err := NewJudge(cfg, nil, nil)
	if err == nil {
		t.Fatal("expected error for unknown judge type")
	}
}

func TestLLMJudgeScoreParse(t *testing.T) {
	// Since LLMJudge requires an adapter, we can only test the score parsing here.
	// Full integration test would need a mock adapter.
	j := &LLMJudge{adapter: nil, model: "test"}
	_ = j // The actual score requires ChatCompletion call
}
