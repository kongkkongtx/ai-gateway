package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/yushi/ai-gateway/internal/config"
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
	if err := config.Save(g.cfg, g.configPath); err != nil {
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
	if err := config.Save(g.cfg, g.configPath); err != nil {
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
	if err := config.Save(g.cfg, g.configPath); err != nil {
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
	if err := config.Save(g.cfg, g.configPath); err != nil {
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

	if err := config.Save(g.cfg, g.configPath); err != nil {
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
	if err := config.Save(g.cfg, g.configPath); err != nil {
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
	if err := config.Save(g.cfg, g.configPath); err != nil {
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
	if err := config.Save(g.cfg, g.configPath); err != nil {
		g.logger.Warn("failed to persist route config after delete", "error", err)
	}
	g.logger.Info("route deleted", "id", id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// configView is a JSON-friendly config representation
type configView struct {
	RateLimit rateLimitView `json:"rate_limit"`
	Log       logView       `json:"log"`
	Redis     redisView     `json:"redis"`
	Auth      authView      `json:"auth"`
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

	if err := config.Save(g.cfg, g.configPath); err != nil {
		g.logger.Warn("failed to persist config", "error", err)
	}
	g.logger.Info("gateway config updated")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
