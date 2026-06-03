package experiment

import (
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Engine manages A/B experiments, matching requests to experiments
// and tracking per-variant metrics.
type Engine struct {
	mu          sync.RWMutex
	experiments map[string]*Experiment // keyed by experiment ID
	results     map[string]*experimentResult
	logger      *slog.Logger
}

type experimentResult struct {
	mu       sync.Mutex
	variants map[string]*variantData
}

type variantData struct {
	count            int64
	errors           int64
	totalLatencyMs   float64
	totalPromptTok   int64
	totalCompletionTok int64
	latencies        *latencyStore
}

// NewEngine creates a new experiment engine from config.
func NewEngine(cfg Config, logger *slog.Logger) *Engine {
	e := &Engine{
		experiments: make(map[string]*Experiment),
		results:     make(map[string]*experimentResult),
		logger:      logger,
	}
	for i := range cfg.Experiments {
		exp := cfg.Experiments[i]
		if exp.ID == "" {
			exp.ID = fmt.Sprintf("exp-%d", time.Now().UnixNano())
		}
		e.experiments[exp.ID] = &exp
		e.results[exp.ID] = newExperimentResult(&exp)
	}
	return e
}

func newExperimentResult(exp *Experiment) *experimentResult {
	r := &experimentResult{
		variants: make(map[string]*variantData),
	}
	for _, v := range exp.Variants {
		r.variants[v.Name] = &variantData{
			latencies: &latencyStore{},
		}
	}
	return r
}

// Match finds an active experiment matching the given model name.
// Returns nil if no experiment matches or none are active.
func (e *Engine) Match(model string) *Experiment {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, exp := range e.experiments {
		if !exp.Active {
			continue
		}
		matched, err := filepath.Match(exp.Model, model)
		if err != nil || !matched {
			continue
		}
		return exp
	}
	return nil
}

// MatchAndSelect combines Match and SelectVariant into a single call.
// Returns nil, nil if no experiment matches.
func (e *Engine) MatchAndSelect(model string) (*Experiment, *Variant) {
	exp := e.Match(model)
	if exp == nil {
		return nil, nil
	}
	variant := SelectVariant(exp)
	if variant == nil {
		return nil, nil
	}
	return exp, variant
}

// SelectVariant picks a variant using weighted random selection.
// Returns nil if no variants are configured.
func SelectVariant(exp *Experiment) *Variant {
	if len(exp.Variants) == 0 {
		return nil
	}

	totalWeight := 0
	for _, v := range exp.Variants {
		totalWeight += v.Weight
	}
	if totalWeight <= 0 {
		return nil
	}

	r := rand.Intn(totalWeight)
	for _, v := range exp.Variants {
		r -= v.Weight
		if r < 0 {
			return &Variant{
				Name:     v.Name,
				Upstream: v.Upstream,
				Weight:   v.Weight,
			}
		}
	}

	// Fallback to last variant
	last := exp.Variants[len(exp.Variants)-1]
	return &Variant{
		Name:     last.Name,
		Upstream: last.Upstream,
		Weight:   last.Weight,
	}
}

// Record records metrics for a variant in an experiment.
func (e *Engine) Record(experimentID, variantName string, rec Record) {
	e.mu.RLock()
	res, ok := e.results[experimentID]
	e.mu.RUnlock()
	if !ok {
		return
	}

	res.mu.Lock()
	defer res.mu.Unlock()

	vd, ok := res.variants[variantName]
	if !ok {
		return
	}

	vd.count++
	if !rec.Success {
		vd.errors++
	}
	latencyMs := float64(rec.Latency) / float64(time.Millisecond)
	vd.totalLatencyMs += latencyMs
	vd.latencies.Add(latencyMs)
	vd.totalPromptTok += int64(rec.PromptTokens)
	vd.totalCompletionTok += int64(rec.CompletionTokens)
}

// GetResults returns aggregated results for an experiment.
func (e *Engine) GetResults(experimentID string) *Result {
	e.mu.RLock()
	exp, ok := e.experiments[experimentID]
	res, resOk := e.results[experimentID]
	e.mu.RUnlock()
	if !ok || !resOk {
		return nil
	}

	result := &Result{
		ExperimentID:   exp.ID,
		ExperimentName: exp.Name,
	}

	for _, v := range exp.Variants {
		res.mu.Lock()
		vd := res.variants[v.Name]
		res.mu.Unlock()

		vm := VariantMetrics{
			VariantName: v.Name,
			Upstream:    v.Upstream,
			Weight:      v.Weight,
		}

		if vd != nil && vd.count > 0 {
			vm.RequestCount = vd.count
			vm.ErrorCount = vd.errors
			vm.ErrorRate = float64(vd.errors) / float64(vd.count)
			vm.TotalLatencyMs = vd.totalLatencyMs
			vm.AvgLatencyMs = vd.totalLatencyMs / float64(vd.count)
			vm.TotalPromptTokens = vd.totalPromptTok
			vm.TotalCompletionTokens = vd.totalCompletionTok
			vm.TotalTokens = vd.totalPromptTok + vd.totalCompletionTok

			// Percentiles
			vd.latencies.mu.Lock()
			sorted := make([]float64, len(vd.latencies.values))
			copy(sorted, vd.latencies.values)
			vd.latencies.mu.Unlock()
			sort.Float64s(sorted)
			if len(sorted) > 0 {
				vm.P50LatencyMs = percentile(sorted, 50)
				vm.P95LatencyMs = percentile(sorted, 95)
			}
		}

		result.Variants = append(result.Variants, vm)
		result.TotalRequests += vm.RequestCount
	}
	result.VariantCount = len(result.Variants)

	return result
}

// AddExperiment adds a new experiment or updates an existing one.
func (e *Engine) AddExperiment(exp Experiment) error {
	if exp.Name == "" {
		return fmt.Errorf("experiment name is required")
	}
	if exp.Model == "" {
		return fmt.Errorf("experiment model pattern is required")
	}
	if len(exp.Variants) == 0 {
		return fmt.Errorf("experiment must have at least one variant")
	}

	totalWeight := 0
	for _, v := range exp.Variants {
		if v.Name == "" {
			return fmt.Errorf("variant name is required")
		}
		if v.Upstream == "" {
			return fmt.Errorf("variant upstream is required for %q", v.Name)
		}
		if v.Weight <= 0 {
			return fmt.Errorf("variant %q weight must be positive", v.Name)
		}
		totalWeight += v.Weight
	}
	if totalWeight != 100 {
		return fmt.Errorf("variant weights must sum to 100, got %d", totalWeight)
	}

	if exp.ID == "" {
		exp.ID = fmt.Sprintf("exp-%d", time.Now().UnixNano())
	}
	exp.CreatedAt = time.Now()
	exp.UpdatedAt = time.Now()

	e.mu.Lock()
	e.experiments[exp.ID] = &exp
	if _, exists := e.results[exp.ID]; !exists {
		e.results[exp.ID] = newExperimentResult(&exp)
	}
	e.mu.Unlock()

	return nil
}

// RemoveExperiment deletes an experiment by ID.
func (e *Engine) RemoveExperiment(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, ok := e.experiments[id]
	if ok {
		delete(e.experiments, id)
		delete(e.results, id)
	}
	return ok
}

// GetExperiment returns a copy of an experiment by ID.
func (e *Engine) GetExperiment(id string) *Experiment {
	e.mu.RLock()
	defer e.mu.RUnlock()
	exp, ok := e.experiments[id]
	if !ok {
		return nil
	}
	copy := *exp
	return &copy
}

// ListExperiments returns all experiments.
func (e *Engine) ListExperiments() []Experiment {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]Experiment, 0, len(e.experiments))
	for _, exp := range e.experiments {
		result = append(result, *exp)
	}
	// Sort by creation time for deterministic output
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result
}

