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

// NewErrorsHandler handles get_new_errors MCP tool
type NewErrorsHandler struct {
	ch *clickhouse.Client
}

// NewNewErrorsHandler creates a new errors handler
func NewNewErrorsHandler(ch *clickhouse.Client) *NewErrorsHandler {
	return &NewErrorsHandler{
		ch: ch,
	}
}

// GetNewErrorsAggregated retrieves aggregated error statistics from last N hours
func (h *NewErrorsHandler) GetNewErrorsAggregated(ctx context.Context, params NewErrorsParams) (string, error) {
	// Start span for the entire operation
	ctx, span := startSpan(ctx, "handlers.GetNewErrorsAggregated",
		attribute.String("handler", "new_errors"),
		attribute.String("cluster_guid", params.ClusterGUID),
		attribute.String("infobase_guid", params.InfobaseGUID),
		attribute.Int("hours", params.Hours),
		attribute.Int("limit", params.Limit),
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

	// Set defaults
	if params.Limit <= 0 {
		params.Limit = 100
	}
	if params.Hours <= 0 {
		params.Hours = 48
	}

	// Query aggregated errors from both event_log and tech_log
	// Union of errors from event_log (level = 'Error') and tech_log (level IN ('ERROR', 'EXCP'))
	query := `
		WITH event_log_errors AS (
			SELECT
				cluster_guid,
				cluster_name,
				infobase_guid,
				infobase_name,
				event_presentation AS event_name,
				comment AS error_text,
				sipHash64(concat(event_presentation, comment, COALESCE(metadata_presentation, ''))) AS error_signature,
				count() AS occurrences,
				min(event_time) AS first_seen,
				max(event_time) AS last_seen,
				groupArray(10)(concat(toString(event_time), ': ', event_presentation, ' - ', comment)) AS sample_lines,
				'event_log' AS source
			FROM logs.event_log
			WHERE cluster_guid = ?
			  AND infobase_guid = ?
			  AND level = 'Error'
			  AND event_time >= now() - INTERVAL ? HOUR
			GROUP BY
				cluster_guid,
				cluster_name,
				infobase_guid,
				infobase_name,
				event_presentation,
				comment,
				error_signature
		),
		tech_log_errors AS (
			SELECT
				tl.cluster_guid,
				COALESCE(el.cluster_name, '') AS cluster_name,
				tl.infobase_guid,
				COALESCE(el.infobase_name, '') AS infobase_name,
				tl.name AS event_name,
				arrayElement(tl.property_value, indexOf(tl.property_key, 'Txt')) AS error_text,
				sipHash64(concat(
					tl.name,
					COALESCE(arrayElement(tl.property_value, indexOf(tl.property_key, 'Descr')), ''),
					COALESCE(arrayElement(tl.property_value, indexOf(tl.property_key, 'Txt')), '')
				)) AS error_signature,
				count() AS occurrences,
				min(tl.ts) AS first_seen,
				max(tl.ts) AS last_seen,
				groupArray(10)(tl.raw_line) AS sample_lines,
				'tech_log' AS source
			FROM logs.tech_log tl
			LEFT JOIN (
				SELECT DISTINCT
					cluster_guid,
					infobase_guid,
					argMax(cluster_name, event_time) AS cluster_name,
					argMax(infobase_name, event_time) AS infobase_name
				FROM logs.event_log
				WHERE cluster_guid = ?
				  AND infobase_guid = ?
				GROUP BY cluster_guid, infobase_guid
			) el ON tl.cluster_guid = el.cluster_guid AND tl.infobase_guid = el.infobase_guid
			WHERE tl.cluster_guid = ?
			  AND tl.infobase_guid = ?
			  AND tl.level IN ('ERROR', 'EXCP')
			  AND tl.ts >= now() - INTERVAL ? HOUR
			GROUP BY
				tl.cluster_guid,
				tl.infobase_guid,
				tl.name,
				error_text,
				error_signature,
				el.cluster_name,
				el.infobase_name
		),
		combined_errors AS (
			SELECT * FROM event_log_errors
			UNION ALL
			SELECT * FROM tech_log_errors
		)
		SELECT
			cluster_guid,
			argMax(cluster_name, last_seen) AS cluster_name,
			infobase_guid,
			argMax(infobase_name, last_seen) AS infobase_name,
			event_name,
			error_text,
			error_signature,
			sum(occurrences) AS occurrences,
			min(first_seen) AS first_seen,
			max(last_seen) AS last_seen,
			arrayFlatten(groupArray(sample_lines)) AS sample_lines,
			groupArray(source) AS sources
		FROM combined_errors
		GROUP BY
			cluster_guid,
			infobase_guid,
			event_name,
			error_text,
			error_signature
		ORDER BY occurrences DESC, last_seen DESC
		LIMIT ?
	`

	args := []interface{}{
		params.ClusterGUID,
		params.InfobaseGUID,
		params.Hours,
		params.ClusterGUID,
		params.InfobaseGUID,
		params.ClusterGUID,
		params.InfobaseGUID,
		params.Hours,
		params.Limit,
	}
	
	// Execute query with span
	_, querySpan := startSpan(ctx, "clickhouse.query",
		attribute.String("db.system", "clickhouse"),
		attribute.String("db.name", "logs"),
		attribute.String("db.sql.table", "event_log, tech_log"),
	)
	
	rows, err := h.ch.Query(ctx, query, args...)
	if err != nil {
		endSpanWithError(querySpan, err, "query execution failed")
		endSpanWithError(span, err, "query failed")
		return "", fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	// Collect results
	var results []AggregatedError
	for rows.Next() {
		var record AggregatedError
		var sources []string
		if err := rows.Scan(
			&record.ClusterGUID,
			&record.ClusterName,
			&record.InfobaseGUID,
			&record.InfobaseName,
			&record.EventName,
			&record.ErrorText,
			&record.ErrorSignature,
			&record.Occurrences,
			&record.FirstSeen,
			&record.LastSeen,
			&record.SampleLines,
			&sources,
		); err != nil {
			endSpanWithError(querySpan, err, "scan failed")
			endSpanWithError(span, err, "scan failed")
			return "", fmt.Errorf("scan failed: %w", err)
		}

		// Set source field
		record.Source = sources

		results = append(results, record)
	}
	
	if err := rows.Err(); err != nil {
		endSpanWithError(querySpan, err, "rows error")
		endSpanWithError(span, err, "rows error")
		return "", fmt.Errorf("rows error: %w", err)
	}
	
	querySpan.SetAttributes(attribute.Int("db.rows.count", len(results)))
	endSpanSuccess(querySpan)
	
	// Convert to JSON
	_, jsonSpan := startSpan(ctx, "json.marshal")
	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		endSpanWithError(jsonSpan, err, "json marshal failed")
		endSpanWithError(span, err, "json marshal failed")
		return "", fmt.Errorf("json marshal failed: %w", err)
	}
	jsonSpan.SetAttributes(attribute.Int("json.size_bytes", len(jsonData)))
	endSpanSuccess(jsonSpan)
	
	span.SetAttributes(attribute.Int("result.records_count", len(results)))
	endSpanSuccess(span)
	return string(jsonData), nil
}

// NewErrorsParams defines parameters for get_new_errors_aggregated tool
type NewErrorsParams struct {
	ClusterGUID  string
	InfobaseGUID string
	Hours        int // Time window in hours (default: 48)
	Limit        int // Max errors to return (default: 100)
}

// AggregatedError represents an aggregated error record from both event_log and tech_log
type AggregatedError struct {
	ClusterGUID    string    `json:"cluster_guid"`
	ClusterName    string    `json:"cluster_name"`
	InfobaseGUID   string    `json:"infobase_guid"`
	InfobaseName   string    `json:"infobase_name"`
	EventName      string    `json:"event_name"`
	ErrorText      string    `json:"error_text"`
	ErrorSignature uint64    `json:"error_signature"`
	Occurrences    uint64    `json:"occurrences"`
	FirstSeen      time.Time `json:"first_seen"`
	LastSeen       time.Time `json:"last_seen"`
	SampleLines    []string  `json:"sample_lines"`
	Source         []string  `json:"source"` // Array of sources: ["event_log"], ["tech_log"], or ["event_log", "tech_log"]
}

