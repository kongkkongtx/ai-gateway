package semantic

import (
	"context"
	"encoding/json"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// Prewarmer periodically warms the semantic cache with seed queries
// and frequently accessed hot queries.
type Prewarmer struct {
	cache   *Cache
	cfg     PrewarmConfig
	embedder Embedder
	logger  *slog.Logger
	stopCh  chan struct{}
	mu      sync.Mutex
	running bool
	stats   PrewarmStats
}

// PrewarmStats tracks prewarming run history.
type PrewarmStats struct {
	LastRunAt    time.Time `json:"last_run_at"`
	LastDuration string    `json:"last_duration"`
	QueriesAdded int       `json:"queries_added"`
	Errors       int       `json:"errors"`
	TotalRuns    int       `json:"total_runs"`
	LastStatus   string    `json:"last_status"` // "success" | "failed"
}

// NewPrewarmer creates a new Prewarmer.
// embedder may be the same one used by the cache, or nil (prewarming disabled).
func NewPrewarmer(cache *Cache, cfg PrewarmConfig, embedder Embedder, logger *slog.Logger) *Prewarmer {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 3
	}
	if cfg.TopK <= 0 {
		cfg.TopK = 10
	}
	return &Prewarmer{
		cache:    cache,
		cfg:      cfg,
		embedder: embedder,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}
}

// Start begins periodic prewarming in the background.
func (p *Prewarmer) Start(ctx context.Context) {
	if !p.cfg.Enabled {
		return
	}
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.mu.Unlock()

	// Run immediately on start
	p.Run(ctx)

	// Then run periodically
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				p.Run(ctx)
			case <-p.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	p.logger.Info("cache prewarmer started", "seed_queries", len(p.cfg.SeedQueries), "topK", p.cfg.TopK)
}

// Stop signals the prewarmer to stop.
func (p *Prewarmer) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running {
		return
	}
	close(p.stopCh)
	p.running = false
	p.logger.Info("cache prewarmer stopped")
}

// Run executes one complete prewarming cycle.
func (p *Prewarmer) Run(ctx context.Context) {
	if !p.cfg.Enabled || p.embedder == nil {
		return
	}

	start := time.Now()
	p.logger.Info("cache prewarm run started")
	totalAdded := 0
	totalErrors := 0

	// Phase 1: Warm seed queries
	if len(p.cfg.SeedQueries) > 0 {
		added, errs := p.warmSeedQueries(ctx)
		totalAdded += added
		totalErrors += errs
	}

	// Phase 2: Warm hot queries from cache stats
	if p.cfg.TopK > 0 {
		added, errs := p.warmHotQueries(ctx)
		totalAdded += added
		totalErrors += errs
	}

	duration := time.Since(start)
	status := "success"
	if totalErrors > 0 {
		status = "failed"
	}

	p.mu.Lock()
	p.stats = PrewarmStats{
		LastRunAt:    start,
		LastDuration: duration.Round(time.Millisecond).String(),
		QueriesAdded: totalAdded,
		Errors:       totalErrors,
		TotalRuns:    p.stats.TotalRuns + 1,
		LastStatus:   status,
	}
	p.mu.Unlock()

	p.logger.Info("cache prewarm run completed",
		"added", totalAdded,
		"errors", totalErrors,
		"duration", duration.Round(time.Millisecond),
	)
}

// Stats returns current prewarming statistics.
func (p *Prewarmer) Stats() PrewarmStats {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stats
}

// warmSeedQueries embeds and caches the configured seed queries.
func (p *Prewarmer) warmSeedQueries(ctx context.Context) (added, errors int) {
	for _, query := range p.cfg.SeedQueries {
		select {
		case <-ctx.Done():
			return
		default:
		}

		resp, err := p.embedder.Embed(query)
		if err != nil {
			p.logger.Warn("prewarm seed query embedding failed", "query", truncate(query, 30), "error", err)
			errors++
			continue
		}
		if len(resp.Data) == 0 || len(resp.Data[0].Embedding) == 0 {
			errors++
			continue
		}

		// Create placeholder response - just enough for cache hit to return
		placeholderResp, _ := json.Marshal(map[string]interface{}{
			"id":      "prewarmed-cache",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "cache",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "I'm a cached response. Please resend your query if this is returned.",
					},
					"finish_reason": "stop",
				},
			},
		})

		p.cache.mu.Lock()
		entry := CacheEntry{
			Query:     query,
			Response:  placeholderResp,
			Embedding: resp.Data[0].Embedding,
			Timestamp: time.Now(),
			Model:     "prewarm",
			ExpiresAt: p.cache.computeExpiry(),
		}

		// Evict oldest if at capacity
		if p.cache.cfg.MaxEntries > 0 && len(p.cache.entries) >= p.cache.cfg.MaxEntries {
			oldest := 0
			for i := 1; i < len(p.cache.entries); i++ {
				if p.cache.entries[i].Timestamp.Before(p.cache.entries[oldest].Timestamp) {
					oldest = i
				}
			}
			p.cache.entries = append(p.cache.entries[:oldest], p.cache.entries[oldest+1:]...)
		}

		p.cache.entries = append(p.cache.entries, entry)
		p.cache.mu.Unlock()
		added++
	}
	return
}

// warmHotQueries re-embeds and refreshes the most popular cached queries.
func (p *Prewarmer) warmHotQueries(ctx context.Context) (added, errors int) {
	p.cache.mu.RLock()
	type hotEntry struct {
		index int
		count int
	}
	var hot []hotEntry
	for i, e := range p.cache.entries {
		if e.HitCount > 0 {
			hot = append(hot, hotEntry{index: i, count: e.HitCount})
		}
	}
	p.cache.mu.RUnlock()

	// Sort by hit count descending
	sort.Slice(hot, func(i, j int) bool { return hot[i].count > hot[j].count })

	// Take top K
	if len(hot) > p.cfg.TopK {
		hot = hot[:p.cfg.TopK]
	}

	for _, h := range hot {
		select {
		case <-ctx.Done():
			return
		default:
		}

		p.cache.mu.RLock()
		query := p.cache.entries[h.index].Query
		p.cache.mu.RUnlock()

		resp, err := p.embedder.Embed(query)
		if err != nil {
			p.logger.Warn("prewarm hot query embedding failed", "query", truncate(query, 30), "error", err)
			errors++
			continue
		}
		if len(resp.Data) == 0 || len(resp.Data[0].Embedding) == 0 {
			errors++
			continue
		}

		p.cache.mu.Lock()
		if h.index < len(p.cache.entries) {
			p.cache.entries[h.index].Embedding = resp.Data[0].Embedding
			p.cache.entries[h.index].Timestamp = time.Now()
			p.cache.entries[h.index].ExpiresAt = p.cache.computeExpiry()
		}
		p.cache.mu.Unlock()
		added++
	}
	return
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
