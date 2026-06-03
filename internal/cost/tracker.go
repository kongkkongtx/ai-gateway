package cost

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/kongkkongtx/ai-gateway/internal/cache"
)

// Config defines cost control and quota management settings.

// QuotaLimit defines a token quota over a time window.

// KeyQuota overrides the default quota for a specific API key.

// DegradeConfig controls what happens when a quota is exceeded.

// Tracker tracks token usage per key per time window.
type Tracker struct {
	mu         sync.RWMutex
	usage      map[string][]usageRecord
	cfg        Config
	logger     *slog.Logger
}

type usageRecord struct {
	InputTokens  int
	OutputTokens int
	Timestamp    time.Time
}

// NewTracker creates a cost tracker.
func NewTracker(cfg Config, logger *slog.Logger) *Tracker {
	return &Tracker{
		usage:  make(map[string][]usageRecord),
		cfg:    cfg,
		logger: logger,
	}
}

// Record adds a usage record for the given key.
func (t *Tracker) Record(keyName string, inputTokens, outputTokens int) {
	if !t.cfg.Enabled {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.usage[keyName] = append(t.usage[keyName], usageRecord{
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Timestamp:    time.Now(),
	})
}

// GetQuota returns the applicable quota for a key.
func (t *Tracker) GetQuota(keyName string) QuotaLimit {
	for _, kq := range t.cfg.KeyLimits {
		if kq.KeyName == keyName {
			return kq.Limit
		}
	}
	return t.cfg.DefaultLimit
}

// Check returns whether the key has exceeded its quota, and current usage info.
func (t *Tracker) Check(keyName string) (allowed bool, inputUsed int, outputUsed int, err error) {
	if !t.cfg.Enabled {
		return true, 0, 0, nil
	}

	quota := t.GetQuota(keyName)
	if quota.InputTokens == 0 && quota.OutputTokens == 0 {
		return true, 0, 0, nil // Unlimited
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	window := 24 * time.Hour
	if quota.Window != "" {
		if parsed, err := time.ParseDuration(quota.Window); err == nil {
			window = parsed
		}
	}
	cutoff := time.Now().Add(-window)

	records := t.usage[keyName]
	for _, r := range records {
		if r.Timestamp.Before(cutoff) {
			continue // Outside window
		}
		inputUsed += r.InputTokens
		outputUsed += r.OutputTokens
	}

	if quota.InputTokens > 0 && inputUsed >= quota.InputTokens {
		return false, inputUsed, outputUsed, nil
	}
	if quota.OutputTokens > 0 && outputUsed >= quota.OutputTokens {
		return false, inputUsed, outputUsed, nil
	}
	return true, inputUsed, outputUsed, nil
}

// UsagePercentage returns how much of the quota has been used (0.0-1.0).
func (t *Tracker) UsagePercentage(keyName string) float64 {
	allowed, inputUsed, outputUsed, _ := t.Check(keyName)
	if allowed {
		return 0
	}
	quota := t.GetQuota(keyName)
	max := 0
	if quota.InputTokens > 0 {
		max = quota.InputTokens
	}
	if quota.OutputTokens > 0 {
		if quota.OutputTokens > max {
			max = quota.OutputTokens
		}
	}
	if max == 0 {
		return 1.0
	}
	used := inputUsed
	if outputUsed > used {
		used = outputUsed
	}
	return float64(used) / float64(max)
}

// ShouldAlert checks if usage has crossed the alert threshold.
func (t *Tracker) ShouldAlert(keyName string) bool {
	if !t.cfg.Enabled || t.cfg.DegradeConfig.AlertThreshold <= 0 {
		return false
	}
	pct := t.UsagePercentage(keyName)
	return pct >= t.cfg.DegradeConfig.AlertThreshold
}

// KeyStat holds aggregated usage for a single API key.
type KeyStat struct {
	KeyName      string `json:"key_name"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	TotalTokens  int    `json:"total_tokens"`
	RecordCount  int    `json:"record_count"`
}
// GetStats returns aggregated usage stats per API key since the given cutoff time.
func (t *Tracker) GetStats(since time.Time) []KeyStat {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var stats []KeyStat
	for keyName, records := range t.usage {
		stat := KeyStat{KeyName: keyName}
		for _, r := range records {
			if r.Timestamp.Before(since) {
				continue
			}
			stat.InputTokens += r.InputTokens
			stat.OutputTokens += r.OutputTokens
			stat.RecordCount++
		}
		stat.TotalTokens = stat.InputTokens + stat.OutputTokens
		if stat.RecordCount > 0 {
			stats = append(stats, stat)
		}
	}
	return stats
}

// SaveToRedis persists usage records to Redis.
func (t *Tracker) SaveToRedis(ctx context.Context, redisCli *cache.Client) error {
	if redisCli == nil || !t.cfg.Enabled {
		return nil
	}
	t.mu.RLock()
	data, err := json.Marshal(t.usage)
	t.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("marshal usage: %w", err)
	}
	return redisCli.SetRaw(ctx, "cost:usage", data)
}

// LoadFromRedis loads usage records from Redis.
func (t *Tracker) LoadFromRedis(ctx context.Context, redisCli *cache.Client) error {
	if redisCli == nil || !t.cfg.Enabled {
		return nil
	}
	data, err := redisCli.GetRaw(ctx, "cost:usage")
	if err != nil {
		return err
	}
	if data == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return json.Unmarshal(data, &t.usage)
}

// TeamStat holds aggregated usage for a team.
type TeamStat struct {
	Team         string `json:"team"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	TotalTokens  int    `json:"total_tokens"`
	KeyCount     int    `json:"key_count"`
}
