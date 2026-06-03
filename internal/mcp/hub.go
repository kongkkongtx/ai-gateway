package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// NamespaceDelimiter separates server name from tool name in aggregated tools.
const NamespaceDelimiter = "__"

// Hub is the MCP server connection hub that aggregates multiple backend MCP servers
// and exposes them as a single MCP endpoint.
type Hub struct {
	mu      sync.RWMutex
	cfg     Config
	servers map[string]*serverConnection
	logger  *slog.Logger
}

// serverConnection holds the state for a backend MCP server.
type serverConnection struct {
	cfg        MCPServer
	client     *http.Client
	healthy    bool
	tools      []ToolDef
	lastErr    string
}

// NewHub creates a new MCP Hub from config.
func NewHub(cfg Config, logger *slog.Logger) *Hub {
	h := &Hub{
		cfg:     cfg,
		servers: make(map[string]*serverConnection),
		logger:  logger,
	}
	return h
}

// ConnectAll establishes connections to all enabled backend MCP servers.
func (h *Hub) ConnectAll() {
	for _, s := range h.cfg.Servers {
		if !s.Enabled {
			continue
		}
		h.connectServer(s)
	}
}

func (h *Hub) connectServer(srv MCPServer) {
	conn := &serverConnection{
		cfg: srv,
		client: &http.Client{
			Timeout: parseTimeout(srv.Timeout, 30*time.Second),
		},
	}

	// Perform MCP initialize handshake
	if err := h.doInitialize(conn); err != nil {
		conn.healthy = false
		conn.lastErr = err.Error()
		h.logger.Warn("mcp server initialize failed",
			"name", srv.Name, "endpoint", srv.Endpoint, "error", err)
	} else {
		// Fetch tools list
		if tools, err := h.fetchTools(conn); err != nil {
			conn.lastErr = err.Error()
			h.logger.Warn("mcp server tools/list failed",
				"name", srv.Name, "endpoint", srv.Endpoint, "error", err)
		} else {
			conn.tools = tools
			conn.healthy = true
			h.logger.Info("mcp server connected",
				"name", srv.Name, "tools", len(tools))
		}
	}

	h.mu.Lock()
	h.servers[srv.Name] = conn
	h.mu.Unlock()
}

// doInitialize sends initialize request to the backend MCP server.
func (h *Hub) doInitialize(conn *serverConnection) error {
	initReq := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      "init-1",
		Method:  MethodInitialize,
	}

	body, err := json.Marshal(initReq)
	if err != nil {
		return fmt.Errorf("marshal init request: %w", err)
	}

	resp, err := conn.client.Post(conn.cfg.Endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("post init request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("init returned status %d", resp.StatusCode)
	}

	var rpcResp JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return fmt.Errorf("decode init response: %w", err)
	}

	// Send initialized notification (fire-and-forget)
	notif := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		Method:  MethodInitialized,
	}
	notifBody, _ := json.Marshal(notif)
	conn.client.Post(conn.cfg.Endpoint, "application/json", bytes.NewReader(notifBody))

	return nil
}

