package mcp

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseNamespacedName(t *testing.T) {
	tests := []struct {
		input        string
		wantServer   string
		wantTool     string
	}{
		{"serverA__get_weather", "serverA", "get_weather"},
		{"serverB__search", "serverB", "search"},
		{"no_delimiter", "", "no_delimiter"},
		{"multi__level__tool", "multi", "level__tool"},
	}
	for _, tt := range tests {
		server, tool := parseNamespacedName(tt.input)
		if server != tt.wantServer || tool != tt.wantTool {
			t.Errorf("parseNamespacedName(%q) = (%q, %q), want (%q, %q)",
				tt.input, server, tool, tt.wantServer, tt.wantTool)
		}
	}
}

func TestApplyToolFilterDeny(t *testing.T) {
	tools := []ToolDef{
		{Name: "get_weather", Description: "Get weather"},
		{Name: "search_docs", Description: "Search docs"},
		{Name: "send_email", Description: "Send email"},
	}
	filter := &ToolFilter{
		Deny: []string{"send_email"},
	}
	filtered := applyToolFilter(tools, filter)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 tools after deny filter, got %d", len(filtered))
	}
	for _, tt := range filtered {
		if tt.Name == "send_email" {
			t.Fatal("send_email should have been denied")
		}
	}
}

func TestApplyToolFilterAllow(t *testing.T) {
	tools := []ToolDef{
		{Name: "get_weather", Description: "Get weather"},
		{Name: "search_docs", Description: "Search docs"},
	}
	filter := &ToolFilter{
		Allow: []string{"get_weather"},
	}
	filtered := applyToolFilter(tools, filter)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 tool after allow filter, got %d", len(filtered))
	}
	if filtered[0].Name != "get_weather" {
		t.Fatalf("expected get_weather, got %s", filtered[0].Name)
	}
}

func TestApplyToolFilterAllowWildcard(t *testing.T) {
	tools := []ToolDef{
		{Name: "get_weather"},
		{Name: "get_stock"},
		{Name: "search_docs"},
	}
	filter := &ToolFilter{
		Allow: []string{"get_*"},
	}
	filtered := applyToolFilter(tools, filter)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 tools after wildcard allow, got %d", len(filtered))
	}
}

func TestApplyToolFilterNil(t *testing.T) {
	tools := []ToolDef{{Name: "a"}, {Name: "b"}}
	filtered := applyToolFilter(tools, nil)
	if len(filtered) != 2 {
		t.Fatalf("expected all tools with nil filter")
	}
}

func TestMatchWildcard(t *testing.T) {
	tests := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"get_*", "get_weather", true},
		{"get_*", "search_docs", false},
		{"*_weather", "get_weather", true},
		{"*_weather", "get_stock", false},
		{"exact", "exact", true},
		{"exact", "different", false},
	}
	for _, tt := range tests {
		got := matchWildcard(tt.pattern, tt.name)
		if got != tt.want {
			t.Errorf("matchWildcard(%q, %q) = %v, want %v", tt.pattern, tt.name, got, tt.want)
		}
	}
}

func TestParseTimeout(t *testing.T) {
	if d := parseTimeout("", 30*time.Second); d != 30*time.Second {
		t.Fatalf("expected default")
	}
	if d := parseTimeout("5s", 30*time.Second); d != 5*time.Second {
		t.Fatalf("expected 5s")
	}
	if d := parseTimeout("invalid", 30*time.Second); d != 30*time.Second {
		t.Fatalf("expected default for invalid")
	}
}

func TestHubListToolsEmpty(t *testing.T) {
	hub := NewHub(Config{}, slog.Default())
	tools := hub.ListTools()
	if len(tools) != 0 {
		t.Fatalf("expected empty tools list, got %d", len(tools))
	}
}

func TestHubGetServerInfoEmpty(t *testing.T) {
	hub := NewHub(Config{}, slog.Default())
	info := hub.GetServerInfo()
	if len(info) != 0 {
		t.Fatalf("expected empty server info, got %d", len(info))
	}
}

