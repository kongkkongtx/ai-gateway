// Package middleware provides the HTTP middleware pipeline for the gateway.
package middleware

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"

	"github.com/kongkkongtx/ai-gateway/internal/user"
)

type contextKey string

const APIKeyNameContextKey contextKey = "api_key_name"
const APIKeyRoleContextKey  contextKey = "api_key_role"
const JWTClaimsContextKey   contextKey = "jwt_claims"
const AdminRole                    = "admin"
const ReadonlyRole                 = "readonly"

// Auth validates API keys and JWT tokens from the Authorization header.
type Auth struct {
	enabled      bool
	keys         map[string]string
	roles        map[string][]string
	keyExpiry    map[string]string // key -> expiry timestamp (future use)
	logger       *slog.Logger
	userStore    *user.Store
}

func NewAuth(enabled bool, keys map[string]string, roles map[string][]string, logger *slog.Logger) *Auth {
	return &Auth{enabled: enabled, keys: keys, roles: roles, logger: logger}
}

// SetUserStore sets the user store for JWT authentication.
func (a *Auth) SetUserStore(store *user.Store) {
	a.userStore = store
}

// Middleware returns an HTTP handler that enforces authentication.
func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.enabled {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")

		// Try JWT Bearer token first
		if a.userStore != nil && len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token := authHeader[7:]
			// Check if it looks like a JWT (has two dots)
			if strings.Count(token, ".") == 2 {
				claims, err := user.ValidateToken(token)
				if err == nil {
					ctx := context.WithValue(r.Context(), APIKeyNameContextKey, claims.Sub)
					ctx = context.WithValue(ctx, APIKeyRoleContextKey, claims.Role)
					ctx = context.WithValue(ctx, JWTClaimsContextKey, claims)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				// JWT invalid — fall through to API key check
			}
		}

		// Admin GET/HEAD pass through for UI access
		if !strings.HasPrefix(r.URL.Path, "/v1/") && !strings.HasPrefix(r.URL.Path, "/admin/login") {
			if r.Method == "GET" || r.Method == "HEAD" {
				ctx := context.WithValue(r.Context(), APIKeyRoleContextKey, AdminRole)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		// Login endpoint bypasses auth
		if r.URL.Path == "/admin/login" && r.Method == "POST" {
			next.ServeHTTP(w, r)
			return
		}

		// API key authentication
		apiKey := authHeader
		if apiKey == "" {
			http.Error(w, `{"error":{"message":"Missing API key","type":"auth_error","code":"missing_api_key"}}`, http.StatusUnauthorized)
			return
		}
		if len(apiKey) > 7 && apiKey[:7] == "Bearer " {
			apiKey = apiKey[7:]
		}

		name, ok := a.validate(apiKey)
		if !ok {
			http.Error(w, `{"error":{"message":"Invalid API key","type":"auth_error","code":"invalid_api_key"}}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), APIKeyNameContextKey, name)
		if roles, ok := a.roles[name]; ok && len(roles) > 0 {
			ctx = context.WithValue(ctx, APIKeyRoleContextKey, roles[0])
		} else {
			ctx = context.WithValue(ctx, APIKeyRoleContextKey, AdminRole)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *Auth) validate(apiKey string) (string, bool) {
	for key, name := range a.keys {
		if subtle.ConstantTimeCompare([]byte(apiKey), []byte(key)) == 1 {
			return name, true
		}
	}
	return "", false
}
