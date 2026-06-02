package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
	"github.com/yushi/ai-gateway/internal/prompt"
	"github.com/yushi/ai-gateway/internal/webhook"
	"github.com/yushi/ai-gateway/internal/plugin"

	"github.com/go-chi/chi/v5"
	"github.com/yushi/ai-gateway/internal/server/middleware"
	"github.com/yushi/ai-gateway/internal/security"
	"github.com/yushi/ai-gateway/internal/config"
	"github.com/yushi/ai-gateway/internal/user"
	"github.com/yushi/ai-gateway/internal/provider/openai"
)

// JSON-friendly upstream request with string durations
type upstreamReq struct {
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
	Provider string `json:"provider"`
	Weight   int    `json:"weight"`
	APIToken string `json:"api_token"`
	Timeout  string `json:"timeout"`  // e.g. "30s"
	Model    string `json:"model"`
}

func (u *upstreamReq) toConfig() config.UpstreamConfig {
	timeout := 30 * time.Second
	if d, err := time.ParseDuration(u.Timeout); err == nil && d > 0 {
		timeout = d
	}
	weight := u.Weight
	if weight <= 0 { weight = 1 }
	return config.UpstreamConfig{
		Name: u.Name, Endpoint: u.Endpoint, Provider: u.Provider,
		APIToken: u.APIToken, Model: u.Model,
		Weight: weight, Timeout: timeout,
	}
}

func (g *Gateway) handleListUpstreams(w http.ResponseWriter, r *http.Request) {
	upstreams := g.balancer.Upstreams()
	type item struct {
		Name     string `json:"name"`
		Endpoint string `json:"endpoint"`
		Provider string `json:"provider"`
		Weight   int    `json:"weight"`
		Healthy  bool   `json:"healthy"`
		Conns    int64  `json:"active_conns"`
	}
	result := make([]item, 0, len(upstreams))
	for _, u := range upstreams {
		result = append(result, item{
			Name: u.Name, Endpoint: u.Endpoint, Provider: u.Provider,
			Weight: u.Weight, Healthy: u.Healthy.Load(), Conns: u.Conns(),
		})
	}
	writeJSON(w, http.StatusOK, result)
}

func (g *Gateway) handleListRoutes(w http.ResponseWriter, r *http.Request) {
	routes := g.router.Routes()
	type item struct {
		ID        string   `json:"id"`
		Model     string   `json:"model"`
		Upstream  string   `json:"upstream"`
		Fallbacks []string `json:"fallbacks,omitempty"`
		Priority  int      `json:"priority"`
	}
	result := make([]item, 0, len(routes))
	for _, rt := range routes {
		result = append(result, item{
			ID: rt.ID, Model: rt.Model, Upstream: rt.Upstream,
			Fallbacks: rt.Fallbacks, Priority: rt.Priority,
		})
	}
	writeJSON(w, http.StatusOK, result)
}

func (g *Gateway) handleListKeys(w http.ResponseWriter, r *http.Request) {
	type item struct {
		Key   string   `json:"key"`
		Name  string   `json:"name"`
		Roles []string `json:"roles,omitempty"`
	}
	result := make([]item, 0, len(g.cfg.Auth.Keys))
	for _, k := range g.cfg.Auth.Keys {
		result = append(result, item{Key: k.Key, Name: k.Name, Roles: k.Roles})
	}
	writeJSON(w, http.StatusOK, result)
}

func (g *Gateway) handleReloadRoutes(w http.ResponseWriter, r *http.Request) {
	var routes []config.RouteConfig
	if err := json.NewDecoder(r.Body).Decode(&routes); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid route config: "+err.Error())
		return
	}
	seenUpstreams := make(map[string]bool)
	for _, u := range g.cfg.Upstream {
		seenUpstreams[u.Name] = true
	}
	for _, route := range routes {
		if route.Model == "" {
			writeError(w, http.StatusBadRequest, "Route must have a model pattern")
			return
		}
		if route.Upstream == "" {
			writeError(w, http.StatusBadRequest, "Route must have an upstream")
			return
		}
		if !seenUpstreams[route.Upstream] {
			writeError(w, http.StatusBadRequest, "Route references unknown upstream: "+route.Upstream)
			return
		}
	}
	g.router.Update(routes)
	g.cfg.Routes = routes
	if _, err := config.SaveVersioned(g.cfg, g.configPath, "API update"); err != nil {
		g.logger.Warn("failed to persist route config", "error", err)
	}
	g.logger.Info("routes reloaded", "count", len(routes))
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "count": len(routes)})
}

