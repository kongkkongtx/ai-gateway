package router

import (
	"testing"

	"github.com/kongkkongtx/ai-gateway/internal/config"
)

func TestMatchExact(t *testing.T) {
	routes := []config.RouteConfig{
		{ID: "r1", Model: "gpt-4", Upstream: "openai-main", Priority: 100},
	}
	m := NewMatcher(routes)
	route := m.Match("gpt-4")
	if route == nil {
		t.Fatal("expected match for gpt-4")
	}
	if route.Upstream != "openai-main" {
		t.Errorf("expected openai-main, got %s", route.Upstream)
	}
}

func TestMatchGlob(t *testing.T) {
	routes := []config.RouteConfig{
		{ID: "r1", Model: "gpt-4*", Upstream: "openai-main", Priority: 100},
		{ID: "r2", Model: "claude-*", Upstream: "anthropic-main", Priority: 90},
		{ID: "r3", Model: "deepseek-*", Upstream: "deepseek-primary", Priority: 80,
			Fallbacks: []string{"openai-main"}},
	}
	m := NewMatcher(routes)

	tests := []struct {
		model string
		want  string
		match bool
	}{
		{"gpt-4", "openai-main", true},
		{"gpt-4-turbo", "openai-main", true},
		{"claude-3-opus", "anthropic-main", true},
		{"deepseek-chat", "deepseek-primary", true},
		{"deepseek-reasoner", "deepseek-primary", true},
		{"unknown", "", false},
	}
	for _, tt := range tests {
		route := m.Match(tt.model)
		if (route != nil) != tt.match {
			t.Errorf("Match(%q): got found=%v, want %v", tt.model, route != nil, tt.match)
			continue
		}
		if route != nil && route.Upstream != tt.want {
			t.Errorf("Match(%q): got upstream=%q, want %q", tt.model, route.Upstream, tt.want)
		}
	}

	// Verify deepseek route has fallbacks
	route := m.Match("deepseek-chat")
	if route == nil {
		t.Fatal("expected match for deepseek-chat")
	}
	if len(route.Fallbacks) != 1 || route.Fallbacks[0] != "openai-main" {
		t.Errorf("expected fallback [openai-main], got %v", route.Fallbacks)
	}
}

func TestMatchPriority(t *testing.T) {
	routes := []config.RouteConfig{
		{ID: "r1", Model: "gpt-4*", Upstream: "specific", Priority: 100},
		{ID: "r2", Model: "*", Upstream: "catch-all", Priority: 0},
	}
	m := NewMatcher(routes)

	route := m.Match("gpt-4-turbo")
	if route == nil {
		t.Fatal("expected match")
	}
	if route.Upstream != "specific" {
		t.Errorf("expected specific (higher priority), got %s", route.Upstream)
	}

	route = m.Match("claude-3")
	if route == nil {
		t.Fatal("expected match on catch-all")
	}
	if route.Upstream != "catch-all" {
		t.Errorf("expected catch-all, got %s", route.Upstream)
	}
}

func TestMatchSimple(t *testing.T) {
	routes := []config.RouteConfig{
		{ID: "r1", Model: "gpt-4", Upstream: "openai", Priority: 100},
	}
	m := NewMatcher(routes)
	upstream, found := m.MatchSimple("gpt-4")
	if !found || upstream != "openai" {
		t.Errorf("MatchSimple failed: got %q, %v", upstream, found)
	}
	_, found = m.MatchSimple("unknown")
	if found {
		t.Error("MatchSimple should return false for unknown model")
	}
}

func TestUpdateRoutes(t *testing.T) {
	m := NewMatcher(nil)
	if m.Match("gpt-4") != nil {
		t.Fatal("expected no match with empty routes")
	}
	routes := []config.RouteConfig{
		{ID: "r1", Model: "gpt-4", Upstream: "openai", Priority: 100},
	}
	m.Update(routes)
	route := m.Match("gpt-4")
	if route == nil || route.Upstream != "openai" {
		t.Errorf("expected match after update")
	}
}