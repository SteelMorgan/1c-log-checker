package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/1c-log-checker/internal/clickhouse"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// GetActualLogTimestampHandler handles get_actual_log_timestamp MCP tool
type GetActualLogTimestampHandler struct {
	ch *clickhouse.Client
}

// NewGetActualLogTimestampHandler creates a new handler
func NewGetActualLogTimestampHandler(ch *clickhouse.Client) *GetActualLogTimestampHandler {
	return &GetActualLogTimestampHandler{
		ch: ch,
	}
}

// GetActualLogTimestamp retrieves the maximum timestamp from tech_log table for given infobase
func (h *GetActualLogTimestampHandler) GetActualLogTimestamp(ctx context.Context, baseID string) (string, error) {
	// Start span for the entire operation
	ctx, span := startSpan(ctx, "handlers.GetActualLogTimestamp",
		attribute.String("handler", "get_actual_log_timestamp"),
		attribute.String("base_id", baseID),
	)
	defer func() {
		if err := recover(); err != nil {
			span.RecordError(fmt.Errorf("panic: %v", err))
			span.SetStatus(codes.Error, fmt.Sprintf("panic: %v", err))
			span.End()
			panic(err)
		}
	}()

	// Validate base_id (infobase_guid)
	if err := ValidateGUID(baseID, "base_id"); err != nil {
		endSpanWithError(span, err, "validation failed")
		return "", err
	}

	// Query for maximum timestamp
	query := `
		SELECT MAX(ts) as max_timestamp
		FROM logs.tech_log
		WHERE infobase_guid = ?
	`

	// Execute query with span
	_, querySpan := startSpan(ctx, "clickhouse.query",
		attribute.String("db.system", "clickhouse"),
		attribute.String("db.name", "logs"),
		attribute.String("db.sql.table", "tech_log"),
	)

	rows, err := h.ch.Query(ctx, query, baseID)
	if err != nil {
		endSpanWithError(querySpan, err, "query execution failed")
		endSpanWithError(span, err, "query failed")
		return "", fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	// Read result - use nullable time.Time to handle NULL values
	var maxTimestamp *time.Time
	if rows.Next() {
		if err := rows.Scan(&maxTimestamp); err != nil {
			endSpanWithError(querySpan, err, "scan failed")
			endSpanWithError(span, err, "scan failed")
			return "", fmt.Errorf("scan failed: %w", err)
		}
	}

	if err := rows.Err(); err != nil {
		endSpanWithError(querySpan, err, "rows error")
		endSpanWithError(span, err, "rows error")
		return "", fmt.Errorf("rows error: %w", err)
	}

	endSpanSuccess(querySpan)

	// Build response
	response := map[string]interface{}{
		"base_id": baseID,
	}

	// Check if result is NULL or empty
	if maxTimestamp == nil {
		// No records found
		response["max_timestamp"] = nil
		response["has_data"] = false
		span.SetAttributes(attribute.Bool("result.has_data", false))
	} else {
		// Format timestamp
		response["max_timestamp"] = maxTimestamp.Format(time.RFC3339Nano)
		response["has_data"] = true
		span.SetAttributes(
			attribute.Bool("result.has_data", true),
			attribute.String("result.max_timestamp", maxTimestamp.Format(time.RFC3339Nano)),
		)
	}

	// Convert to JSON
	_, jsonSpan := startSpan(ctx, "json.marshal")
	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		endSpanWithError(jsonSpan, err, "json marshal failed")
		endSpanWithError(span, err, "json marshal failed")
		return "", fmt.Errorf("json marshal failed: %w", err)
	}
	jsonSpan.SetAttributes(attribute.Int("json.size_bytes", len(jsonData)))
	endSpanSuccess(jsonSpan)

	endSpanSuccess(span)
	return string(jsonData), nil
}

