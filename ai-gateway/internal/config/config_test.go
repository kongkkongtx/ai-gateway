package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if c.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", c.Server.Port)
	}
	if c.Log.Level != "info" {
		t.Errorf("expected log level info, got %s", c.Log.Level)
	}
}

func TestLoadValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte(`
server:
  host: "127.0.0.1"
  port: 9090
auth:
  enabled: false
upstreams:
  - name: "test-upstream"
    endpoint: "https://api.openai.com"
    provider: "openai"
    api_token: "sk-test"
    weight: 1
routes:
  - id: "test-route"
    model: "gpt-4*"
    upstream: "test-upstream"
    priority: 100
log:
  level: "debug"
`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("expected log level debug, got %s", cfg.Log.Level)
	}
	if len(cfg.Upstream) != 1 {
		t.Errorf("expected 1 upstream, got %d", len(cfg.Upstream))
	}
	if cfg.Upstream[0].Weight != 1 {
		t.Errorf("expected weight 1, got %d", cfg.Upstream[0].Weight)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestEnvOverrides(t *testing.T) {
	os.Setenv("GATEWAY_SERVER_PORT", "3000")
	os.Setenv("GATEWAY_LOG_LEVEL", "error")
	defer func() {
		os.Unsetenv("GATEWAY_SERVER_PORT")
		os.Unsetenv("GATEWAY_LOG_LEVEL")
	}()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte(`
server:
  host: "0.0.0.0"
  port: 8080
auth:
  enabled: false
upstreams:
  - name: "test"
    endpoint: "https://test.com"
    provider: "openai"
    api_token: "sk-test"
    weight: 1
routes:
  - id: "r1"
    model: "*"
    upstream: "test"
    priority: 0
`)
	os.WriteFile(path, content, 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Server.Port != 3000 {
		t.Errorf("expected port 3000 from env, got %d", cfg.Server.Port)
	}
	if cfg.Log.Level != "error" {
		t.Errorf("expected log level error from env, got %s", cfg.Log.Level)
	}
}