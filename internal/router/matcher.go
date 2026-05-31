package router

import (
	"path/filepath"
	"sort"
	"sync"

	"github.com/yushi/ai-gateway/internal/config"
)

type Route struct {
	ID        string
	Model     string
	Upstream  string
	Fallbacks []string
	Priority  int
}

type Matcher struct {
	mu     sync.RWMutex
	routes []Route
}

func NewMatcher(routes []config.RouteConfig) *Matcher {
	m := &Matcher{}
	m.Update(routes)
	return m
}

func (m *Matcher) Update(routes []config.RouteConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.routes = make([]Route, len(routes))
	for i, r := range routes {
		fallbacks := make([]string, len(r.Fallbacks))
		copy(fallbacks, r.Fallbacks)
		m.routes[i] = Route{
			ID: r.ID, Model: r.Model, Upstream: r.Upstream,
			Fallbacks: fallbacks, Priority: r.Priority,
		}
	}
	sort.Slice(m.routes, func(i, j int) bool {
		return m.routes[i].Priority > m.routes[j].Priority
	})
}

// Match returns the full Route (with fallbacks) matching the given model string.
// Returns nil if no route matches.
func (m *Matcher) Match(model string) *Route {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, route := range m.routes {
		matched, _ := filepath.Match(route.Model, model)
		if matched {
			return &route
		}
	}
	return nil
}

// MatchSimple returns just the upstream name. Returns empty string and false if unmatched.
func (m *Matcher) MatchSimple(model string) (string, bool) {
	r := m.Match(model)
	if r == nil {
		return "", false
	}
	return r.Upstream, true
}

func (m *Matcher) Routes() []Route {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Route, len(m.routes))
	copy(result, m.routes)
	return result
}