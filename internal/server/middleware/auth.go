// Package middleware provides the HTTP middleware pipeline for the gateway.
package middleware

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
)

type contextKey string

const APIKeyNameContextKey contextKey = "api_key_name" // Context key to store the authenticated key name

// Auth validates API keys from the Authorization header.
// Uses constant-time comparison to prevent timing attacks.
// Skips auth for admin and metrics endpoints.
type Auth struct {
	enabled bool
	keys    map[string]string // Maps API key -> human-readable name
	logger  *slog.Logger
}

// NewAuth creates an Auth middleware. When enabled is false, all requests pass through.
func NewAuth(enabled bool, keys map[string]string, logger *slog.Logger) *Auth {
	return &Auth{enabled: enabled, keys: keys, logger: logger}
}

// Middleware returns an HTTP handler that enforces API key authentication.
func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.enabled {
			next.ServeHTTP(w, r)
			return
		}
		// Allow unauthenticated access to admin and observability endpoints
		if !strings.HasPrefix(r.URL.Path, "/v1/") {
			next.ServeHTTP(w, r)
			return
		}
		apiKey := r.Header.Get("Authorization")
		if apiKey == "" {
			http.Error(w, `{"error":{"message":"Missing API key","type":"auth_error","code":"missing_api_key"}}`, http.StatusUnauthorized)
			return
		}
		// Strip "Bearer " prefix if present
		if len(apiKey) > 7 && apiKey[:7] == "Bearer " {
			apiKey = apiKey[7:]
		}
		name, ok := a.validate(apiKey)
		if !ok {
			http.Error(w, `{"error":{"message":"Invalid API key","type":"auth_error","code":"invalid_api_key"}}`, http.StatusUnauthorized)
			return
		}
		// Store the key name in context for downstream use (rate limiting, audit logging)
		ctx := context.WithValue(r.Context(), APIKeyNameContextKey, name)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// validate checks the provided API key against the configured keys.
// Uses ConstantTimeCompare to prevent timing side-channel attacks.
func (a *Auth) validate(apiKey string) (string, bool) {
	for key, name := range a.keys {
		if subtle.ConstantTimeCompare([]byte(apiKey), []byte(key)) == 1 {
			return name, true
		}
	}
	return "", false
}
