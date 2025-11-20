package handlers

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

const (
	tracerName = "1c-log-checker/handlers"
)

// startSpan creates a new span for handler operation
func startSpan(ctx context.Context, operationName string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, operationName)
	
	// Add default attributes
	span.SetAttributes(
		semconv.ServiceNameKey.String("1c-log-checker"),
	)
	
	// Add custom attributes
	if len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
	
	return ctx, span
}

// endSpanWithError marks span as failed and ends it
func endSpanWithError(span trace.Span, err error, msg string) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, fmt.Sprintf("%s: %v", msg, err))
	} else {
		span.SetStatus(codes.Ok, msg)
	}
	span.End()
}

// endSpanSuccess marks span as successful and ends it
func endSpanSuccess(span trace.Span) {
	span.SetStatus(codes.Ok, "success")
	span.End()
}