// mockMCPServer returns a minimal MCP server for testing.
func mockMCPServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var req JSONRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		switch req.Method {
		case MethodInitialize:
			json.NewEncoder(w).Encode(NewSuccessResponse(req.ID, map[string]interface{}{
				"protocolVersion": "2025-06-18",
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{"listChanged": true},
				},
				"serverInfo": map[string]interface{}{
					"name": "mock-server", "version": "1.0",
				},
			}))
		case MethodToolsList:
			json.NewEncoder(w).Encode(NewSuccessResponse(req.ID, map[string]interface{}{
				"tools": []map[string]interface{}{
					{
						"name":        "get_weather",
						"description": "Get current weather",
						"inputSchema": map[string]interface{}{
							"type":     "object",
							"required": []string{"location"},
							"properties": map[string]interface{}{
								"location": map[string]interface{}{"type": "string"},
							},
						},
					},
					{
						"name":        "search",
						"description": "Search the web",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"query": map[string]interface{}{"type": "string"},
							},
						},
					},
				},
			}))
		case MethodToolsCall:
			var params ToolsCallParams
			json.Unmarshal(req.Params, &params)
			json.NewEncoder(w).Encode(NewSuccessResponse(req.ID, NewToolCallResult(
				"executed: "+params.Name+" with args")))
		default:
			json.NewEncoder(w).Encode(NewErrorResponse(req.ID, ErrCodeMethodNotFound,
				"unknown method: "+req.Method, nil))
		}
	}))
}

func TestHubConnectAndListTools(t *testing.T) {
	mock := mockMCPServer()
	defer mock.Close()

	cfg := Config{
		Enabled: true,
		Servers: []MCPServer{
			{
				Name:      "weather-svc",
				Transport: "http",
				Endpoint:  mock.URL,
				Timeout:   "5s",
				Enabled:   true,
			},
		},
	}

	hub := NewHub(cfg, slog.Default())
	hub.ConnectAll()

	tools := hub.ListTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 aggregated tools, got %d", len(tools))
	}

	// Check namespace prefix
	if !strings.HasPrefix(tools[0].Name, "weather-svc__") {
		t.Fatalf("expected namespace prefix, got %s", tools[0].Name)
	}

	// Check tool names
	names := make(map[string]bool)
	for _, tt := range tools {
		names[tt.Name] = true
	}
	if !names["weather-svc__get_weather"] {
		t.Fatal("expected weather-svc__get_weather")
	}
	if !names["weather-svc__search"] {
		t.Fatal("expected weather-svc__search")
	}
}

func TestHubServeHTTPInitialize(t *testing.T) {
	hub := NewHub(Config{}, slog.Default())

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	req := httptest.NewRequest("POST", "/mcp", strings.NewReader(body))
	w := httptest.NewRecorder()
	hub.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      float64         `json:"id"`
		Result  InitializeResult `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Result.ServerInfo.Name != "ai-gateway" {
		t.Fatalf("expected ai-gateway, got %s", resp.Result.ServerInfo.Name)
	}
	if resp.Result.Capabilities.Tools == nil {
		t.Fatal("expected tools capability")
	}
}

func TestHubServeHTTPToolsList(t *testing.T) {
	hub := NewHub(Config{}, slog.Default())

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	req := httptest.NewRequest("POST", "/mcp", strings.NewReader(body))
	w := httptest.NewRecorder()
	hub.ServeHTTP(w, req)

	var resp struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      float64         `json:"id"`
		Result  ToolsListResult `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(resp.Result.Tools) != 0 {
		t.Fatalf("expected empty tools, got %d", len(resp.Result.Tools))
	}
}

func TestHubServeHTTPPing(t *testing.T) {
	hub := NewHub(Config{}, slog.Default())

	body := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	req := httptest.NewRequest("POST", "/mcp", strings.NewReader(body))
	w := httptest.NewRecorder()
	hub.ServeHTTP(w, req)

	var resp struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      float64     `json:"id"`
		Result  interface{} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Result == nil {
		t.Fatal("expected non-nil result for ping")
	}
}

