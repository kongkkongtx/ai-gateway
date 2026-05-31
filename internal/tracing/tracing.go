// Package tracing sets up OpenTelemetry for distributed tracing.
// Traces are exported to stdout in pretty-printed JSON format by default.
package tracing

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/yushi/ai-gateway"

// Init creates and configures a TracerProvider with a stdout span exporter.
// The exporter writes JSON traces to stdout for development visibility.
// In production, this can be replaced with an OTLP exporter sending to a collector.
func Init(serviceName string) (*sdktrace.TracerProvider, error) {
	exporter, err := stdouttrace.New(
		stdouttrace.WithWriter(os.Stdout),
		stdouttrace.WithPrettyPrint(),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			attribute.String("service.name", serviceName),
			attribute.String("service.version", "0.1.0"),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	return tp, nil
}

// Tracer returns the application's named tracer for creating spans.
func Tracer() trace.Tracer {
	return otel.Tracer(tracerName)
}