// StartExperiment activates an experiment.
func (e *Engine) StartExperiment(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	exp, ok := e.experiments[id]
	if !ok {
		return fmt.Errorf("experiment %q not found", id)
	}
	exp.Active = true
	exp.UpdatedAt = time.Now()
	return nil
}

// StopExperiment deactivates an experiment.
func (e *Engine) StopExperiment(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	exp, ok := e.experiments[id]
	if !ok {
		return fmt.Errorf("experiment %q not found", id)
	}
	exp.Active = false
	exp.UpdatedAt = time.Now()
	return nil
}

// UpdateExperiment updates a single experiment (replaces existing).
func (e *Engine) UpdateExperiment(exp Experiment) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	existing, ok := e.experiments[exp.ID]
	if !ok {
		return fmt.Errorf("experiment %q not found", exp.ID)
	}

	// Preserve creation time
	exp.CreatedAt = existing.CreatedAt
	exp.UpdatedAt = time.Now()

	e.experiments[exp.ID] = &exp
	// Reinitialize results collection for new variants
	if _, exists := e.results[exp.ID]; !exists {
		e.results[exp.ID] = newExperimentResult(&exp)
	} else {
		// Add any new variant entries
		e.results[exp.ID].mu.Lock()
		for _, v := range exp.Variants {
			if _, exists := e.results[exp.ID].variants[v.Name]; !exists {
				e.results[exp.ID].variants[v.Name] = &variantData{
					latencies: &latencyStore{},
				}
			}
		}
		e.results[exp.ID].mu.Unlock()
	}

	return nil
}

