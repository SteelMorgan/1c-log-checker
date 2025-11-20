package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// TracerConfig holds configuration for OpenTelemetry tracer
type TracerConfig struct {
	ServiceName    string
	ServiceVersion string
	Endpoint       string // OTLP endpoint (e.g., "localhost:4317" for gRPC, "http://localhost:4318" for HTTP)
	Protocol       string // "grpc" or "http"
	Enabled        bool
}

// InitTracer initializes OpenTelemetry tracer with OTLP exporter
func InitTracer(cfg TracerConfig) (func(context.Context) error, error) {
	if !cfg.Enabled {
		// Return no-op tracer if disabled
		tracer := trace.NewNoopTracerProvider()
		otel.SetTracerProvider(tracer)
		return func(ctx context.Context) error { return nil }, nil
	}

	// Create resource with service information
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceVersionKey.String(cfg.ServiceVersion),
		),
		resource.WithFromEnv(), // Read OTEL_RESOURCE_ATTRIBUTES from environment
		resource.WithProcess(),  // Add process attributes
		resource.WithOS(),      // Add OS attributes
		resource.WithHost(),    // Add host attributes
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create OTLP exporter based on protocol
	var client otlptrace.Client
	switch cfg.Protocol {
	case "grpc":
		if cfg.Endpoint == "" {
			cfg.Endpoint = "localhost:4317"
		}
		client = otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpoint(cfg.Endpoint),
			otlptracegrpc.WithInsecure(), // Use TLS in production
		)
	case "http":
		if cfg.Endpoint == "" {
			cfg.Endpoint = "http://localhost:4318"
		}
		client = otlptracehttp.NewClient(
			otlptracehttp.WithEndpoint(cfg.Endpoint),
			otlptracehttp.WithInsecure(), // Use TLS in production
		)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s (use 'grpc' or 'http')", cfg.Protocol)
	}

	// Create exporter
	exporter, err := otlptrace.New(context.Background(), client)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create tracer provider with batch span processor
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithMaxExportBatchSize(512),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // Sample all spans (adjust for production)
	)

	otel.SetTracerProvider(tp)

	// Return shutdown function
	shutdown := func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return tp.Shutdown(ctx)
	}

	return shutdown, nil
}
