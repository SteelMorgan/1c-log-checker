package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// InitTracer initializes OpenTelemetry tracer (stub for now)
// TODO: Implement full OpenTelemetry setup with exporter
func InitTracer(serviceName string) (func(context.Context) error, error) {
	// For now, use a no-op tracer
	// In production, configure OTLP exporter to Jaeger/Tempo/etc.
	
	tracer := trace.NewNoopTracerProvider()
	otel.SetTracerProvider(tracer)

	// Return a no-op shutdown function
	shutdown := func(ctx context.Context) error {
		return nil
	}

	return shutdown, nil
}

