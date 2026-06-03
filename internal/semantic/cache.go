package semantic

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/kongkkongtx/ai-gateway/internal/cache"
)

// Cache provides semantic caching of AI responses.
// It stores query-response pairs indexed by their embedding vectors
// and retrieves them based on cosine similarity.
type Cache struct {
	cfg       SemanticCacheConfig
	redis     *cache.Client
	mu        sync.RWMutex
	entries   []CacheEntry
	embedder  Embedder
	logger    *slog.Logger
}

// CacheEntry represents a cached query-response pair.
type CacheEntry struct {
	Query      string
	Response   []byte
	Embedding  []float64
	Timestamp  time.Time
	Model      string
	HitCount   int
	ExpiresAt  time.Time // zero = never expires
}

// NewCache creates a semantic cache.
// When redis is nil, uses in-memory cache (single instance).
// When redis is set, entries are distributed across instances.
func NewCache(cfg SemanticCacheConfig, redis *cache.Client, embedder Embedder, logger *slog.Logger) *Cache {
	return &Cache{
		cfg:      cfg,
		redis:    redis,
		embedder: embedder,
		logger:   logger,
	}
}

// Get looks up a semantically similar cached response for the query.
// Returns the cached response bytes and the similarity score, or nil if no match.
func (sc *Cache) Get(query string, threshold float64) ([]byte, float64, error) {
	if !sc.cfg.Enabled || sc.embedder == nil {
		return nil, 0, nil
	}

	// Compute query embedding
	resp, err := sc.embedder.Embed(query)
	if err != nil {
		return nil, 0, fmt.Errorf("compute embedding: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, 0, nil
	}
	queryEmbed := resp.Data[0].Embedding

	// Expire stale entries
	sc.mu.Lock()
	sc.expireStale()
	sc.mu.Unlock()

	sc.mu.RLock()
	var bestIdx int = -1
	var bestScore float64

	for i, entry := range sc.entries {
		score := CosineSimilarity(queryEmbed, entry.Embedding)
		if score > threshold && score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	if bestIdx >= 0 {
		sc.entries[bestIdx].HitCount++
		resp := sc.entries[bestIdx].Response
		sc.mu.RUnlock()
		sc.logger.Debug("semantic cache hit",
			"score", math.Round(bestScore*1000)/1000)
		return resp, bestScore, nil
	}

	sc.mu.RUnlock()
	return nil, 0, nil
}

// CacheStats holds cache performance statistics.
type CacheStats struct {
	TotalEntries int     `json:"total_entries"`
	MaxEntries   int     `json:"max_entries"`
	HitCount     int64   `json:"hit_count"`
	TotalLookups int64   `json:"total_lookups"`
	HitRate      float64 `json:"hit_rate"`
	StoreType    string  `json:"store_type"`
}

// Stats returns current cache statistics.
func (sc *Cache) Stats() CacheStats {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	var totalHits int64
	for _, e := range sc.entries {
		totalHits += int64(e.HitCount)
	}
	storeType := "memory"
	if sc.redis != nil {
		storeType = "redis"
	}

	return CacheStats{
		TotalEntries: len(sc.entries),
		MaxEntries:   sc.cfg.MaxEntries,
		HitCount:     totalHits,
		StoreType:    storeType,
	}
}

// expireStale removes expired cache entries.
// Must be called with at least a read lock held.
func (sc *Cache) expireStale() {
	ttlStr := sc.cfg.TTL
	if ttlStr == "" {
		return // no TTL configured
	}
	ttl, err := time.ParseDuration(ttlStr)
	if err != nil || ttl <= 0 {
		return
	}
	now := time.Now()
	var kept []CacheEntry
	for _, e := range sc.entries {
		if !e.ExpiresAt.IsZero() && now.After(e.ExpiresAt) {
			continue // skip expired
		}
		kept = append(kept, e)
	}
	sc.entries = kept
}

// Set caches a response for the given query.
func (sc *Cache) Set(query string, response []byte, model string) {
	if !sc.cfg.Enabled || sc.embedder == nil {
		return
	}

	resp, err := sc.embedder.Embed(query)
	if err != nil {
		sc.logger.Warn("failed to compute cache embedding", "error", err)
		return
	}
	if len(resp.Data) == 0 {
		return
	}

	entry := CacheEntry{
		Query:     query,
		Response:  response,
		Embedding: resp.Data[0].Embedding,
		Timestamp: time.Now(),
		Model:     model,
		ExpiresAt: sc.computeExpiry(),
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Evict oldest if at capacity
	if sc.cfg.MaxEntries > 0 && len(sc.entries) >= sc.cfg.MaxEntries {
		oldest := 0
		for i := 1; i < len(sc.entries); i++ {
			if sc.entries[i].Timestamp.Before(sc.entries[oldest].Timestamp) {
				oldest = i
			}
		}
		sc.entries = append(sc.entries[:oldest], sc.entries[oldest+1:]...)
	}

	sc.entries = append(sc.entries, entry)
	sc.logger.Debug("semantic cache set", "model", model, "entries", len(sc.entries))
}

// computeExpiry returns the expiration time based on TTL config.
func (sc *Cache) computeExpiry() time.Time {
	if sc.cfg.TTL == "" {
		return time.Time{} // zero = never expires
	}
	ttl, err := time.ParseDuration(sc.cfg.TTL)
	if err != nil || ttl <= 0 {
		return time.Time{}
	}
	return time.Now().Add(ttl)
}

// Embedder returns the cache's embedder, or nil if none was set.
func (sc *Cache) Embedder() Embedder {
	return sc.embedder
}

// ToRedis serializes and stores cache entries in Redis.
func (sc *Cache) ToRedis(ctx context.Context) error {
	if sc.redis == nil || !sc.cfg.Enabled {
		return nil
	}
	sc.mu.RLock()
	data, err := json.Marshal(sc.entries)
	sc.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}
	return sc.redis.SetRaw(ctx, "semantic:cache", data)
}

// FromRedis loads cache entries from Redis.
func (sc *Cache) FromRedis(ctx context.Context) error {
	if sc.redis == nil || !sc.cfg.Enabled {
		return nil
	}
	data, err := sc.redis.GetRaw(ctx, "semantic:cache")
	if err != nil {
		return err
	}
	if data == nil {
		return nil
	}
	var entries []CacheEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("unmarshal cache: %w", err)
	}
	sc.mu.Lock()
	sc.entries = entries
	sc.mu.Unlock()
	return nil
}