func (g *Gateway) handleReloadUpstreams(w http.ResponseWriter, r *http.Request) {
	var upstreamReqs []upstreamReq
	if err := json.NewDecoder(r.Body).Decode(&upstreamReqs); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid upstream config: "+err.Error())
		return
	}
	upstreamCfgs := make([]config.UpstreamConfig, len(upstreamReqs))
	for i, ur := range upstreamReqs {
		upstreamCfgs[i] = ur.toConfig()
	}
	for _, u := range g.balancer.Upstreams() {
		g.balancer.RemoveUpstream(u.Name)
		delete(g.adapters, u.Name)
		delete(g.providerMap, u.Name)
	}
	for _, u := range upstreamCfgs {
		g.balancer.AddUpstream(u.Name, u.Endpoint, u.Provider, u.Weight)
		switch u.Provider {
		case "openai":
			g.adapters[u.Name] = openai.NewAdapter(u.Endpoint, u.APIToken, u.Model, u.Timeout)
		default:
			g.logger.Warn("unsupported provider for reload", "provider", u.Provider, "name", u.Name)
		}
		g.providerMap[u.Name] = u.Provider
	}
	g.cfg.Upstream = upstreamCfgs
	if _, err := config.SaveVersioned(g.cfg, g.configPath, "API update"); err != nil {
		g.logger.Warn("failed to persist upstream config", "error", err)
	}
	g.balancer.StartHealthChecks(r.Context(), 30*time.Second, 5*time.Second)
	g.logger.Info("upstreams reloaded", "count", len(upstreamCfgs))
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "count": len(upstreamCfgs)})
}

