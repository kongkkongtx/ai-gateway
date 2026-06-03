// Package mcp implements MCP (Model Context Protocol) server aggregation hub.
// It allows the gateway to connect to multiple backend MCP servers, aggregate
// their tools, and expose a unified MCP endpoint where all tool invocations
// go through the gateway's governance (auth, rate limit, audit, cost tracking).
package mcp

// Config defines the MCP hub configuration.
type Config struct {
	Enabled bool        `yaml:"enabled" json:"enabled"`
	Servers []MCPServer `yaml:"servers" json:"servers"`
}

// MCPServer defines a backend MCP server to connect to.
type MCPServer struct {
	Name        string     `yaml:"name" json:"name"`                   // unique name, used as tool namespace prefix
	Description string     `yaml:"description,omitempty" json:"description,omitempty"`
	Transport   string     `yaml:"transport" json:"transport"`          // "http" | "stdio"
	Endpoint    string     `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`       // HTTP endpoint URL
	Command     string     `yaml:"command,omitempty" json:"command,omitempty"`         // stdio command (reserved)
	Args        []string   `yaml:"args,omitempty" json:"args,omitempty"`               // stdio args (reserved)
	Timeout     string     `yaml:"timeout,omitempty" json:"timeout,omitempty"`         // request timeout, e.g. "30s"
	Enabled     bool       `yaml:"enabled" json:"enabled"`
	ToolFilter  *ToolFilter `yaml:"tool_filter,omitempty" json:"tool_filter,omitempty"`
}

// ToolFilter defines allow/deny rules for tools from a server.
type ToolFilter struct {
	Allow []string `yaml:"allow,omitempty" json:"allow,omitempty"` // allowlist (empty = all allowed)
	Deny  []string `yaml:"deny,omitempty" json:"deny,omitempty"`   // denylist
}
