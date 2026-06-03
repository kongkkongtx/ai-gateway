package experiment

import (
	"log/slog"
	"math"
	"sync"
	"testing"
	"time"
)

func testEngine() *Engine {
	cfg := Config{
		Enabled: true,
		Experiments: []Experiment{
			{
				ID:     "exp-1",
				Name:   "GPT-4o Comparison",
				Model:  "gpt-4o*",
				Active: true,
				Variants: []Variant{
					{Name: "v1", Upstream: "openai-us", Weight: 70},
					{Name: "v2", Upstream: "openai-eu", Weight: 30},
				},
			},
			{
				ID:     "exp-2",
				Name:   "Claude Test",
				Model:  "claude-*",
				Active: false,
				Variants: []Variant{
					{Name: "c1", Upstream: "anthropic-us", Weight: 50},
					{Name: "c2", Upstream: "anthropic-eu", Weight: 50},
				},
			},
		},
	}
	return NewEngine(cfg, slog.Default())
}

func TestMatchExact(t *testing.T) {
	e := testEngine()
	exp := e.Match("gpt-4o-mini")
	if exp == nil {
		t.Fatal("expected match for gpt-4o-mini")
	}
	if exp.ID != "exp-1" {
		t.Fatalf("expected exp-1, got %s", exp.ID)
	}
}

func TestMatchGlob(t *testing.T) {
	e := testEngine()
	// exp-1 has Model "gpt-4o*", so "gpt-4o-turbo" should match
	exp := e.Match("gpt-4o-turbo")
	if exp == nil {
		t.Fatal("expected match for gpt-4o-turbo against gpt-4o*")
	}
	if exp.ID != "exp-1" {
		t.Fatalf("expected exp-1, got %s", exp.ID)
	}
}

func TestMatchInactive(t *testing.T) {
	e := testEngine()
	// exp-2 has Model "claude-*" but is inactive
	exp := e.Match("claude-3-5-sonnet")
	if exp != nil {
		t.Fatal("expected no match for inactive experiment")
	}
}

func TestMatchWildcard(t *testing.T) {
	cfg := Config{
		Enabled: true,
		Experiments: []Experiment{
			{
				ID: "exp-catchall", Name: "All Models", Model: "*", Active: true,
				Variants: []Variant{{Name: "default", Upstream: "primary", Weight: 100}},
			},
		},
	}
	e := NewEngine(cfg, slog.Default())
	exp := e.Match("any-model-v1")
	if exp == nil {
		t.Fatal("expected match for catch-all *")
	}
	if exp.ID != "exp-catchall" {
		t.Fatalf("expected exp-catchall, got %s", exp.ID)
	}
}

func TestMatchNoMatch(t *testing.T) {
	cfg := Config{
		Enabled: true,
		Experiments: []Experiment{
			{ID: "exp-specific", Name: "Specific", Model: "gpt-4-turbo", Active: true,
				Variants: []Variant{{Name: "v1", Upstream: "up", Weight: 100}}},
		},
	}
	e := NewEngine(cfg, slog.Default())
	exp := e.Match("gpt-3.5-turbo")
	if exp != nil {
		t.Fatal("expected no match for different model")
	}
}

func TestSelectVariantWeightedDistribution(t *testing.T) {
	exp := &Experiment{
		ID: "test", Name: "Test", Model: "*", Active: true,
		Variants: []Variant{
			{Name: "a", Upstream: "up-a", Weight: 70},
			{Name: "b", Upstream: "up-b", Weight: 30},
		},
	}

	counts := map[string]int{"a": 0, "b": 0}
	iterations := 10000
	for i := 0; i < iterations; i++ {
		v := SelectVariant(exp)
		counts[v.Name]++
	}

	// Check distribution is roughly 70/30 within 5% tolerance
	ratioA := float64(counts["a"]) / float64(iterations)
	if math.Abs(ratioA-0.70) > 0.05 {
		t.Fatalf("expected ~70%% for variant a, got %.2f%%", ratioA*100)
	}
}

