package middleware

import (
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/yushi/ai-gateway/internal/tracing"
)

// TracingMiddleware creates an OpenTelemetry span for each HTTP request.
// The span captures method, path, host, and client IP, and is propagated
// to downstream handler code where AI-specific attributes are attached.
type TracingMiddleware struct {
	tracer trace.Tracer
}

// NewTracingMiddleware creates a TracingMiddleware using the global OTel tracer.
func NewTracingMiddleware() *TracingMiddleware {
	return &TracingMiddleware{
		tracer: tracing.Tracer(),
	}
}

// Middleware returns an HTTP handler that wraps the request in an OTel span.
func (m *TracingMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		spanName := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		ctx, span := m.tracer.Start(r.Context(), spanName,
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.path", r.URL.Path),
				attribute.String("http.host", r.Host),
				attribute.String("net.peer.ip", r.RemoteAddr),
			),
		)
		defer span.End()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}