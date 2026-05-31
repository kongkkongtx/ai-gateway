package balancer

import (
	"log/slog"
	"testing"
)

func TestAddUpstream(t *testing.T) {
	b := New(slog.Default())
	b.AddUpstream("test", "https://api.openai.com", "openai", 10)
	upstreams := b.Upstreams()
	if len(upstreams) != 1 {
		t.Fatalf("expected 1 upstream, got %d", len(upstreams))
	}
	if upstreams[0].Name != "test" {
		t.Errorf("expected name test, got %s", upstreams[0].Name)
	}
	if !upstreams[0].Healthy.Load() {
		t.Error("expected upstream to be healthy initially")
	}
}

func TestNextWithSingleUpstream(t *testing.T) {
	b := New(slog.Default())
	b.AddUpstream("test", "https://api.openai.com", "openai", 10)
	u := b.Next()
	if u == nil {
		t.Fatal("expected upstream, got nil")
	}
	if u.Name != "test" {
		t.Errorf("expected test, got %s", u.Name)
	}
}

func TestNextWithNoUpstreams(t *testing.T) {
	b := New(slog.Default())
	u := b.Next()
	if u != nil {
		t.Error("expected nil when no upstreams")
	}
}

func TestNextWithUnhealthyUpstreams(t *testing.T) {
	b := New(slog.Default())
	b.AddUpstream("test", "https://api.openai.com", "openai", 10)
	upstreams := b.Upstreams()
	upstreams[0].Healthy.Store(false)
	u := b.Next()
	if u != nil {
		t.Error("expected nil when all upstreams unhealthy")
	}
}

func TestRemoveUpstream(t *testing.T) {
	b := New(slog.Default())
	b.AddUpstream("a", "https://a.com", "openai", 10)
	b.AddUpstream("b", "https://b.com", "openai", 10)
	b.RemoveUpstream("a")
	upstreams := b.Upstreams()
	if len(upstreams) != 1 {
		t.Fatalf("expected 1 upstream, got %d", len(upstreams))
	}
	if upstreams[0].Name != "b" {
		t.Errorf("expected upstream b to remain, got %s", upstreams[0].Name)
	}
}

func TestWeightedDistribution(t *testing.T) {
	b := New(slog.Default())
	b.AddUpstream("heavy", "https://heavy.com", "openai", 90)
	b.AddUpstream("light", "https://light.com", "openai", 10)

	counts := map[string]int{"heavy": 0, "light": 0}
	for i := 0; i < 1000; i++ {
		u := b.Next()
		if u != nil {
			counts[u.Name]++
		}
	}

	ratio := float64(counts["heavy"]) / float64(counts["light"])
	if ratio < 5 || ratio > 15 {
		t.Errorf("expected heavy/light ratio ~9, got %.2f (heavy=%d, light=%d)", ratio, counts["heavy"], counts["light"])
	}
}