func TestSelectVariantSingle(t *testing.T) {
	exp := &Experiment{
		ID: "test", Name: "Test", Model: "*", Active: true,
		Variants: []Variant{{Name: "only", Upstream: "up-only", Weight: 100}},
	}
	for i := 0; i < 100; i++ {
		v := SelectVariant(exp)
		if v.Name != "only" {
			t.Fatalf("expected only variant, got %s", v.Name)
		}
	}
}

func TestSelectVariantEmpty(t *testing.T) {
	exp := &Experiment{
		ID: "test", Name: "Test", Model: "*", Active: true,
		Variants: []Variant{},
	}
	v := SelectVariant(exp)
	if v != nil {
		t.Fatal("expected nil for empty variants")
	}
}

func TestRecordAndGetResults(t *testing.T) {
	e := testEngine()

	// Record some metrics
	records := []struct {
		variant string
		success bool
		latency time.Duration
		prompt  int
		complet int
	}{
		{"v1", true, 100 * time.Millisecond, 50, 100},
		{"v1", true, 200 * time.Millisecond, 60, 120},
		{"v1", false, 300 * time.Millisecond, 70, 140},
		{"v2", true, 150 * time.Millisecond, 30, 60},
		{"v2", true, 250 * time.Millisecond, 40, 80},
	}

	for _, r := range records {
		e.Record("exp-1", r.variant, Record{
			Latency:          r.latency,
			Success:          r.success,
			PromptTokens:     r.prompt,
			CompletionTokens: r.complet,
		})
	}

	res := e.GetResults("exp-1")
	if res == nil {
		t.Fatal("expected results")
	}
	if res.ExperimentID != "exp-1" {
		t.Fatalf("expected exp-1, got %s", res.ExperimentID)
	}

	// Check v1 metrics
	var v1, v2 *VariantMetrics
	for i := range res.Variants {
		switch res.Variants[i].VariantName {
		case "v1":
			v1 = &res.Variants[i]
		case "v2":
			v2 = &res.Variants[i]
		}
	}
	if v1 == nil || v2 == nil {
		t.Fatal("expected both variants in results")
	}

	if v1.RequestCount != 3 {
		t.Fatalf("expected v1 request count 3, got %d", v1.RequestCount)
	}
	if v1.ErrorCount != 1 {
		t.Fatalf("expected v1 error count 1, got %d", v1.ErrorCount)
	}
	if v1.ErrorRate != 1.0/3.0 {
		t.Fatalf("expected v1 error rate 0.333, got %f", v1.ErrorRate)
	}
	if v1.TotalPromptTokens != 180 {
		t.Fatalf("expected v1 prompt tokens 180, got %d", v1.TotalPromptTokens)
	}

	if v2.RequestCount != 2 {
		t.Fatalf("expected v2 request count 2, got %d", v2.RequestCount)
	}
	if v2.ErrorCount != 0 {
		t.Fatalf("expected v2 error count 0, got %d", v2.ErrorCount)
	}
	if v2.TotalTokens != 210 {
		t.Fatalf("expected v2 total tokens 210, got %d", v2.TotalTokens)
	}

	// Check total
	if res.TotalRequests != 5 {
		t.Fatalf("expected total requests 5, got %d", res.TotalRequests)
	}
}

func TestGetResultsNonexistent(t *testing.T) {
	e := testEngine()
	res := e.GetResults("does-not-exist")
	if res != nil {
		t.Fatal("expected nil for nonexistent experiment")
	}
}

func TestAddExperiment(t *testing.T) {
	e := testEngine()
	err := e.AddExperiment(Experiment{
		Name:  "New Test",
		Model: "new-model",
		Variants: []Variant{
			{Name: "a", Upstream: "up-a", Weight: 50},
			{Name: "b", Upstream: "up-b", Weight: 50},
		},
	})
	if err != nil {
		t.Fatalf("AddExperiment failed: %v", err)
	}

	exp := e.GetExperiment("exp-1") // old one should still be there
	if exp == nil {
		t.Fatal("expected old experiment to still exist")
	}

	// Find the newly added one - it should exist in the list
	all := e.ListExperiments()
	found := false
	for _, ex := range all {
		if ex.Name == "New Test" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected new experiment in list")
	}
}

