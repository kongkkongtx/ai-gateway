// Package experiment implements A/B testing engine for model routing.
// It allows splitting traffic across multiple upstream providers for the same model,
// tracking per-variant metrics (latency, tokens, error rate), and detecting
// statistically significant differences between variants.
package experiment

import (
	"sync"
	"time"
)

// Config defines the top-level A/B testing configuration.
type Config struct {
	Enabled    bool         `yaml:"enabled" json:"enabled"`
	Experiments []Experiment `yaml:"experiments" json:"experiments"`
}

// Experiment defines a single A/B test experiment.
type Experiment struct {
	ID          string    `yaml:"id" json:"id"`
	Name        string    `yaml:"name" json:"name"`
	Description string    `yaml:"description,omitempty" json:"description,omitempty"`
	Model       string    `yaml:"model" json:"model"` // glob pattern to match model names
	Variants    []Variant `yaml:"variants" json:"variants"`
	Active      bool      `yaml:"active" json:"active"`
	CreatedAt   time.Time `yaml:"-" json:"created_at,omitempty"`
	UpdatedAt   time.Time `yaml:"-" json:"updated_at,omitempty"`
}

// Variant defines a single variant in an A/B experiment.
type Variant struct {
	Name     string `yaml:"name" json:"name"`
	Upstream string `yaml:"upstream" json:"upstream"`
	Weight   int    `yaml:"weight" json:"weight"` // percentage (all variants must sum to 100)
}

// Record stores per-request metrics for a variant.
type Record struct {
	Latency          time.Duration `json:"latency"`
	Success          bool          `json:"success"`
	PromptTokens     int           `json:"prompt_tokens"`
	CompletionTokens int           `json:"completion_tokens"`
}

// VariantMetrics holds aggregated metrics for one variant.
type VariantMetrics struct {
	VariantName                string    `json:"variant_name"`
	Upstream                   string    `json:"upstream"`
	Weight                     int       `json:"weight"`
	RequestCount               int64     `json:"request_count"`
	ErrorCount                 int64     `json:"error_count"`
	ErrorRate                  float64   `json:"error_rate"`
	TotalLatencyMs             float64   `json:"total_latency_ms"`
	AvgLatencyMs               float64   `json:"avg_latency_ms"`
	P50LatencyMs               float64   `json:"p50_latency_ms"`
	P95LatencyMs               float64   `json:"p95_latency_ms"`
	TotalPromptTokens          int64     `json:"total_prompt_tokens"`
	TotalCompletionTokens      int64     `json:"total_completion_tokens"`
	TotalTokens                int64     `json:"total_tokens"`
}

// Result holds aggregated results for an experiment.
type Result struct {
	ExperimentID   string           `json:"experiment_id"`
	ExperimentName string           `json:"experiment_name"`
	VariantCount   int              `json:"variant_count"`
	TotalRequests  int64            `json:"total_requests"`
	Variants       []VariantMetrics `json:"variants"`
}

// store holds raw latency values for percentile calculation.
type latencyStore struct {
	mu     sync.Mutex
	values []float64 // ms
}

func (ls *latencyStore) Add(v float64) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	const maxCapacity = 10000
	if len(ls.values) >= maxCapacity {
		// Drop oldest half to bound memory
		ls.values = ls.values[len(ls.values)/2:]
	}
	ls.values = append(ls.values, v)
}
