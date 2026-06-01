package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ============================================================
// Plugin Interfaces
// ============================================================

// ProviderPlugin handles chat completion and embedding requests.
type ProviderPlugin interface {
	Name() string
	ChatCompletion(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error)
	Embeddings(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error)
}

// SecurityPlugin checks and optionally modifies prompts.
type SecurityPlugin interface {
	Name() string
	// CheckPrompt returns nil (allow), error (block), or modified messages.
	CheckPrompt(ctx context.Context, messages []map[string]interface{}) ([]map[string]interface{}, error)
}

// RouterPlugin makes routing decisions based on request context.
type RouterPlugin interface {
	Name() string
	// SelectUpstream returns the upstream name to use, or empty for default routing.
	SelectUpstream(ctx context.Context, model string, messages []map[string]interface{}) (string, error)
}

// ============================================================
// Plugin Configuration
// ============================================================

// PluginType enumerates supported plugin types.
type PluginType string

const (
	PluginTypeProvider PluginType = "provider"
	PluginTypeSecurity PluginType = "security"
	PluginTypeRouter   PluginType = "router"
)

// PluginConfig defines a single plugin.
type PluginConfig struct {
	Name     string     `yaml:"name" json:"name"`
	Type     PluginType `yaml:"type" json:"type"`
	Endpoint string     `yaml:"endpoint" json:"endpoint"` // HTTP URL for external plugin
	Enabled  bool       `yaml:"enabled" json:"enabled"`
	Timeout  string     `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// Config holds all plugin definitions.
type Config struct {
	Plugins []PluginConfig `yaml:"plugins,omitempty" json:"plugins,omitempty"`
}

// ============================================================
// HTTP Plugin Client
// ============================================================

// httpPlugin calls an external HTTP service for plugin operations.
type httpPlugin struct {
	cfg     PluginConfig
	client  *http.Client
	logger  *slog.Logger
}

func newHTTPPlugin(cfg PluginConfig, logger *slog.Logger) *httpPlugin {
	timeout := 10 * time.Second
	if cfg.Timeout != "" {
		if d, err := time.ParseDuration(cfg.Timeout); err == nil {
			timeout = d
		}
	}
	return &httpPlugin{
		cfg:    cfg,
		client: &http.Client{Timeout: timeout},
		logger: logger,
	}
}

func (p *httpPlugin) call(ctx context.Context, action string, payload interface{}) (map[string]interface{}, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("plugin %s: marshal: %w", p.cfg.Name, err)
	}

	url := strings.TrimRight(p.cfg.Endpoint, "/") + "/" + action
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("plugin %s: request: %w", p.cfg.Name, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Plugin-Name", p.cfg.Name)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("plugin %s: call: %w", p.cfg.Name, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("plugin %s: read: %w", p.cfg.Name, err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("plugin %s: HTTP %d: %s", p.cfg.Name, resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("plugin %s: unmarshal: %w", p.cfg.Name, err)
	}
	return result, nil
}

func (p *httpPlugin) Name() string { return p.cfg.Name }

func (p *httpPlugin) ChatCompletion(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	return p.call(ctx, "chat-completion", req)
}

func (p *httpPlugin) Embeddings(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	return p.call(ctx, "embeddings", req)
}

func (p *httpPlugin) CheckPrompt(ctx context.Context, messages []map[string]interface{}) ([]map[string]interface{}, error) {
	payload := map[string]interface{}{"messages": messages}
	result, err := p.call(ctx, "check-prompt", payload)
	if err != nil {
		return nil, err
	}
	if modified, ok := result["messages"]; ok {
		if msgs, ok := modified.([]interface{}); ok {
			var out []map[string]interface{}
			for _, m := range msgs {
				if msg, ok := m.(map[string]interface{}); ok {
					out = append(out, msg)
				}
			}
			return out, nil
		}
	}
	// Blocked: check for "blocked" field
	if blocked, _ := result["blocked"].(bool); blocked {
		return nil, fmt.Errorf("plugin blocked: %s", result["reason"])
	}
	return messages, nil
}

func (p *httpPlugin) SelectUpstream(ctx context.Context, model string, messages []map[string]interface{}) (string, error) {
	payload := map[string]interface{}{"model": model, "messages": messages}
	result, err := p.call(ctx, "select-upstream", payload)
	if err != nil {
		return "", err
	}
	upstream, _ := result["upstream"].(string)
	return upstream, nil
}

// ============================================================
// Plugin Manager
// ============================================================

// Manager loads and manages plugins.
type Manager struct {
	mu              sync.RWMutex
	providerPlugins map[string]ProviderPlugin
	securityPlugins map[string]SecurityPlugin
	routerPlugins   map[string]RouterPlugin
	logger          *slog.Logger
}

// NewManager creates a plugin manager from configuration.
func NewManager(cfg Config, logger *slog.Logger) *Manager {
	m := &Manager{
		providerPlugins: make(map[string]ProviderPlugin),
		securityPlugins: make(map[string]SecurityPlugin),
		routerPlugins:   make(map[string]RouterPlugin),
		logger:          logger,
	}
	m.Load(cfg)
	return m
}

// Load initializes or reloads plugins from configuration.
func (m *Manager) Load(cfg Config) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear existing
	m.providerPlugins = make(map[string]ProviderPlugin)
	m.securityPlugins = make(map[string]SecurityPlugin)
	m.routerPlugins = make(map[string]RouterPlugin)

	for _, pc := range cfg.Plugins {
		if !pc.Enabled {
			continue
		}
		if pc.Endpoint == "" {
			m.logger.Warn("plugin has no endpoint, skipping", "name", pc.Name)
			continue
		}

		plug := newHTTPPlugin(pc, m.logger)

		switch pc.Type {
		case PluginTypeProvider:
			m.providerPlugins[pc.Name] = plug
			m.logger.Info("provider plugin loaded", "name", pc.Name, "endpoint", pc.Endpoint)
		case PluginTypeSecurity:
			m.securityPlugins[pc.Name] = plug
			m.logger.Info("security plugin loaded", "name", pc.Name, "endpoint", pc.Endpoint)
		case PluginTypeRouter:
			m.routerPlugins[pc.Name] = plug
			m.logger.Info("router plugin loaded", "name", pc.Name, "endpoint", pc.Endpoint)
		default:
			m.logger.Warn("unknown plugin type", "name", pc.Name, "type", pc.Type)
		}
	}
}

// ProviderPlugins returns all loaded provider plugins.
func (m *Manager) ProviderPlugins() map[string]ProviderPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]ProviderPlugin, len(m.providerPlugins))
	for k, v := range m.providerPlugins {
		result[k] = v
	}
	return result
}

// SecurityPlugins returns all loaded security plugins.
func (m *Manager) SecurityPlugins() map[string]SecurityPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]SecurityPlugin, len(m.securityPlugins))
	for k, v := range m.securityPlugins {
		result[k] = v
	}
	return result
}

// RouterPlugins returns all loaded router plugins.
func (m *Manager) RouterPlugins() map[string]RouterPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]RouterPlugin, len(m.routerPlugins))
	for k, v := range m.routerPlugins {
		result[k] = v
	}
	return result
}