// CheckSignificance checks if any variant shows a significant difference
// in error rate or latency compared to others. Returns a descriptive message
// if significance is detected, empty string otherwise.
func (e *Engine) CheckSignificance(experimentID string) string {
	res := e.GetResults(experimentID)
	if res == nil || len(res.Variants) < 2 {
		return ""
	}

	for i := 0; i < len(res.Variants); i++ {
		for j := i + 1; j < len(res.Variants); j++ {
			a, b := res.Variants[i], res.Variants[j]

			// Check error rate significance (2x threshold)
			if a.ErrorRate > 0 || b.ErrorRate > 0 {
				minErr := math.Min(a.ErrorRate, b.ErrorRate)
				maxErr := math.Max(a.ErrorRate, b.ErrorRate)
				if minErr > 0 && maxErr/minErr > 2.0 {
					return fmt.Sprintf("error rate significant: %s=%.1f%% vs %s=%.1f%%",
						a.VariantName, a.ErrorRate*100, b.VariantName, b.ErrorRate*100)
				}
			}

			// Check latency significance (50% difference threshold)
			if a.AvgLatencyMs > 0 && b.AvgLatencyMs > 0 {
				minLat := math.Min(a.AvgLatencyMs, b.AvgLatencyMs)
				maxLat := math.Max(a.AvgLatencyMs, b.AvgLatencyMs)
				if maxLat/minLat > 1.5 {
					return fmt.Sprintf("latency significant: %s=%.0fms vs %s=%.0fms",
						a.VariantName, a.AvgLatencyMs, b.VariantName, b.AvgLatencyMs)
				}
			}
		}
	}
	return ""
}

// Reload replaces all experiments and resets results from new config.
func (e *Engine) Reload(cfg Config) {
	experiments := make(map[string]*Experiment)
	results := make(map[string]*experimentResult)
	for i := range cfg.Experiments {
		exp := cfg.Experiments[i]
		if exp.ID == "" {
			exp.ID = fmt.Sprintf("exp-%d", time.Now().UnixNano())
		}
		experiments[exp.ID] = &exp
		results[exp.ID] = newExperimentResult(&exp)
	}
	e.mu.Lock()
	e.experiments = experiments
	e.results = results
	e.mu.Unlock()
	e.logger.Info("experiment engine reloaded", "count", len(experiments))
}

// percentile computes the p-th percentile from a sorted slice.
func percentile(sorted []float64, p int) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}
	index := (float64(p) / 100.0) * float64(len(sorted)-1)
	floor := int(math.Floor(index))
	ceil := int(math.Ceil(index))
	if floor == ceil {
		return sorted[floor]
	}
	return sorted[floor] + (index-float64(floor))*(sorted[ceil]-sorted[floor])
}