func TestAddExperimentValidation(t *testing.T) {
	e := testEngine()
	tests := []struct {
		name string
		exp  Experiment
	}{
		{"empty name", Experiment{Model: "*", Variants: []Variant{{Name: "v", Upstream: "u", Weight: 100}}}},
		{"empty model", Experiment{Name: "test", Variants: []Variant{{Name: "v", Upstream: "u", Weight: 100}}}},
		{"no variants", Experiment{Name: "test", Model: "*", Variants: []Variant{}}},
		{"weight not 100", Experiment{Name: "test", Model: "*", Variants: []Variant{
			{Name: "a", Upstream: "u-a", Weight: 50},
		}}},
		{"zero weight", Experiment{Name: "test", Model: "*", Variants: []Variant{
			{Name: "a", Upstream: "u-a", Weight: 0},
		}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := e.AddExperiment(tt.exp); err == nil {
				t.Fatal("expected error for invalid experiment")
			}
		})
	}
}

func TestRemoveExperiment(t *testing.T) {
	e := testEngine()
	ok := e.RemoveExperiment("exp-1")
	if !ok {
		t.Fatal("expected removal to succeed")
	}
	if e.GetExperiment("exp-1") != nil {
		t.Fatal("expected experiment to be gone")
	}
	ok = e.RemoveExperiment("nonexistent")
	if ok {
		t.Fatal("expected removal to fail for nonexistent")
	}
}

func TestStartStopExperiment(t *testing.T) {
	e := testEngine()
	// exp-2 starts inactive
	if err := e.StartExperiment("exp-2"); err != nil {
		t.Fatalf("StartExperiment failed: %v", err)
	}
	exp := e.GetExperiment("exp-2")
	if exp == nil || !exp.Active {
		t.Fatal("expected experiment to be active")
	}

	if err := e.StopExperiment("exp-2"); err != nil {
		t.Fatalf("StopExperiment failed: %v", err)
	}
	exp = e.GetExperiment("exp-2")
	if exp == nil || exp.Active {
		t.Fatal("expected experiment to be inactive")
	}
}

func TestConcurrentRecording(t *testing.T) {
	e := testEngine()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.Record("exp-1", "v1", Record{
				Latency: 50 * time.Millisecond,
				Success: true,
			})
		}()
	}
	wg.Wait()

	res := e.GetResults("exp-1")
	if res == nil {
		t.Fatal("expected results")
	}
	var v1 *VariantMetrics
	for i := range res.Variants {
		if res.Variants[i].VariantName == "v1" {
			v1 = &res.Variants[i]
			break
		}
	}
	if v1 == nil {
		t.Fatal("expected v1 metrics")
	}
	if v1.RequestCount != 100 {
		t.Fatalf("expected 100 requests, got %d", v1.RequestCount)
	}
}

func TestCheckSignificance(t *testing.T) {
	e := testEngine()

	// v1: 80% error rate (4/5)
	e.Record("exp-1", "v1", Record{Success: false, Latency: 100 * time.Millisecond})
	e.Record("exp-1", "v1", Record{Success: false, Latency: 100 * time.Millisecond})
	e.Record("exp-1", "v1", Record{Success: false, Latency: 100 * time.Millisecond})
	e.Record("exp-1", "v1", Record{Success: false, Latency: 100 * time.Millisecond})
	e.Record("exp-1", "v1", Record{Success: true, Latency: 100 * time.Millisecond})

	// v2: 20% error rate (1/5)
	e.Record("exp-1", "v2", Record{Success: false, Latency: 100 * time.Millisecond})
	e.Record("exp-1", "v2", Record{Success: true, Latency: 100 * time.Millisecond})
	e.Record("exp-1", "v2", Record{Success: true, Latency: 100 * time.Millisecond})
	e.Record("exp-1", "v2", Record{Success: true, Latency: 100 * time.Millisecond})
	e.Record("exp-1", "v2", Record{Success: true, Latency: 100 * time.Millisecond})

	msg := e.CheckSignificance("exp-1")
	if msg == "" {
		t.Fatal("expected significance message for error rate divergence")
	}
}

