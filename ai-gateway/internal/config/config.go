package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/yushi/ai-gateway/internal/security"
	"github.com/yushi/ai-gateway/internal/cost"
	"github.com/yushi/ai-gateway/internal/semantic"
)

type Config struct {
	Server    ServerConfig       `yaml:"server"`
	Auth      AuthConfig         `yaml:"auth"`
	Redis     RedisConfig        `yaml:"redis"`
	Routes    []RouteConfig      `yaml:"routes"`
	Upstream  []UpstreamConfig   `yaml:"upstreams"`
	RateLimit RateLimitConfig    `yaml:"rate_limit"`
	Semantic  semantic.SemanticRouterConfig `yaml:"semantic"`
	SemanticCache semantic.SemanticCacheConfig  `yaml:"semantic_cache"`
	Cost        cost.Config              `yaml:"cost"`
	Security  security.Config    `yaml:"security"`
	Log       LogConfig          `yaml:"log"`
}

type ServerConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

type AuthConfig struct {
	Enabled bool     `yaml:"enabled"`
	Keys    []APIKey `yaml:"keys"`
}

type APIKey struct {
	Key   string   `yaml:"key"`
	Name  string   `yaml:"name"`
	Roles []string `yaml:"roles"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type RouteConfig struct {
	ID        string   `yaml:"id"`
	Model     string   `yaml:"model"`
	Upstream  string   `yaml:"upstream"`
	Fallbacks []string `yaml:"fallbacks,omitempty"`
	Priority  int      `yaml:"priority"`
}

type UpstreamConfig struct {
	Name        string             `yaml:"name"`
	Endpoint    string             `yaml:"endpoint"`
	Provider    string             `yaml:"provider"`
	APIVersion  string             `yaml:"api_version,omitempty"`
	APIToken    string             `yaml:"api_token"`
	Model       string             `yaml:"model,omitempty"`
	Weight      int                `yaml:"weight"`
	Timeout     time.Duration      `yaml:"timeout"`
	RetryCount  int                `yaml:"retry_count"`
	HealthCheck *HealthCheckConfig `yaml:"health_check,omitempty"`
}

type HealthCheckConfig struct {
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
	Path     string        `yaml:"path,omitempty"`
}

type RateLimitConfig struct {
	Enabled bool            `yaml:"enabled"`
	Global  *RateLimitRule  `yaml:"global,omitempty"`
	PerKey  []RateLimitRule `yaml:"per_key,omitempty"`
}

type RateLimitRule struct {
	Key    string        `yaml:"key"`
	Limit  int           `yaml:"limit"`
	Window time.Duration `yaml:"window"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0", Port: 8080,
			ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second,
			ShutdownTimeout: 10 * time.Second,
		},
		Auth:      AuthConfig{Enabled: true},
		RateLimit: RateLimitConfig{Enabled: false},
		Security:  security.DefaultConfig(),
		Semantic:     semantic.SemanticRouterConfig{},
		SemanticCache: semantic.SemanticCacheConfig{},
		Cost:          cost.Config{},
		Log:       LogConfig{Level: "info", Format: "text"},
	}
}

func Load(path string) (*Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	applyEnvOverrides(cfg)
	if err := validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("GATEWAY_SERVER_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("GATEWAY_SERVER_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	}
	if v := os.Getenv("GATEWAY_LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
	if v := os.Getenv("GATEWAY_LOG_FORMAT"); v != "" {
		cfg.Log.Format = v
	}
	if v := os.Getenv("GATEWAY_AUTH_ENABLED"); v != "" {
		cfg.Auth.Enabled = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("GATEWAY_REDIS_ADDR"); v != "" {
		cfg.Redis.Addr = v
	}
	if v := os.Getenv("GATEWAY_REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}
}

func validate(cfg *Config) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}
	seenUpstreams := make(map[string]bool)
	for i := range cfg.Upstream {
		u := &cfg.Upstream[i]
		if u.Name == "" {
			return fmt.Errorf("upstream must have a name")
		}
		if seenUpstreams[u.Name] {
			return fmt.Errorf("duplicate upstream name: %s", u.Name)
		}
		seenUpstreams[u.Name] = true
		if u.Endpoint == "" {
			return fmt.Errorf("upstream %q must have an endpoint", u.Name)
		}
		if u.Provider == "" {
			return fmt.Errorf("upstream %q must have a provider", u.Name)
		}
		if u.Weight <= 0 {
			u.Weight = 1
		}
		if u.Timeout <= 0 {
			u.Timeout = 30 * time.Second
		}
	}
	for _, r := range cfg.Routes {
		if r.Model == "" {
			return fmt.Errorf("route must have a model pattern")
		}
		if r.Upstream == "" {
			return fmt.Errorf("route %q must have an upstream", r.ID)
		}
		if !seenUpstreams[r.Upstream] {
			return fmt.Errorf("route %q references unknown upstream %q", r.ID, r.Upstream)
		}
		for _, f := range r.Fallbacks {
			if !seenUpstreams[f] {
				return fmt.Errorf("route %q references unknown fallback upstream %q", r.ID, f)
			}
			if f == r.Upstream {
				return fmt.Errorf("route %q fallback %q is the same as primary", r.ID, f)
			}
		}
	}
	return nil
}
