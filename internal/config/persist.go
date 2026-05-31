package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Save writes the Config to the specified YAML file path.
func Save(cfg *Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
}

// RemoveAPIKey removes an API key from the config by key value.
func (cfg *Config) RemoveAPIKey(key string) bool {
	for i, k := range cfg.Auth.Keys {
		if k.Key == key {
			cfg.Auth.Keys = append(cfg.Auth.Keys[:i], cfg.Auth.Keys[i+1:]...)
			return true
		}
	}
	return false
}

// RemoveUpstream removes an upstream from the config by name.
func (cfg *Config) RemoveUpstream(name string) bool {
	for i, u := range cfg.Upstream {
		if u.Name == name {
			cfg.Upstream = append(cfg.Upstream[:i], cfg.Upstream[i+1:]...)
			return true
		}
	}
	return false
}

// RemoveRoute removes a route from the config by ID.
func (cfg *Config) RemoveRoute(id string) bool {
	for i, r := range cfg.Routes {
		if r.ID == id {
			cfg.Routes = append(cfg.Routes[:i], cfg.Routes[i+1:]...)
			return true
		}
	}
	return false
}

// UpsertUpstream adds or updates an upstream by name.
func (cfg *Config) UpsertUpstream(u UpstreamConfig) {
	for i, existing := range cfg.Upstream {
		if existing.Name == u.Name {
			cfg.Upstream[i] = u
			return
		}
	}
	cfg.Upstream = append(cfg.Upstream, u)
}

// UpsertRoute adds or updates a route by ID.
func (cfg *Config) UpsertRoute(r RouteConfig) {
	for i, existing := range cfg.Routes {
		if existing.ID == r.ID {
			cfg.Routes[i] = r
			return
		}
	}
	cfg.Routes = append(cfg.Routes, r)
}