func TestCheckSignificanceNoSignificance(t *testing.T) {
	e := testEngine()
	// All similar metrics
	for i := 0; i < 10; i++ {
		e.Record("exp-1", "v1", Record{Success: true, Latency: 100 * time.Millisecond})
		e.Record("exp-1", "v2", Record{Success: true, Latency: 105 * time.Millisecond})
	}
	msg := e.CheckSignificance("exp-1")
	if msg != "" {
		t.Fatalf("expected no significance, got: %s", msg)
	}
}

func TestPercentile(t *testing.T) {
	data := []float64{10, 20, 30, 40}

	tests := []struct {
		p    int
		want float64
	}{
		{0, 10},
		{50, 25},  // linear interpolation between 20 and 30: 20 + 0.5*10
		{100, 40},
	}
	for _, tt := range tests {
		got := percentile(data, tt.p)
		if got != tt.want {
			t.Fatalf("percentile(%d) = %f, want %f", tt.p, got, tt.want)
		}
	}
}

func TestPercentileEmpty(t *testing.T) {
	if got := percentile(nil, 50); got != 0 {
		t.Fatalf("expected 0 for empty, got %f", got)
	}
}

func TestListExperiments(t *testing.T) {
	e := testEngine()
	all := e.ListExperiments()
	if len(all) != 2 {
		t.Fatalf("expected 2 experiments, got %d", len(all))
	}
}

func TestUpdateExperiment(t *testing.T) {
	e := testEngine()
	err := e.UpdateExperiment(Experiment{
		ID:    "exp-1",
		Name:  "Updated Name",
		Model: "updated-*",
		Variants: []Variant{
			{Name: "new-v1", Upstream: "new-up", Weight: 100},
		},
		Active: true,
	})
	if err != nil {
		t.Fatalf("UpdateExperiment failed: %v", err)
	}
	exp := e.GetExperiment("exp-1")
	if exp == nil {
		t.Fatal("expected experiment after update")
	}
	if exp.Name != "Updated Name" {
		t.Fatalf("expected Updated Name, got %s", exp.Name)
	}
	if exp.Model != "updated-*" {
		t.Fatalf("expected updated-*, got %s", exp.Model)
	}
	if len(exp.Variants) != 1 || exp.Variants[0].Name != "new-v1" {
		t.Fatal("expected updated variants")
	}
}

func TestUpdateExperimentNotFound(t *testing.T) {
	e := testEngine()
	err := e.UpdateExperiment(Experiment{ID: "does-not-exist"})
	if err == nil {
		t.Fatal("expected error for nonexistent experiment")
	}
}

func TestMatchAndSelect(t *testing.T) {
	e := testEngine()
	exp, variant := e.MatchAndSelect("gpt-4o")
	if exp == nil {
		t.Fatal("expected experiment match")
	}
	if variant == nil {
		t.Fatal("expected variant selection")
	}
	if variant.Upstream == "" {
		t.Fatal("expected variant to have upstream")
	}
}

func TestMatchAndSelectNoMatch(t *testing.T) {
	e := testEngine()
	// Deactivate all
	e.StopExperiment("exp-1")
	e.StopExperiment("exp-3")
	exp, variant := e.MatchAndSelect("gpt-4o")
	if exp != nil || variant != nil {
		t.Fatal("expected nil for no match")
	}
}

func TestReload(t *testing.T) {
	e := testEngine()
	cfg := Config{
		Enabled: true,
		Experiments: []Experiment{
			{
				ID: "exp-new", Name: "After Reload", Model: "new-*", Active: true,
				Variants: []Variant{{Name: "v", Upstream: "up", Weight: 100}},
			},
		},
	}
	e.Reload(cfg)
	all := e.ListExperiments()
	if len(all) != 1 {
		t.Fatalf("expected 1 experiment after reload, got %d", len(all))
	}
	if all[0].Name != "After Reload" {
		t.Fatalf("expected 'After Reload', got %s", all[0].Name)
	}
}
