package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"golang.org/x/time/rate"

	"github.com/yushi/ai-gateway/internal/cache"
	"github.com/yushi/ai-gateway/internal/config"
)

// RateLimiter provides request rate limiting with Redis or in-memory backend.
// When a Redis client is configured, it uses Redis-backed sliding window rate limiting
// for distributed enforcement. Otherwise, it falls back to in-memory token bucket.
type RateLimiter struct {
	enabled      bool
	redisClient  *cache.Client
	globalRule   *config.RateLimitRule
	perKeyRules  []config.RateLimitRule
	globalLimiter *rate.Limiter
	perKeyLimit  map[string]*rate.Limiter
	mu           sync.Mutex
	logger       *slog.Logger
}

// NewRateLimiter creates a rate limiter from config.
// When redisClient is non-nil and addr is set, it uses Redis for distributed rate limiting.
func NewRateLimiter(cfg config.RateLimitConfig, redisClient *cache.Client, logger *slog.Logger) *RateLimiter {
	rl := &RateLimiter{
		enabled:     cfg.Enabled,
		redisClient: redisClient,
		globalRule:  cfg.Global,
		perKeyRules: cfg.PerKey,
		perKeyLimit: make(map[string]*rate.Limiter),
		logger:      logger,
	}

	if cfg.Enabled && redisClient == nil {
		// In-memory fallback: use configured limits or defaults
		globalRPS := 1000
		globalBurst := 200
		if cfg.Global != nil && cfg.Global.Limit > 0 {
			// Convert window+limit to RPS
			globalRPS = cfg.Global.Limit
			globalBurst = cfg.Global.Limit / 2
			if globalBurst < 1 {
				globalBurst = 1
			}
		}
		rl.globalLimiter = rate.NewLimiter(rate.Limit(globalRPS), globalBurst)

		if cfg.Enabled {
			logger.Info("rate limiter initialized (in-memory backend)",
				"global_rps", globalRPS, "global_burst", globalBurst)
		}
	} else if cfg.Enabled && redisClient != nil {
		logger.Info("rate limiter initialized (Redis backend)",
			"addr", cfg.Global)
	}

	return rl
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Skip rate limiting for admin/metrics paths
		if r.URL.Path == "/admin/health" || r.URL.Path == "/admin/status" || r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		// Try Redis-backed rate limiting first
		if rl.redisClient != nil {
			rl.handleRedisLimiting(w, r, next)
			return
		}

		// Fall back to in-memory rate limiting
		rl.handleMemoryLimiting(w, r, next)
	})
}

func (rl *RateLimiter) handleRedisLimiting(w http.ResponseWriter, r *http.Request, next http.Handler) {
	ctx := r.Context()

	// Check global limit
	if rl.globalRule != nil && rl.globalRule.Limit > 0 {
		allowed, remaining, err := rl.redisClient.Allow(ctx, "ratelimit:global", rl.globalRule.Limit, rl.globalRule.Window)
		if err != nil {
			rl.logger.Error("redis rate limit check failed, falling back to allow", "error", err)
		} else if !allowed {
			w.Header().Set("Retry-After", "1")
			w.Header().Set("X-RateLimit-Type", "global")
			w.Header().Set("X-RateLimit-Remaining", "0")
			http.Error(w, `{"error":{"message":"Rate limit exceeded","type":"rate_limit_error","code":"rate_limited"}}`, http.StatusTooManyRequests)
			return
		} else {
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		}
	}

	// Check per-key limits
	keyName, _ := r.Context().Value(APIKeyNameContextKey).(string)
	if keyName != "" {
		for _, rule := range rl.perKeyRules {
			redisKey := "ratelimit:key:" + keyName
			allowed, remaining, err := rl.redisClient.Allow(ctx, redisKey, rule.Limit, rule.Window)
			if err != nil {
				rl.logger.Error("redis per-key rate limit check failed", "error", err, "key", keyName)
				continue
			}
			if !allowed {
				w.Header().Set("Retry-After", "1")
				w.Header().Set("X-RateLimit-Type", "key")
				w.Header().Set("X-RateLimit-Remaining", "0")
				http.Error(w, `{"error":{"message":"Rate limit exceeded for key","type":"rate_limit_error","code":"rate_limited"}}`, http.StatusTooManyRequests)
				return
			}
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
			break // Only apply first matching per-key rule
		}
	}

	next.ServeHTTP(w, r)
}

func (rl *RateLimiter) handleMemoryLimiting(w http.ResponseWriter, r *http.Request, next http.Handler) {
	if rl.globalLimiter != nil && !rl.globalLimiter.Allow() {
		w.Header().Set("Retry-After", "1")
		w.Header().Set("X-RateLimit-Type", "global")
		http.Error(w, `{"error":{"message":"Rate limit exceeded","type":"rate_limit_error","code":"rate_limited"}}`, http.StatusTooManyRequests)
		return
	}

	keyName, _ := r.Context().Value(APIKeyNameContextKey).(string)
	if keyName != "" {
		limiter := rl.getKeyLimiter(keyName)
		if limiter != nil && !limiter.Allow() {
			w.Header().Set("Retry-After", "1")
			w.Header().Set("X-RateLimit-Type", "key")
			http.Error(w, `{"error":{"message":"Rate limit exceeded for key","type":"rate_limit_error","code":"rate_limited"}}`, http.StatusTooManyRequests)
			return
		}
	}

	next.ServeHTTP(w, r)
}

func (rl *RateLimiter) getKeyLimiter(keyName string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	limiter, ok := rl.perKeyLimit[keyName]
	if !ok {
		rps := 100
		burst := 10
		if len(rl.perKeyRules) > 0 {
			rps = rl.perKeyRules[0].Limit
			burst = rps / 2
			if burst < 1 {
				burst = 1
			}
		}
		limiter = rate.NewLimiter(rate.Limit(rps), burst)
		rl.perKeyLimit[keyName] = limiter
	}
	return limiter
}