// fetchTools sends tools/list to the backend and returns the parsed tools.
func (h *Hub) fetchTools(conn *serverConnection) ([]ToolDef, error) {
	req := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      "tools-1",
		Method:  MethodToolsList,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal tools/list: %w", err)
	}

	resp, err := conn.client.Post(conn.cfg.Endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("post tools/list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("tools/list returned status %d", resp.StatusCode)
	}

	var rpcResp struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      interface{} `json:"id"`
		Result  struct {
			Tools      []ToolDef `json:"tools"`
			NextCursor string    `json:"nextCursor,omitempty"`
		} `json:"result"`
		Error *RPCError `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("decode tools/list response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("tools/list error: %s", rpcResp.Error.Message)
	}

	tools := rpcResp.Result.Tools

	// Apply tool filters
	if conn.cfg.ToolFilter != nil {
		tools = applyToolFilter(tools, conn.cfg.ToolFilter)
	}

	return tools, nil
}

// applyToolFilter filters tools based on allow/deny lists.
func applyToolFilter(tools []ToolDef, filter *ToolFilter) []ToolDef {
	if filter == nil {
		return tools
	}

	denySet := make(map[string]bool)
	for _, d := range filter.Deny {
		denySet[d] = true
	}

	var result []ToolDef
	for _, t := range tools {
		if denySet[t.Name] {
			continue
		}
		if len(filter.Allow) > 0 {
			allowed := false
			for _, a := range filter.Allow {
				if a == t.Name || matchWildcard(a, t.Name) {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}
		result = append(result, t)
	}
	return result
}

// matchWildcard checks if a pattern matches a name with * wildcard.
func matchWildcard(pattern, name string) bool {
	if !strings.Contains(pattern, "*") {
		return pattern == name
	}
	parts := strings.Split(pattern, "*")
	if len(parts) != 2 {
		return pattern == name
	}
	return strings.HasPrefix(name, parts[0]) && strings.HasSuffix(name, parts[1])
}

// ListTools returns all tools aggregated from all healthy backend servers,
// with namespace prefixes applied.
func (h *Hub) ListTools() []ToolDef {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var all []ToolDef
	for name, conn := range h.servers {
		if !conn.healthy {
			continue
		}
		for _, t := range conn.tools {
			prefixed := ToolDef{
				Name:        name + NamespaceDelimiter + t.Name,
				Description: t.Description,
				InputSchema: t.InputSchema,
			}
			all = append(all, prefixed)
		}
	}
	return all
}

// CallTool invokes a tool by name, routing to the appropriate backend server.
// The name should include the namespace prefix (e.g., "serverA__get_weather").
func (h *Hub) CallTool(name string, args map[string]interface{}) (*ToolsCallResult, error) {
	serverName, toolName := parseNamespacedName(name)
	if serverName == "" {
		return nil, fmt.Errorf("tool %q does not include server namespace", name)
	}

	h.mu.RLock()
	conn, ok := h.servers[serverName]
	h.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("mcp server %q not found", serverName)
	}
	if !conn.healthy {
		return nil, fmt.Errorf("mcp server %q is unhealthy: %s", serverName, conn.lastErr)
	}

	// Build tools/call request
	callReq := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      "call-1",
		Method:  MethodToolsCall,
	}
	callParams := ToolsCallParams{
		Name:      toolName,
		Arguments: args,
	}
	paramsData, _ := json.Marshal(callParams)
	callReq.Params = paramsData

	body, err := json.Marshal(callReq)
	if err != nil {
		return nil, fmt.Errorf("marshal tools/call: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), parseTimeout(conn.cfg.Timeout, 30*time.Second))
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, conn.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create tools/call request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := conn.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("post tools/call to %s: %w", serverName, err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 400 {
		return nil, fmt.Errorf("tools/call returned status %d from %s", httpResp.StatusCode, serverName)
	}

	var rpcResp struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      interface{}     `json:"id"`
		Result  *ToolsCallResult `json:"result,omitempty"`
		Error   *RPCError       `json:"error,omitempty"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("decode tools/call response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("tools/call error from %s: %s", serverName, rpcResp.Error.Message)
	}
	if rpcResp.Result == nil {
		return nil, fmt.Errorf("tools/call returned empty result from %s", serverName)
	}

	return rpcResp.Result, nil
}

// parseNamespacedName parses a namespaced tool name into server and tool parts.
// Returns ("", toolName) if no namespace delimiter is found.
func parseNamespacedName(name string) (server, tool string) {
	idx := strings.Index(name, NamespaceDelimiter)
	if idx < 0 {
		return "", name
	}
	return name[:idx], name[idx+len(NamespaceDelimiter):]
}

// GetServerInfo returns status info about all backend MCP servers.
func (h *Hub) GetServerInfo() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	servers := make(map[string]interface{})
	for name, conn := range h.servers {
		toolNames := make([]string, len(conn.tools))
		for i, t := range conn.tools {
			toolNames[i] = t.Name
		}
		servers[name] = map[string]interface{}{
			"name":         conn.cfg.Name,
			"endpoint":     conn.cfg.Endpoint,
			"transport":    conn.cfg.Transport,
			"enabled":      true,
			"healthy":      conn.healthy,
			"tools":        toolNames,
			"last_error":   conn.lastErr,
		}
	}
	return servers
}

// Reload replaces all server configurations and reconnects.
func (h *Hub) Reload(cfg Config) {
	h.mu.Lock()
	h.cfg = cfg
	h.servers = make(map[string]*serverConnection)
	h.mu.Unlock()
	h.ConnectAll()
	h.logger.Info("mcp hub reloaded", "servers", len(cfg.Servers))
}