func (g *Gateway) handleAddKey(w http.ResponseWriter, r *http.Request) {
	var keyReq struct {
		Key   string   `json:"key"`
		Name  string   `json:"name"`
		Roles []string `json:"roles,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&keyReq); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if keyReq.Key == "" || keyReq.Name == "" {
		writeError(w, http.StatusBadRequest, "Key and name are required")
		return
	}
	for _, k := range g.cfg.Auth.Keys {
		if k.Key == keyReq.Key {
			writeError(w, http.StatusConflict, "API key already exists")
			return
		}
	}
	g.cfg.Auth.Keys = append(g.cfg.Auth.Keys, config.APIKey{
		Key: keyReq.Key, Name: keyReq.Name, Roles: keyReq.Roles,
	})
	if _, err := config.SaveVersioned(g.cfg, g.configPath, "API update"); err != nil {
		g.logger.Warn("failed to persist key config", "error", err)
	}
	g.logger.Info("API key added", "name", keyReq.Name)
	writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

func (g *Gateway) handleDeleteKey(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "Key is required")
		return
	}
	if !g.cfg.RemoveAPIKey(key) {
		writeError(w, http.StatusNotFound, "API key not found")
		return
	}
	if _, err := config.SaveVersioned(g.cfg, g.configPath, "API update"); err != nil {
		g.logger.Warn("failed to persist key config after delete", "error", err)
	}
	g.logger.Info("API key deleted")
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (g *Gateway) handleAddUpstream(w http.ResponseWriter, r *http.Request) {
	var ur upstreamReq
	if err := json.NewDecoder(r.Body).Decode(&ur); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid upstream config: "+err.Error())
		return
	}
	if ur.Name == "" || ur.Endpoint == "" || ur.Provider == "" {
		writeError(w, http.StatusBadRequest, "name, endpoint, and provider are required")
		return
	}
	u := ur.toConfig()

	existing := g.getUpstream(u.Name)
	if existing != nil {
		g.balancer.RemoveUpstream(u.Name)
		delete(g.adapters, u.Name)
		delete(g.providerMap, u.Name)
	}

	g.balancer.AddUpstream(u.Name, u.Endpoint, u.Provider, u.Weight)
	switch u.Provider {
	case "openai":
		g.adapters[u.Name] = openai.NewAdapter(u.Endpoint, u.APIToken, u.Model, u.Timeout)
	default:
		g.logger.Warn("unsupported provider", "provider", u.Provider, "name", u.Name)
	}
	g.providerMap[u.Name] = u.Provider
	g.cfg.UpsertUpstream(u)

	if _, err := config.SaveVersioned(g.cfg, g.configPath, "API update"); err != nil {
		g.logger.Warn("failed to persist upstream config", "error", err)
	}
	g.balancer.StartHealthChecks(r.Context(), 30*time.Second, 5*time.Second)
	g.logger.Info("upstream saved", "name", u.Name)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (g *Gateway) handleDeleteUpstream(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "Upstream name is required")
		return
	}
	for _, route := range g.cfg.Routes {
		if route.Upstream == name {
			writeError(w, http.StatusBadRequest, "Upstream is referenced by route: "+route.ID)
			return
		}
		for _, fb := range route.Fallbacks {
			if fb == name {
				writeError(w, http.StatusBadRequest, "Upstream is referenced as fallback by route: "+route.ID)
				return
			}
		}
	}
	if !g.balancer.RemoveUpstream(name) {
		writeError(w, http.StatusNotFound, "Upstream not found")
		return
	}
	delete(g.adapters, name)
	delete(g.providerMap, name)
	g.cfg.RemoveUpstream(name)
	if _, err := config.SaveVersioned(g.cfg, g.configPath, "API update"); err != nil {
		g.logger.Warn("failed to persist upstream config after delete", "error", err)
	}
	g.logger.Info("upstream deleted", "name", name)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (g *Gateway) handleAddRoute(w http.ResponseWriter, r *http.Request) {
	var rt config.RouteConfig
	if err := json.NewDecoder(r.Body).Decode(&rt); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid route config: "+err.Error())
		return
	}
	if rt.Model == "" || rt.Upstream == "" {
		writeError(w, http.StatusBadRequest, "model and upstream are required")
		return
	}
	seenUpstreams := make(map[string]bool)
	for _, u := range g.cfg.Upstream {
		seenUpstreams[u.Name] = true
	}
	if !seenUpstreams[rt.Upstream] {
		writeError(w, http.StatusBadRequest, "Unknown upstream: "+rt.Upstream)
		return
	}
	for _, fb := range rt.Fallbacks {
		if !seenUpstreams[fb] {
			writeError(w, http.StatusBadRequest, "Unknown fallback upstream: "+fb)
			return
		}
	}
	g.cfg.UpsertRoute(rt)
	g.router.Update(g.cfg.Routes)
	if _, err := config.SaveVersioned(g.cfg, g.configPath, "API update"); err != nil {
		g.logger.Warn("failed to persist route config", "error", err)
	}
	g.logger.Info("route saved", "id", rt.ID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (g *Gateway) handleDeleteRoute(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Route ID is required")
		return
	}
	if !g.cfg.RemoveRoute(id) {
		writeError(w, http.StatusNotFound, "Route not found")
		return
	}
	g.router.Update(g.cfg.Routes)
	if _, err := config.SaveVersioned(g.cfg, g.configPath, "API update"); err != nil {
		g.logger.Warn("failed to persist route config after delete", "error", err)
	}
	g.logger.Info("route deleted", "id", id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// JSON-friendly config views for the admin API

type securityView struct {
	PromptInjection promptInjectionView `json:"prompt_injection,omitempty"`
	PII             piiView             `json:"pii,omitempty"`
}

type promptInjectionView struct {
	Enabled    bool     `json:"enabled"`
	Action     string   `json:"action"`
	RiskThresh string   `json:"risk_threshold"`
	Keywords   []string `json:"keywords,omitempty"`
	ExternalURL string  `json:"external_url,omitempty"`
}

type piiView struct {
	Enabled bool     `json:"enabled"`
	Action  string   `json:"action"`
	Types   []string `json:"types,omitempty"`
}

type semanticView struct {
	Enabled   bool           `json:"enabled"`
	Provider  string         `json:"provider"`
	Threshold float64        `json:"threshold"`
	Categories []categoryView `json:"categories,omitempty"`
}

type categoryView struct {
	Name     string   `json:"name"`
	Examples []string `json:"examples"`
	Target   string   `json:"target"`
	Priority int      `json:"priority"`
}

type semanticCacheView struct {
	Enabled    bool    `json:"enabled"`
	Threshold  float64 `json:"threshold"`
	TTL        string  `json:"ttl"`
	MaxEntries int     `json:"max_entries"`
}

type costView struct {
	Enabled       bool             `json:"enabled"`
	DefaultLimit  quotaLimitView   `json:"default_limit,omitempty"`
	KeyLimits     []keyQuotaView   `json:"key_limits,omitempty"`
	DegradeConfig degradeView      `json:"degrade,omitempty"`
}

type quotaLimitView struct {
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	Window       string `json:"window"`
}

type keyQuotaView struct {
	KeyName string        `json:"key_name"`
	Limit   quotaLimitView `json:"limit"`
}

type degradeView struct {
	Action          string  `json:"action"`
	CheaperProvider string  `json:"cheaper_provider,omitempty"`
	AlertWebhook    string  `json:"alert_webhook,omitempty"`
	AlertThreshold  float64 `json:"alert_threshold"`
}

// configView is a JSON-friendly// configView is a JSON-friendly config representation
type configView struct {
	RateLimit rateLimitView `json:"rate_limit"`
	Log       logView       `json:"log"`
	Redis     redisView     `json:"redis"`
	Auth      authView      `json:"auth"`
	Security  securityView  `json:"security,omitempty"`
	Semantic  semanticView  `json:"semantic,omitempty"`
	SemanticCache semanticCacheView `json:"semantic_cache,omitempty"`
	Cost      costView      `json:"cost,omitempty"`
}

type rateLimitView struct {
	Enabled bool                `json:"enabled"`
	Global  *rateLimitRuleView  `json:"global,omitempty"`
	PerKey  []rateLimitRuleView `json:"per_key,omitempty"`
}

type rateLimitRuleView struct {
	Key    string `json:"key,omitempty"`
	Limit  int    `json:"limit"`
	Window string `json:"window"`
}

type logView struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

type redisView struct {
	Addr     string `json:"addr"`
	Password string `json:"password"`
	DB       int    `json:"db"`
}

type authView struct {
	Enabled bool `json:"enabled"`
}

// configUpdate is used for PUT /admin/config
type configUpdate struct {
	RateLimit *rateLimitUpdate `json:"rate_limit,omitempty"`
	Log       *logView         `json:"log,omitempty"`
	Auth      *struct {
		Enabled *bool `json:"enabled,omitempty"`
	} `json:"auth,omitempty"`
	Security      *securityView        `json:"security,omitempty"`
	Semantic      *semanticView        `json:"semantic,omitempty"`
	SemanticCache *semanticCacheView   `json:"semantic_cache,omitempty"`
	Cost          *costView            `json:"cost,omitempty"`
}

type rateLimitUpdate struct {
	Enabled bool                `json:"enabled"`
	Global  *rateLimitRuleView  `json:"global,omitempty"`
	PerKey  []rateLimitRuleView `json:"per_key,omitempty"`
}

func (g *Gateway) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cv := configView{
		RateLimit: rateLimitView{Enabled: g.cfg.RateLimit.Enabled},
		Log:       logView{Level: g.cfg.Log.Level, Format: g.cfg.Log.Format},
		Redis:     redisView{Addr: g.cfg.Redis.Addr, Password: g.cfg.Redis.Password, DB: g.cfg.Redis.DB},
		Auth:      authView{Enabled: g.cfg.Auth.Enabled},
		Security: securityView{
			PromptInjection: promptInjectionView{
				Enabled: g.cfg.Security.PromptInjection.Enabled,
				Action: g.cfg.Security.PromptInjection.Action,
				RiskThresh: g.cfg.Security.PromptInjection.RiskThresh,
				Keywords: g.cfg.Security.PromptInjection.Keywords,
				ExternalURL: g.cfg.Security.PromptInjection.ExternalURL,
			},
			PII: piiView{
				Enabled: g.cfg.Security.PII.Enabled,
				Action: g.cfg.Security.PII.Action,
				Types: g.cfg.Security.PII.Types,
			},
		},
		Semantic: semanticView{
			Enabled: g.cfg.Semantic.Enabled,
			Provider: g.cfg.Semantic.Provider,
			Threshold: g.cfg.Semantic.Threshold,
		},
		SemanticCache: semanticCacheView{
			Enabled: g.cfg.SemanticCache.Enabled,
			Threshold: g.cfg.SemanticCache.Threshold,
			TTL: g.cfg.SemanticCache.TTL,
			MaxEntries: g.cfg.SemanticCache.MaxEntries,
		},
		Cost: costView{
			Enabled: g.cfg.Cost.Enabled,
			DefaultLimit: quotaLimitView{
				InputTokens: g.cfg.Cost.DefaultLimit.InputTokens,
				OutputTokens: g.cfg.Cost.DefaultLimit.OutputTokens,
				Window: g.cfg.Cost.DefaultLimit.Window,
			},
			DegradeConfig: degradeView{
				Action: g.cfg.Cost.DegradeConfig.Action,
				CheaperProvider: g.cfg.Cost.DegradeConfig.CheaperProvider,
				AlertWebhook: g.cfg.Cost.DegradeConfig.AlertWebhook,
				AlertThreshold: g.cfg.Cost.DegradeConfig.AlertThreshold,
			},
		},
	}
	if g.cfg.RateLimit.Global != nil {
		cv.RateLimit.Global = &rateLimitRuleView{
			Key: g.cfg.RateLimit.Global.Key,
			Limit: g.cfg.RateLimit.Global.Limit,
			Window: g.cfg.RateLimit.Global.Window.String(),
		}
	}
	if len(g.cfg.RateLimit.PerKey) > 0 {
		for _, rl := range g.cfg.RateLimit.PerKey {
			cv.RateLimit.PerKey = append(cv.RateLimit.PerKey, rateLimitRuleView{
				Key: rl.Key, Limit: rl.Limit, Window: rl.Window.String(),
			})
		}
	}
	if len(g.cfg.Semantic.Categories) > 0 {
		for _, cat := range g.cfg.Semantic.Categories {
			cv.Semantic.Categories = append(cv.Semantic.Categories, categoryView{
				Name: cat.Name, Examples: cat.Examples,
				Target: cat.Target, Priority: cat.Priority,
			})
		}
	}
	if len(g.cfg.Cost.KeyLimits) > 0 {
		for _, kl := range g.cfg.Cost.KeyLimits {
			cv.Cost.KeyLimits = append(cv.Cost.KeyLimits, keyQuotaView{
				KeyName: kl.KeyName,
				Limit: quotaLimitView{
					InputTokens: kl.Limit.InputTokens,
					OutputTokens: kl.Limit.OutputTokens,
					Window: kl.Limit.Window,
				},
			})
		}
	}
	writeJSON(w, http.StatusOK, cv)
}

func (g *Gateway) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var body configUpdate
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if body.RateLimit != nil {
		g.cfg.RateLimit.Enabled = body.RateLimit.Enabled
		g.cfg.RateLimit.PerKey = nil
		if body.RateLimit.Global != nil {
			d, err := time.ParseDuration(body.RateLimit.Global.Window)
			if err != nil {
				writeError(w, http.StatusBadRequest, "Invalid rate limit window: "+err.Error())
				return
			}
			g.cfg.RateLimit.Global = &config.RateLimitRule{
				Key: body.RateLimit.Global.Key,
				Limit: body.RateLimit.Global.Limit,
				Window: d,
			}
		} else {
			g.cfg.RateLimit.Global = nil
		}
		if len(body.RateLimit.PerKey) > 0 {
			for _, rl := range body.RateLimit.PerKey {
				d, err := time.ParseDuration(rl.Window)
				if err != nil {
					writeError(w, http.StatusBadRequest, "Invalid per-key window: "+err.Error())
					return
				}
				g.cfg.RateLimit.PerKey = append(g.cfg.RateLimit.PerKey, config.RateLimitRule{
					Key: rl.Key, Limit: rl.Limit, Window: d,
				})
			}
		}
	}
	if body.Log != nil {
		g.cfg.Log.Level = body.Log.Level
		g.cfg.Log.Format = body.Log.Format
	}
	if body.Auth != nil && body.Auth.Enabled != nil {
		g.cfg.Auth.Enabled = *body.Auth.Enabled
	}
	if body.Security != nil {
		// Validate prompt injection action
		switch body.Security.PromptInjection.Action {
		case "block", "log", "sanitize":
		case "":
			body.Security.PromptInjection.Action = "block"
		default:
			writeError(w, http.StatusBadRequest, "security.prompt_injection.action must be one of: block, log, sanitize")
			return
		}
		// Validate risk threshold
		switch body.Security.PromptInjection.RiskThresh {
		case "low", "medium", "high", "critical":
		case "":
			body.Security.PromptInjection.RiskThresh = "medium"
		default:
			writeError(w, http.StatusBadRequest, "security.prompt_injection.risk_threshold must be one of: low, medium, high, critical")
			return
		}
		// Validate PII action
		switch body.Security.PII.Action {
		case "mask", "block", "log":
		case "":
			body.Security.PII.Action = "mask"
		default:
			writeError(w, http.StatusBadRequest, "security.pii.action must be one of: mask, block, log")
			return
		}
		// Validate PII types
		if len(body.Security.PII.Types) > 0 {
			validTypes := map[string]bool{"email": true, "phone": true, "ip": true, "key": true, "ssn": true, "credit_card": true}
			for _, t := range body.Security.PII.Types {
				if !validTypes[t] {
					writeError(w, http.StatusBadRequest, "security.pii.types contains invalid type: "+t)
					return
				}
			}
		}
		g.cfg.Security.PromptInjection.Enabled = body.Security.PromptInjection.Enabled
		g.cfg.Security.PromptInjection.Action = body.Security.PromptInjection.Action
		g.cfg.Security.PromptInjection.RiskThresh = body.Security.PromptInjection.RiskThresh
		if body.Security.PromptInjection.Keywords != nil {
			g.cfg.Security.PromptInjection.Keywords = body.Security.PromptInjection.Keywords
		}
		g.cfg.Security.PII.Enabled = body.Security.PII.Enabled
		g.cfg.Security.PII.Action = body.Security.PII.Action
		if body.Security.PII.Types != nil {
			g.cfg.Security.PII.Types = body.Security.PII.Types
		}
	}
	if body.Semantic != nil {
		if body.Semantic.Threshold < 0 || body.Semantic.Threshold > 1 {
			writeError(w, http.StatusBadRequest, "semantic.threshold must be between 0 and 1")
			return
		}
		if body.Semantic.Enabled && body.Semantic.Provider == "" {
			writeError(w, http.StatusBadRequest, "semantic.provider is required when semantic is enabled")
			return
		}
		if len(body.Semantic.Categories) > 0 {
			for _, c := range body.Semantic.Categories {
				if c.Name == "" {
					writeError(w, http.StatusBadRequest, "semantic.categories requires non-empty name")
					return
				}
				if len(c.Examples) == 0 {
					writeError(w, http.StatusBadRequest, "semantic.categories requires at least one example for: "+c.Name)
					return
				}
			}
		}
		g.cfg.Semantic.Enabled = body.Semantic.Enabled
		g.cfg.Semantic.Provider = body.Semantic.Provider
		g.cfg.Semantic.Threshold = body.Semantic.Threshold
	}
	if body.SemanticCache != nil {
		if body.SemanticCache.Threshold < 0 || body.SemanticCache.Threshold > 1 {
			writeError(w, http.StatusBadRequest, "semantic_cache.threshold must be between 0 and 1")
			return
		}
		if body.SemanticCache.TTL != "" {
			if _, err := time.ParseDuration(body.SemanticCache.TTL); err != nil {
				writeError(w, http.StatusBadRequest, "semantic_cache.ttl is invalid: "+err.Error())
				return
			}
		}
		if body.SemanticCache.MaxEntries < 0 {
			writeError(w, http.StatusBadRequest, "semantic_cache.max_entries must be non-negative")
			return
		}
		g.cfg.SemanticCache.Enabled = body.SemanticCache.Enabled
		g.cfg.SemanticCache.Threshold = body.SemanticCache.Threshold
		g.cfg.SemanticCache.TTL = body.SemanticCache.TTL
		g.cfg.SemanticCache.MaxEntries = body.SemanticCache.MaxEntries
	}
	if body.Cost != nil {
		if body.Cost.DefaultLimit.InputTokens < 0 {
			writeError(w, http.StatusBadRequest, "cost.default_limit.input_tokens must be non-negative")
			return
		}
		if body.Cost.DefaultLimit.OutputTokens < 0 {
			writeError(w, http.StatusBadRequest, "cost.default_limit.output_tokens must be non-negative")
			return
		}
		if body.Cost.DefaultLimit.Window != "" {
			if _, err := time.ParseDuration(body.Cost.DefaultLimit.Window); err != nil {
				writeError(w, http.StatusBadRequest, "cost.default_limit.window is invalid: "+err.Error())
				return
			}
		}
		// Validate degrade action
		switch body.Cost.DegradeConfig.Action {
		case "warn", "block", "degrade":
		case "":
			body.Cost.DegradeConfig.Action = "warn"
		default:
			writeError(w, http.StatusBadRequest, "cost.degrade.action must be one of: warn, block, degrade")
			return
		}
		if body.Cost.DegradeConfig.AlertThreshold < 0 || body.Cost.DegradeConfig.AlertThreshold > 1 {
			writeError(w, http.StatusBadRequest, "cost.degrade.alert_threshold must be between 0 and 1")
			return
		}
		g.cfg.Cost.Enabled = body.Cost.Enabled
		g.cfg.Cost.DefaultLimit.InputTokens = body.Cost.DefaultLimit.InputTokens
		g.cfg.Cost.DefaultLimit.OutputTokens = body.Cost.DefaultLimit.OutputTokens
		g.cfg.Cost.DefaultLimit.Window = body.Cost.DefaultLimit.Window
		g.cfg.Cost.DegradeConfig.Action = body.Cost.DegradeConfig.Action
		g.cfg.Cost.DegradeConfig.CheaperProvider = body.Cost.DegradeConfig.CheaperProvider
		g.cfg.Cost.DegradeConfig.AlertThreshold = body.Cost.DegradeConfig.AlertThreshold
	}

	if _, err := config.SaveVersioned(g.cfg, g.configPath, "API update"); err != nil {
		g.logger.Warn("failed to persist config", "error", err)
	}
	g.logger.Info("gateway config updated")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (g *Gateway) handleGetAuditLogs(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	level := r.URL.Query().Get("level")
	keyName := r.URL.Query().Get("key_name")
	path := r.URL.Query().Get("path")
	statusStr := r.URL.Query().Get("status")

	limit := 100
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}
	status := 0
	if s, err := strconv.Atoi(statusStr); err == nil {
		status = s
	}

	entries := g.auditStore.Query(limit, level, keyName, path, status)
	writeJSON(w, http.StatusOK, entries)
}


func (g *Gateway) handleGetCostStats(w http.ResponseWriter, r *http.Request) {
	if g.costTracker == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}
	// Parse optional time range parameter (default: 24h)
	duration := 24 * time.Hour
	if d := r.URL.Query().Get("since"); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil && parsed > 0 {
			duration = parsed
		}
	}
	since := time.Now().Add(-duration)
	stats := g.costTracker.GetStats(since)
	writeJSON(w, http.StatusOK, stats)
}



func (g *Gateway) handleHealthHistory(w http.ResponseWriter, r *http.Request) {
	upstreams := g.balancer.Upstreams()
	type item struct {
		Name     string `json:"name"`
		Endpoint string `json:"endpoint"`
		Provider string `json:"provider"`
		Healthy  bool   `json:"healthy"`
		Conns    int64  `json:"active_conns"`
	}
	result := make([]item, 0, len(upstreams))
	for _, u := range upstreams {
		result = append(result, item{
			Name: u.Name, Endpoint: u.Endpoint, Provider: u.Provider,
			Healthy: u.Healthy.Load(), Conns: u.Conns(),
		})
	}
	healthyCount := 0
	for _, u := range result {
		if u.Healthy {
			healthyCount++
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"upstreams":     result,
		"total":         len(result),
		"healthy_count": healthyCount,
	})
}

func (g *Gateway) handleListPrompts(w http.ResponseWriter, r *http.Request) {
	list := g.promptManager.List()
	writeJSON(w, http.StatusOK, list)
}

func (g *Gateway) handleGetPrompt(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	t, ok := g.promptManager.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "Prompt not found")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (g *Gateway) handleSavePrompt(w http.ResponseWriter, r *http.Request) {
	var t prompt.Template
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if t.Name == "" {
		writeError(w, http.StatusBadRequest, "Template name is required")
		return
	}
	if err := g.promptManager.Save(&t); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "id": t.ID})
}

func (g *Gateway) handleDeletePrompt(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !g.promptManager.Delete(id) {
		writeError(w, http.StatusNotFound, "Prompt not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (g *Gateway) handleAddPromptVersion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	t, ok := g.promptManager.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "Prompt not found")
		return
	}
	var body struct {
		Content   string                  `json:"content"` 
		Variables []prompt.TemplateVariable `json:"variables,omitempty"` 
		CreatedBy string                  `json:"created_by,omitempty"` 
		Comment   string                  `json:"comment,omitempty"` 
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if body.Content == "" {
		writeError(w, http.StatusBadRequest, "Content is required")
		return
	}
	t.AddVersion(body.Content, body.Variables, body.CreatedBy, body.Comment)
	g.promptManager.Save(t)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": fmt.Sprintf("%d", t.CurrentVer)})
}



func (g *Gateway) handleGetWebhookConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, g.cfg.Webhook)
}

func (g *Gateway) handleUpdateWebhookConfig(w http.ResponseWriter, r *http.Request) {
	var cfg webhook.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	g.cfg.Webhook = cfg
	g.webhookNotifier.UpdateConfig(cfg)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}



func (g *Gateway) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "docs/openapi.yaml")
}



func (g *Gateway) handleListPlugins(w http.ResponseWriter, r *http.Request) {
	result := map[string]interface{}{
		"config": g.cfg.Plugin,
		"providers": len(g.pluginManager.ProviderPlugins()),
		"security":  len(g.pluginManager.SecurityPlugins()),
		"routers":   len(g.pluginManager.RouterPlugins()),
	}
	writeJSON(w, http.StatusOK, result)
}

func (g *Gateway) handleReloadPlugins(w http.ResponseWriter, r *http.Request) {
	var cfg plugin.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	g.cfg.Plugin = cfg
	g.pluginManager.Load(cfg)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}


func (g *Gateway) handleConfigVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := config.ListVersions(g.configPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list config versions: "+err.Error())
		return
	}
	if versions == nil {
		versions = []config.ConfigVersion{}
	}
	writeJSON(w, http.StatusOK, versions)
}

func (g *Gateway) handleConfigRollback(w http.ResponseWriter, r *http.Request) {
	versionStr := chi.URLParam(r, "version")
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid version number")
		return
	}

	cfg, err := config.Rollback(g.configPath, version)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Rollback failed: "+err.Error())
		return
	}

	// Reload gateway config
	g.cfg = cfg
	g.logger.Info("config rolled back", "version", version)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"version": version,
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}






// ---- User Authentication ----

func (g *Gateway) handleLogin(w http.ResponseWriter, r *http.Request) {
	if g.userStore == nil {
		writeError(w, http.StatusServiceUnavailable, "User authentication is not configured")
		return
	}
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	u, err := g.userStore.Authenticate(creds.Username, creds.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}
	token, err := user.GenerateToken(u)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":    token,
		"username": u.Username,
		"role":     u.Role,
		"team":     u.Team,
	})
}

// ---- User Management ----

func (g *Gateway) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if g.userStore == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}
	type userResp struct {
		Username  string `json:"username"`
		Role      string `json:"role"`
		Team      string `json:"team"`
		CreatedAt string `json:"created_at"`
	}
	users := g.userStore.List()
	result := make([]userResp, 0, len(users))
	for _, u := range users {
		result = append(result, userResp{
			Username: u.Username, Role: u.Role, Team: u.Team,
			CreatedAt: u.CreatedAt.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, result)
}

func (g *Gateway) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if g.userStore == nil {
		writeError(w, http.StatusServiceUnavailable, "User store not available")
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Team     string `json:"team,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if body.Username == "" {
		writeError(w, http.StatusBadRequest, "Username is required")
		return
	}
	if body.Password == "" {
		writeError(w, http.StatusBadRequest, "Password is required")
		return
	}
	if body.Role == "" {
		body.Role = "viewer"
	}
	if body.Role != "admin" && body.Role != "editor" && body.Role != "viewer" {
		writeError(w, http.StatusBadRequest, "Invalid role: must be admin, editor, or viewer")
		return
	}
	u, err := g.userStore.Create(body.Username, body.Password, body.Role, body.Team)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"status": "ok", "username": u.Username,
	})
}

