package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/1c-log-checker/internal/clickhouse"
	"github.com/1c-log-checker/internal/mapping"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// TechLogHandler handles get_tech_log MCP tool
type TechLogHandler struct {
	ch         *clickhouse.Client
	clusterMap *mapping.ClusterMap
}

// TechLogMinimal represents minimal tech log record
type TechLogMinimal struct {
	Ts            time.Time `json:"ts"`
	Name          string    `json:"name"`
	Level         string    `json:"level"`
	Duration      uint64    `json:"duration"`
	Process       string    `json:"process"`
	Usr           string    `json:"usr"`
	SessionID     string    `json:"session_id"`
	TransactionID string    `json:"transaction_id"`
	PropertyKey   []string  `json:"property_key"`
	PropertyValue []string  `json:"property_value"`
}

// TechLogFull represents full tech log record with raw_line
type TechLogFull struct {
	TechLogMinimal
	RawLine string `json:"raw_line"`
}

// NewTechLogHandler creates a new tech log handler
func NewTechLogHandler(ch *clickhouse.Client, clusterMap *mapping.ClusterMap) *TechLogHandler {
	return &TechLogHandler{
		ch:         ch,
		clusterMap: clusterMap,
	}
}

// GetTechLog retrieves tech log records
func (h *TechLogHandler) GetTechLog(ctx context.Context, params TechLogParams) (string, error) {
	// Start span for the entire operation
	ctx, span := startSpan(ctx, "handlers.GetTechLog",
		attribute.String("handler", "tech_log"),
		attribute.String("cluster_guid", params.ClusterGUID),
		attribute.String("infobase_guid", params.InfobaseGUID),
		attribute.String("mode", params.Mode),
		attribute.Int("limit", params.Limit),
		attribute.String("event_name", params.Name),
	)
	defer func() {
		if err := recover(); err != nil {
			span.RecordError(fmt.Errorf("panic: %v", err))
			span.SetStatus(codes.Error, fmt.Sprintf("panic: %v", err))
			span.End()
			panic(err)
		}
	}()

	// Validate GUIDs
	if err := ValidateGUID(params.ClusterGUID, "cluster_guid"); err != nil {
		endSpanWithError(span, err, "validation failed")
		return "", err
	}
	if err := ValidateGUID(params.InfobaseGUID, "infobase_guid"); err != nil {
		endSpanWithError(span, err, "validation failed")
		return "", err
	}

	// Validate time range
	if err := ValidateTimeRange(params.From.Format(time.RFC3339), params.To.Format(time.RFC3339)); err != nil {
		endSpanWithError(span, err, "validation failed")
		return "", err
	}

	// Validate mode
	if err := ValidateMode(params.Mode); err != nil {
		endSpanWithError(span, err, "validation failed")
		return "", err
	}

	// Set defaults
	if params.Mode == "" {
		params.Mode = "minimal"
	}
	if params.Limit <= 0 {
		params.Limit = 1000
	}

	// Build query - always select specific columns (ClickHouse Go driver doesn't support MapScan)
	query := `
		SELECT
			ts,
			name,
			level,
			duration,
			process,
			usr,
			session_id,
			transaction_id,
			property_key,
			property_value,
			raw_line
		FROM logs.tech_log
		WHERE cluster_guid = ? AND infobase_guid = ?
		  AND ts BETWEEN ? AND ?
	`
	
	// Add event name filter if specified
	if params.Name != "" {
		query += " AND name = ?"
	}
	
	query += " ORDER BY ts DESC LIMIT ?"
	
	// Build args
	args := []interface{}{
		params.ClusterGUID,
		params.InfobaseGUID,
		params.From,
		params.To,
	}
	if params.Name != "" {
		args = append(args, params.Name)
	}
	args = append(args, params.Limit)
	
	// Execute query with span
	_, querySpan := startSpan(ctx, "clickhouse.query",
		attribute.String("query.mode", params.Mode),
		attribute.String("db.system", "clickhouse"),
		attribute.String("db.name", "logs"),
		attribute.String("db.sql.table", "tech_log"),
	)
	
	rows, err := h.ch.Query(ctx, query, args...)
	if err != nil {
		endSpanWithError(querySpan, err, "query execution failed")
		endSpanWithError(span, err, "query failed")
		return "", fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var jsonData []byte
	var resultsCount int

	// Collect results based on mode
	if params.Mode == "minimal" {
		var results []TechLogMinimal
		for rows.Next() {
			var record TechLogMinimal
			var rawLine string // Read but discard for minimal mode
			if err := rows.Scan(
				&record.Ts,
				&record.Name,
				&record.Level,
				&record.Duration,
				&record.Process,
				&record.Usr,
				&record.SessionID,
				&record.TransactionID,
				&record.PropertyKey,
				&record.PropertyValue,
				&rawLine,
			); err != nil {
				endSpanWithError(querySpan, err, "scan failed")
				endSpanWithError(span, err, "scan failed")
				return "", fmt.Errorf("scan failed: %w", err)
			}
			results = append(results, record)
		}

		if err := rows.Err(); err != nil {
			endSpanWithError(querySpan, err, "rows error")
			endSpanWithError(span, err, "rows error")
			return "", fmt.Errorf("rows error: %w", err)
		}

		resultsCount = len(results)
		querySpan.SetAttributes(attribute.Int("db.rows.count", resultsCount))
		endSpanSuccess(querySpan)

		// Convert to JSON
		_, jsonSpan := startSpan(ctx, "json.marshal")
		jsonData, err = json.MarshalIndent(results, "", "  ")
		if err != nil {
			endSpanWithError(jsonSpan, err, "json marshal failed")
			endSpanWithError(span, err, "json marshal failed")
			return "", fmt.Errorf("json marshal failed: %w", err)
		}
		jsonSpan.SetAttributes(attribute.Int("json.size_bytes", len(jsonData)))
		endSpanSuccess(jsonSpan)
	} else {
		// Full mode: Include raw_line
		var results []TechLogFull
		for rows.Next() {
			var record TechLogFull
			if err := rows.Scan(
				&record.Ts,
				&record.Name,
				&record.Level,
				&record.Duration,
				&record.Process,
				&record.Usr,
				&record.SessionID,
				&record.TransactionID,
				&record.PropertyKey,
				&record.PropertyValue,
				&record.RawLine,
			); err != nil {
				endSpanWithError(querySpan, err, "scan failed")
				endSpanWithError(span, err, "scan failed")
				return "", fmt.Errorf("scan failed: %w", err)
			}
			results = append(results, record)
		}

		if err := rows.Err(); err != nil {
			endSpanWithError(querySpan, err, "rows error")
			endSpanWithError(span, err, "rows error")
			return "", fmt.Errorf("rows error: %w", err)
		}

		resultsCount = len(results)
		querySpan.SetAttributes(attribute.Int("db.rows.count", resultsCount))
		endSpanSuccess(querySpan)

		// Convert to JSON
		_, jsonSpan := startSpan(ctx, "json.marshal")
		jsonData, err = json.MarshalIndent(results, "", "  ")
		if err != nil {
			endSpanWithError(jsonSpan, err, "json marshal failed")
			endSpanWithError(span, err, "json marshal failed")
			return "", fmt.Errorf("json marshal failed: %w", err)
		}
		jsonSpan.SetAttributes(attribute.Int("json.size_bytes", len(jsonData)))
		endSpanSuccess(jsonSpan)
	}

	span.SetAttributes(attribute.Int("result.records_count", resultsCount))
	endSpanSuccess(span)
	return string(jsonData), nil
}

// TechLogParams defines parameters for get_tech_log tool
type TechLogParams struct {
	ClusterGUID  string
	InfobaseGUID string
	From         time.Time
	To           time.Time
	Name         string // Optional filter: EXCP, DBMSSQL, TLOCK, etc.
	Mode         string // minimal or full
	Limit        int    // Max records to return
}

