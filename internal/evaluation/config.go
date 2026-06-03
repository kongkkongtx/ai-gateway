// Package evaluation implements model quality evaluation framework.
// It allows defining test prompt suites, running them against multiple
// model targets, and scoring responses using various judge strategies
// (exact match, semantic similarity, LLM-as-judge).
package evaluation

import "time"

// Config defines the top-level evaluation configuration.
type Config struct {
	Enabled bool        `yaml:"enabled" json:"enabled"`
	Suites  []EvalSuite `yaml:"suites" json:"suites"`
}

// EvalSuite defines a single evaluation test suite.
type EvalSuite struct {
	ID              string       `yaml:"id" json:"id"`
	Name            string       `yaml:"name" json:"name"`
	Description     string       `yaml:"description,omitempty" json:"description,omitempty"`
	TestPrompts     []TestPrompt `yaml:"test_prompts" json:"test_prompts"`
	TargetModels    []TargetModel `yaml:"target_models" json:"target_models"`
	Judge           JudgeConfig  `yaml:"judge" json:"judge"`
	CreatedAt       time.Time    `yaml:"-" json:"created_at,omitempty"`
	UpdatedAt       time.Time    `yaml:"-" json:"updated_at,omitempty"`
}

// TestPrompt defines a single test prompt with optional reference answer.
type TestPrompt struct {
	ID              string `yaml:"id" json:"id"`
	Prompt          string `yaml:"prompt" json:"prompt"`
	ReferenceAnswer string `yaml:"reference_answer,omitempty" json:"reference_answer,omitempty"`
}

// TargetModel defines a model/upstream to evaluate.
type TargetModel struct {
	Name     string `yaml:"name" json:"name"`         // display name
	Upstream string `yaml:"upstream" json:"upstream"` // upstream name in gateway config
	Model    string `yaml:"model" json:"model"`       // model name to pass in request
}

// JudgeConfig configures how responses are scored.
type JudgeConfig struct {
	Type         JudgeType `yaml:"type" json:"type"` // llm_judge, semantic_similarity, exact_match
	JudgeUpstream string   `yaml:"judge_upstream,omitempty" json:"judge_upstream,omitempty"`
	JudgeModel   string    `yaml:"judge_model,omitempty" json:"judge_model,omitempty"`
	Threshold    float64   `yaml:"threshold,omitempty" json:"threshold,omitempty"` // passing threshold (default 0.7)
}

// JudgeType defines the scoring strategy.
type JudgeType string

const (
	JudgeLLM             JudgeType = "llm_judge"
	JudgeSemantic        JudgeType = "semantic_similarity"
	JudgeExactMatch      JudgeType = "exact_match"
)