// ReconnectServer disconnects and reconnects a single server.
func (h *Hub) ReconnectServer(name string) error {
	h.mu.Lock()
	delete(h.servers, name)
	h.mu.Unlock()

	var srv *MCPServer
	for _, s := range h.cfg.Servers {
		if s.Name == name {
			srv = &s
			break
		}
	}
	if srv == nil {
		return fmt.Errorf("mcp server %q not found in config", name)
	}

	h.connectServer(*srv)
	return nil
}

// parseTimeout parses a duration string with a default fallback.
func parseTimeout(s string, defaultDuration time.Duration) time.Duration {
	if s == "" {
		return defaultDuration
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultDuration
	}
	return d
}

// ServeHTTP handles HTTP requests to the MCP endpoint.
// It expects JSON-RPC 2.0 request bodies and returns JSON-RPC responses.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeHTTPError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONResponse(w, NewErrorResponse(nil, ErrCodeParse,
			"Parse error: invalid JSON", err.Error()))
		return
	}

	if req.JSONRPC != JSONRPCVersion {
		writeJSONResponse(w, NewErrorResponse(req.ID, ErrCodeInvalidRequest,
			"Invalid JSON-RPC version", nil))
		return
	}

	h.handleRequest(w, req)
}

func (h *Hub) handleRequest(w http.ResponseWriter, req JSONRPCRequest) {
	// Notifications have no ID - no response needed
	if req.ID == nil && req.Method != MethodInitialized {
		return
	}

	var resp interface{}
	switch req.Method {
	case MethodInitialize:
		resp = h.handleInitialize(req)
	case MethodPing:
		resp = NewSuccessResponse(req.ID, map[string]interface{}{})
	case MethodToolsList:
		resp = h.handleToolsList(req)
	case MethodToolsCall:
		resp = h.handleToolsCall(req)
	case MethodResourcesList:
		resp = h.handleForwardToFirst(req, MethodResourcesList)
	case MethodPromptsList:
		resp = h.handleForwardToFirst(req, MethodPromptsList)
	default:
		resp = NewErrorResponse(req.ID, ErrCodeMethodNotFound,
			fmt.Sprintf("Method not found: %s", req.Method), nil)
	}

	writeJSONResponse(w, resp)
}

func (h *Hub) handleInitialize(req JSONRPCRequest) interface{} {
	result := InitializeResult{
		ProtocolVersion: "2025-06-18",
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{
				ListChanged: true,
			},
			Resources: &ResourcesCapability{},
			Prompts:   &PromptsCapability{},
		},
		ServerInfo: ServerInfo{
			Name:    "ai-gateway",
			Version: "3.0.0",
		},
	}
	return NewSuccessResponse(req.ID, result)
}

func (h *Hub) handleToolsList(req JSONRPCRequest) interface{} {
	tools := h.ListTools()
	result := ToolsListResult{
		Tools: tools,
	}
	return NewSuccessResponse(req.ID, result)
}

func (h *Hub) handleToolsCall(req JSONRPCRequest) interface{} {
	var params ToolsCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams,
			"Invalid params", err.Error())
	}

	result, err := h.CallTool(params.Name, params.Arguments)
	if err != nil {
		return NewSuccessResponse(req.ID, NewToolCallError(err.Error()))
	}

	return NewSuccessResponse(req.ID, result)
}

// handleForwardToFirst forwards a request to the first healthy backend server.
// Used for resources/* and prompts/* that we proxy directly.
func (h *Hub) handleForwardToFirst(req JSONRPCRequest, method string) interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for name, conn := range h.servers {
		if !conn.healthy {
			continue
		}

		forwardReq := JSONRPCRequest{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Method:  method,
			Params:  req.Params,
		}

		body, _ := json.Marshal(forwardReq)
		resp, err := conn.client.Post(conn.cfg.Endpoint, "application/json", bytes.NewReader(body))
		if err != nil {
			h.logger.Warn("forward request failed",
				"method", method, "server", name, "error", err)
			continue
		}
		defer resp.Body.Close()

		var rpcResp JSONRPCResponse
		if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
			continue
		}
		return rpcResp
	}

	return NewErrorResponse(req.ID, ErrCodeInternal,
		fmt.Sprintf("No healthy server available for %s", method), nil)
}

// writeJSONResponse writes a JSON-RPC response to the HTTP response writer.
func writeJSONResponse(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// writeHTTPError writes an HTTP-level error (not JSON-RPC).
func writeHTTPError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
