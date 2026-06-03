package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kongkkongtx/ai-gateway/internal/config"
	"github.com/kongkkongtx/ai-gateway/internal/mcp"
)

type mcpServerReq struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Transport   string `json:"transport"`
	Endpoint    string `json:"endpoint,omitempty"`
	Timeout     string `json:"timeout,omitempty"`
	Enabled     bool   `json:"enabled"`
}

func (r *mcpServerReq) toMCPServer() mcp.MCPServer {
	return mcp.MCPServer{
		Name:        r.Name,
		Description: r.Description,
		Transport:   r.Transport,
		Endpoint:    r.Endpoint,
		Timeout:     r.Timeout,
		Enabled:     r.Enabled,
	}
}

func (g *Gateway) handleListMCPServers(w http.ResponseWriter, r *http.Request) {
	if g.mcpHub == nil {
		writeJSON(w, http.StatusOK, []mcp.MCPServer{})
		return
	}
	writeJSON(w, http.StatusOK, g.cfg.MCP.Servers)
}

func (g *Gateway) handleAddMCPServer(w http.ResponseWriter, r *http.Request) {
	if g.mcpHub == nil {
		writeError(w, http.StatusBadRequest, "MCP engine is not enabled")
		return
	}

	var req mcpServerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Server name is required")
		return
	}
	if req.Transport != "http" && req.Transport != "stdio" {
		writeError(w, http.StatusBadRequest, "Transport must be 'http' or 'stdio'")
		return
	}
	if req.Transport == "http" && req.Endpoint == "" {
		writeError(w, http.StatusBadRequest, "HTTP transport requires an endpoint")
		return
	}

	// Check for duplicate names
	for _, s := range g.cfg.MCP.Servers {
		if s.Name == req.Name {
			writeError(w, http.StatusBadRequest, "MCP server name already exists: "+req.Name)
			return
		}
	}

	srv := req.toMCPServer()
	g.cfg.MCP.Servers = append(g.cfg.MCP.Servers, srv)

	// Persist
	if _, err := config.SaveVersioned(g.cfg, g.configPath, "MCP server added: "+srv.Name); err != nil {
		g.logger.Warn("failed to persist MCP config", "error", err)
	}

	// Try to connect
	if err := g.mcpHub.ReconnectServer(srv.Name); err != nil {
		g.logger.Warn("mcp server connect failed", "name", srv.Name, "error", err)
	}

	g.logger.Info("mcp server added", "name", srv.Name)
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (g *Gateway) handleGetMCPServer(w http.ResponseWriter, r *http.Request) {
	if g.mcpHub == nil {
		writeError(w, http.StatusNotFound, "MCP engine not enabled")
		return
	}
	name := chi.URLParam(r, "name")
	for _, s := range g.cfg.MCP.Servers {
		if s.Name == name {
			writeJSON(w, http.StatusOK, s)
			return
		}
	}
	writeError(w, http.StatusNotFound, "MCP server not found")
}

func (g *Gateway) handleUpdateMCPServer(w http.ResponseWriter, r *http.Request) {
	if g.mcpHub == nil {
		writeError(w, http.StatusBadRequest, "MCP engine is not enabled")
		return
	}
	name := chi.URLParam(r, "name")

	var req mcpServerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	req.Name = name // Use URL param as name
	found := false
	for i, s := range g.cfg.MCP.Servers {
		if s.Name == name {
			g.cfg.MCP.Servers[i] = req.toMCPServer()
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "MCP server not found")
		return
	}

	if _, err := config.SaveVersioned(g.cfg, g.configPath, "MCP server updated: "+name); err != nil {
		g.logger.Warn("failed to persist MCP config", "error", err)
	}

	g.mcpHub.Reload(g.cfg.MCP)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (g *Gateway) handleDeleteMCPServer(w http.ResponseWriter, r *http.Request) {
	if g.mcpHub == nil {
		writeError(w, http.StatusNotFound, "MCP engine not enabled")
		return
	}
	name := chi.URLParam(r, "name")
	servers := g.cfg.MCP.Servers
	for i, s := range servers {
		if s.Name == name {
			g.cfg.MCP.Servers = append(servers[:i], servers[i+1:]...)
			if _, err := config.SaveVersioned(g.cfg, g.configPath, "MCP server deleted: "+name); err != nil {
				g.logger.Warn("failed to persist MCP config", "error", err)
			}
			g.mcpHub.Reload(g.cfg.MCP)
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}
	}
	writeError(w, http.StatusNotFound, "MCP server not found")
}

func (g *Gateway) handleListMCPTools(w http.ResponseWriter, r *http.Request) {
	if g.mcpHub == nil {
		writeJSON(w, http.StatusOK, []mcp.ToolDef{})
		return
	}
	tools := g.mcpHub.ListTools()
	writeJSON(w, http.StatusOK, tools)
}

func (g *Gateway) handleReconnectMCPServer(w http.ResponseWriter, r *http.Request) {
	if g.mcpHub == nil {
		writeError(w, http.StatusBadRequest, "MCP engine is not enabled")
		return
	}
	name := chi.URLParam(r, "name")
	if err := g.mcpHub.ReconnectServer(name); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (g *Gateway) handleReloadMCP(w http.ResponseWriter, r *http.Request) {
	if g.mcpHub == nil {
		writeError(w, http.StatusBadRequest, "MCP engine is not enabled")
		return
	}
	g.mcpHub.Reload(g.cfg.MCP)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
