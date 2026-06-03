package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kongkkongtx/ai-gateway/internal/config"
	"github.com/kongkkongtx/ai-gateway/internal/server"
)

func testConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Host: "127.0.0.1", Port: 0,
		},
		Auth: config.AuthConfig{Enabled: false},
		Log:  config.LogConfig{Level: "error", Format: "text"},
	}
}

func newTestGateway(t *testing.T) *server.Gateway {
	t.Helper()
	cfg := testConfig()
	gw, err := server.NewForTest(cfg)
	if err != nil {
		t.Fatalf("failed to create gateway: %v", err)
	}
	return gw
}

func TestHealthEndpoint(t *testing.T) {
	gw := newTestGateway(t)
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/admin/health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", body)
	}
}

func TestStatusEndpoint(t *testing.T) {
	gw := newTestGateway(t)
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/admin/status")
	if err != nil {
		t.Fatalf("status check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuthBlocksV1Endpoint(t *testing.T) {
	cfg := testConfig()
	cfg.Auth.Enabled = true
	cfg.Auth.Keys = []config.APIKey{{Key: "sk-test", Name: "test", Roles: []string{"admin"}}}

	gw, err := server.NewForTest(cfg)
	if err != nil {
		t.Fatalf("failed to create gateway: %v", err)
	}
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()

	// V1 endpoint without auth should be rejected
	req, _ := http.NewRequest("POST", srv.URL+"/v1/chat/completions", bytes.NewReader([]byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthAllowsV1WithKey(t *testing.T) {
	cfg := testConfig()
	cfg.Auth.Enabled = true
	cfg.Auth.Keys = []config.APIKey{{Key: "sk-test", Name: "test", Roles: []string{"admin"}}}

	gw, err := server.NewForTest(cfg)
	if err != nil {
		t.Fatalf("failed to create gateway: %v", err)
	}
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()

	req, _ := http.NewRequest("POST", srv.URL+"/v1/chat/completions", bytes.NewReader([]byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-test")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		t.Errorf("authorized request was rejected with 401")
	}
}

func TestAdminEndpointsAccessible(t *testing.T) {
	gw := newTestGateway(t)
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()

	endpoints := []string{
		"/admin/health",
		"/admin/status",
		"/admin/upstreams",
		"/admin/routes",
		"/admin/keys",
		"/admin/config",
		"/admin/audit-logs",
		"/admin/cost-stats",
		"/admin/prompts",
		"/admin/webhook",
		"/admin/plugins",
	}

	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			resp, err := http.Get(srv.URL + ep)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("%s: expected 200, got %d", ep, resp.StatusCode)
			}
		})
	}
}

func TestMetricsEndpoint(t *testing.T) {
	gw := newTestGateway(t)
	srv := httptest.NewServer(gw.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