func TestHubServeHTTPUnknownMethod(t *testing.T) {
	hub := NewHub(Config{}, slog.Default())

	body := `{"jsonrpc":"2.0","id":1,"method":"unknown_method"}`
	req := httptest.NewRequest("POST", "/mcp", strings.NewReader(body))
	w := httptest.NewRecorder()
	hub.ServeHTTP(w, req)

	var resp struct {
		JSONRPC string   `json:"jsonrpc"`
		ID      float64  `json:"id"`
		Error   RPCError `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Error.Code != ErrCodeMethodNotFound {
		t.Fatalf("expected method not found, got code %d", resp.Error.Code)
	}
}

func TestHubServeHTTPParseError(t *testing.T) {
	hub := NewHub(Config{}, slog.Default())

	req := httptest.NewRequest("POST", "/mcp", strings.NewReader(`{invalid json`))
	w := httptest.NewRecorder()
	hub.ServeHTTP(w, req)

	var resp struct {
		JSONRPC string   `json:"jsonrpc"`
		ID      interface{} `json:"id"`
		Error   RPCError `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Error.Code != ErrCodeParse {
		t.Fatalf("expected parse error, got code %d", resp.Error.Code)
	}
}

func TestHubServeHTTPWrongMethod(t *testing.T) {
	hub := NewHub(Config{}, slog.Default())

	req := httptest.NewRequest("GET", "/mcp", nil)
	w := httptest.NewRecorder()
	hub.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHubCallToolWithNamespace(t *testing.T) {
	mock := mockMCPServer()
	defer mock.Close()

	cfg := Config{
		Enabled: true,
		Servers: []MCPServer{
			{
				Name:      "svc",
				Transport: "http",
				Endpoint:  mock.URL,
				Timeout:   "5s",
				Enabled:   true,
			},
		},
	}

	hub := NewHub(cfg, slog.Default())
	hub.ConnectAll()

	result, err := hub.CallTool("svc__get_weather", map[string]interface{}{"location": "NYC"})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	if result.IsError {
		t.Fatal("expected no error")
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(result.Content))
	}
	if !strings.Contains(result.Content[0].Text, "executed: get_weather") {
		t.Fatalf("unexpected response: %s", result.Content[0].Text)
	}
}

func TestHubCallToolNoNamespace(t *testing.T) {
	hub := NewHub(Config{}, slog.Default())
	_, err := hub.CallTool("no_prefix_tool", nil)
	if err == nil {
		t.Fatal("expected error for tool without namespace")
	}
}

func TestHubCallToolUnknownServer(t *testing.T) {
	hub := NewHub(Config{}, slog.Default())
	_, err := hub.CallTool("unknown__tool", nil)
	if err == nil {
		t.Fatal("expected error for unknown server")
	}
}

func TestHubServerInfo(t *testing.T) {
	mock := mockMCPServer()
	defer mock.Close()

	cfg := Config{
		Enabled: true,
		Servers: []MCPServer{
			{Name: "s1", Transport: "http", Endpoint: mock.URL, Timeout: "5s", Enabled: true},
		},
	}
	hub := NewHub(cfg, slog.Default())
	hub.ConnectAll()

	info := hub.GetServerInfo()
	if len(info) != 1 {
		t.Fatalf("expected 1 server in info, got %d", len(info))
	}
	s1 := info["s1"].(map[string]interface{})
	if s1["healthy"] != true {
		t.Fatal("expected healthy=true")
	}
}

func TestHubReload(t *testing.T) {
	hub := NewHub(Config{}, slog.Default())
	hub.Reload(Config{Enabled: true, Servers: []MCPServer{}})
	if hub == nil {
		t.Fatal("hub should not be nil after reload")
	}
}

func TestHubServeHTTPToolsCallViaHandler(t *testing.T) {
	mock := mockMCPServer()
	defer mock.Close()

	cfg := Config{
		Enabled: true,
		Servers: []MCPServer{
			{Name: "svc", Transport: "http", Endpoint: mock.URL, Timeout: "5s", Enabled: true},
		},
	}
	hub := NewHub(cfg, slog.Default())
	hub.ConnectAll()

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"svc__get_weather","arguments":{"location":"NYC"}}}`
	req := httptest.NewRequest("POST", "/mcp", strings.NewReader(body))
	w := httptest.NewRecorder()
	hub.ServeHTTP(w, req)

	var resp struct {
		JSONRPC string         `json:"jsonrpc"`
		ID      float64        `json:"id"`
		Result  ToolsCallResult `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Result.IsError {
		t.Fatal("expected no error from tools/call handler")
	}
}
