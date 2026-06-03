package middleware

import (
	"net/http"
	"strings"
)

// AdminPermission enforces that admin write endpoints are only accessible by admin role.
// Read endpoints (GET) are accessible by all authenticated users.
type AdminPermission struct{}

func NewAdminPermission() *AdminPermission {
	return &AdminPermission{}
}

func (p *AdminPermission) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip permission check for proxy endpoints and MCP
		if strings.HasPrefix(r.URL.Path, "/v1/") || r.URL.Path == "/admin/login" || strings.HasPrefix(r.URL.Path, "/mcp") {
			next.ServeHTTP(w, r)
			return
		}

		// GET requests are read-only, accessible by all roles
		if r.Method == "GET" || r.Method == "HEAD" {
			next.ServeHTTP(w, r)
			return
		}

		// Write operations require admin role
		role, _ := r.Context().Value(APIKeyRoleContextKey).(string)
		keyName, _ := r.Context().Value(APIKeyNameContextKey).(string)
		if role != AdminRole {
			http.Error(w, `{"error":{"message":"Admin permission required for write operations","type":"permission_error","code":"forbidden"}}`, http.StatusForbidden)
			return
		}
		_ = keyName
		next.ServeHTTP(w, r)
	})
}
