package middleware

import (
	"net/http"
	"strconv"
	"strings"
)

// CORS handles Cross-Origin Resource Sharing headers.
// In production, restrict origins to specific domains.
type CORS struct {
	allowedOrigins []string
	allowedMethods []string
	allowedHeaders []string
	maxAge         int
}

// NewCORS creates a CORS middleware. If origins is empty, allows all origins (*).
func NewCORS(origins []string) *CORS {
	if len(origins) == 0 {
		origins = []string{"*"}
	}
	return &CORS{
		allowedOrigins: origins,
		allowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		allowedHeaders: []string{"Content-Type", "Authorization", "X-Requested-With"},
		maxAge:         86400,
	}
}

// Middleware returns an HTTP handler that sets CORS headers.
func (c *CORS) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			allowedOrigin := c.resolveOrigin(origin)
			if allowedOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(c.allowedMethods, ", "))
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(c.allowedHeaders, ", "))
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Max-Age", strconv.Itoa(c.maxAge))
			}
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (c *CORS) resolveOrigin(origin string) string {
	for _, allowed := range c.allowedOrigins {
		if allowed == "*" || allowed == origin {
			return allowed
		}
	}
	return ""
}