func (g *Gateway) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if g.userStore == nil {
		writeError(w, http.StatusServiceUnavailable, "User store not available")
		return
	}
	username := chi.URLParam(r, "username")
	requester, _ := r.Context().Value(middleware.APIKeyNameContextKey).(string)
	if err := g.userStore.Delete(username, requester); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (g *Gateway) handleUpdateUserRole(w http.ResponseWriter, r *http.Request) {
	if g.userStore == nil {
		writeError(w, http.StatusServiceUnavailable, "User store not available")
		return
	}
	username := chi.URLParam(r, "username")
	var body struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if body.Role != "admin" && body.Role != "editor" && body.Role != "viewer" {
		writeError(w, http.StatusBadRequest, "Invalid role")
		return
	}
	if err := g.userStore.UpdateRole(username, body.Role); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---- API Key Lifecycle ----

func (g *Gateway) handleRotateKey(w http.ResponseWriter, r *http.Request) {
	keyName := chi.URLParam(r, "key")
	// Find the key
	found := false
	for i, k := range g.cfg.Auth.Keys {
		if k.Key == keyName {
			newKey, err := user.GenerateAPIKey()
			if err != nil {
				writeError(w, http.StatusInternalServerError, "Failed to generate new key")
				return
			}
			g.cfg.Auth.Keys[i].Key = newKey
			g.cfg.Auth.Keys[i].LastRotated = time.Now().Format(time.RFC3339)
			found = true
			if _, err := config.SaveVersioned(g.cfg, g.configPath, "Key rotated: "+k.Name); err != nil {
				g.logger.Warn("failed to persist key rotation", "error", err)
			}
			writeJSON(w, http.StatusOK, map[string]string{
				"status":  "ok",
				"old_key": keyName,
				"new_key": newKey,
			})
			return
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "Key not found")
	}
}

// ---- Security Policy Center ----

func (g *Gateway) handleGetSecurityPolicies(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"prompt_injection": g.cfg.Security.PromptInjection,
		"pii":              g.cfg.Security.PII,
		"ip_allowlist":     g.cfg.Security.IPAllowlist,
		"ip_blocklist":     g.cfg.Security.IPBlocklist,
	})
}

func (g *Gateway) handleUpdateSecurityPolicies(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PromptInjection *security.PromptInjectionConfig `json:"prompt_injection,omitempty"`
		PII             *security.PIIConfig             `json:"pii,omitempty"`
		IPAllowlist     *[]string                       `json:"ip_allowlist,omitempty"`
		IPBlocklist     *[]string                       `json:"ip_blocklist,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if body.PromptInjection != nil {
		g.cfg.Security.PromptInjection = *body.PromptInjection
	}
	if body.PII != nil {
		g.cfg.Security.PII = *body.PII
	}
	if body.IPAllowlist != nil {
		g.cfg.Security.IPAllowlist = *body.IPAllowlist
	}
	if body.IPBlocklist != nil {
		g.cfg.Security.IPBlocklist = *body.IPBlocklist
	}
	if _, err := config.SaveVersioned(g.cfg, g.configPath, "Security policies updated"); err != nil {
		g.logger.Warn("failed to persist security policies", "error", err)
	}
	g.logger.Info("security policies updated")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

