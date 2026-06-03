package semantic

import (
	"context"
	"encoding/json"
	"log/slog"
	"github.com/kongkkongtx/ai-gateway/internal/provider/openai"
	"testing"
	"time"
)

func TestPrewarmerSeedQueries(t *testing.T) {
	cfg := SemanticCacheConfig{
		Enabled:    true,
		MaxEntries: 100,
		TTL:        "10m",
	}
	cache := NewCache(cfg, nil, &mockEmbedder{}, slog.Default())
	pw := NewPrewarmer(cache, PrewarmConfig{
		Enabled:     true,
		SeedQueries: []string{"hello", "what is ai", "explain ml"},
		TopK:        5,
	}, &mockEmbedder{}, slog.Default())

	ctx := context.Background()
	pw.Run(ctx)

	stats := pw.Stats()
	if stats.QueriesAdded != 3 {
		t.Fatalf("expected 3 queries added, got %d", stats.QueriesAdded)
	}
	if stats.LastStatus != "success" {
		t.Fatalf("expected success, got %s", stats.LastStatus)
	}

	cacheStats := cache.Stats()
	if cacheStats.TotalEntries != 3 {
		t.Fatalf("expected 3 cache entries after prewarm, got %d", cacheStats.TotalEntries)
	}
}

func TestPrewarmerHotQueries(t *testing.T) {
	cfg := SemanticCacheConfig{
		Enabled:    true,
		MaxEntries: 100,
		TTL:        "10m",
	}
	cache := NewCache(cfg, nil, &mockEmbedder{}, slog.Default())

	// Manually add some cache entries with hit counts
	cache.mu.Lock()
	emb := []float64{0.1, 0.2, 0.3}
	for i, q := range []string{"query a", "query b", "query c", "query d"} {
		resp, _ := json.Marshal(map[string]string{"response": q})
		e := CacheEntry{
			Query:     q,
			Response:  resp,
			Embedding: emb,
			Timestamp: time.Now(),
			Model:     "test",
			HitCount:  (3 - i) * 10, // 30, 20, 10, 0
			ExpiresAt: time.Now().Add(10 * time.Minute),
		}
		cache.entries = append(cache.entries, e)
	}
	cache.mu.Unlock()

	pw := NewPrewarmer(cache, PrewarmConfig{
		Enabled: true,
		TopK:    2,
	}, &mockEmbedder{}, slog.Default())

	ctx := context.Background()
	pw.Run(ctx)

	stats := pw.Stats()
	if stats.QueriesAdded < 1 {
		t.Fatalf("expected at least 1 hot query warmed, got %d", stats.QueriesAdded)
	}
}

func TestPrewarmerDisabled(t *testing.T) {
	cache := NewCache(SemanticCacheConfig{}, nil, &mockEmbedder{}, slog.Default())
	pw := NewPrewarmer(cache, PrewarmConfig{Enabled: false}, &mockEmbedder{}, slog.Default())

	pw.Run(context.Background())

	stats := pw.Stats()
	if stats.QueriesAdded != 0 {
		t.Fatalf("expected 0 queries when disabled, got %d", stats.QueriesAdded)
	}
}

func TestPrewarmerEmptySeedQueries(t *testing.T) {
	cache := NewCache(SemanticCacheConfig{Enabled: true, MaxEntries: 100}, nil, &mockEmbedder{}, slog.Default())
	pw := NewPrewarmer(cache, PrewarmConfig{
		Enabled:     true,
		SeedQueries: []string{},
		TopK:        0,
	}, &mockEmbedder{}, slog.Default())

	pw.Run(context.Background())

	stats := pw.Stats()
	if stats.QueriesAdded != 0 {
		t.Fatalf("expected 0 with no seed queries, got %d", stats.QueriesAdded)
	}
}

func TestCacheHitCount(t *testing.T) {
	cfg := SemanticCacheConfig{
		Enabled:    true,
		MaxEntries: 10,
	}
	cache := NewCache(cfg, nil, &mockEmbedder{}, slog.Default())

	// Add an entry
	resp := []byte(`{"response":"test"}`)
	cache.Set("test query", resp, "test-model")

	// Get it multiple times
	for i := 0; i < 5; i++ {
		got, _, err := cache.Get("test query", 0.5)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if got == nil {
			t.Fatalf("expected cache hit on iteration %d", i)
		}
	}

	stats := cache.Stats()
	if stats.HitCount < 5 {
		t.Fatalf("expected at least 5 hits, got %d", stats.HitCount)
	}
}

func TestCacheTTLExpiry(t *testing.T) {
	cfg := SemanticCacheConfig{
		Enabled:    true,
		MaxEntries: 10,
		TTL:        "1ms", // very short TTL
	}
	cache := NewCache(cfg, nil, &mockEmbedder{}, slog.Default())

	resp := []byte(`{"response":"test"}`)
	cache.Set("test query", resp, "test-model")

	// Immediately get should hit
	got, _, err := cache.Get("test query", 0.5)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected cache hit before expiry")
	}

	// Wait for TTL to expire
	time.Sleep(5 * time.Millisecond)

	// Get should miss (entry expired)
	got, _, err = cache.Get("test query", 0.5)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != nil {
		t.Fatal("expected cache miss after TTL expiry")
	}
}

func TestCacheStats(t *testing.T) {
	cfg := SemanticCacheConfig{
		Enabled:    true,
		MaxEntries: 50,
	}
	cache := NewCache(cfg, nil, &mockEmbedder{}, slog.Default())

	stats := cache.Stats()
	if stats.StoreType != "memory" {
		t.Fatalf("expected memory store, got %s", stats.StoreType)
	}
	if stats.MaxEntries != 50 {
		t.Fatalf("expected MaxEntries 50, got %d", stats.MaxEntries)
	}

	cache.Set("q1", []byte("r1"), "m1")
	cache.Set("q2", []byte("r2"), "m2")

	stats = cache.Stats()
	if stats.TotalEntries != 2 {
		t.Fatalf("expected 2 entries, got %d", stats.TotalEntries)
	}
}

// mockEmbedder returns a fixed embedding for any text.
type mockEmbedder struct{}

func (m *mockEmbedder) Embed(text string) (*openai.EmbeddingResponse, error) {
	return &openai.EmbeddingResponse{
		Data: []openai.EmbeddingData{
			{
				Object:    "embedding",
				Index:     0,
				Embedding: []float64{0.1, 0.2, 0.3, 0.4, 0.5},
			},
		},
	}, nil
}
