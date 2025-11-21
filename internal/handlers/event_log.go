package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/SteelMorgan/1c-log-checker/internal/clickhouse"
	"github.com/SteelMorgan/1c-log-checker/internal/mapping"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// EventLogHandler handles get_event_log MCP tool
type EventLogHandler struct {
	ch         *clickhouse.Client
	clusterMap *mapping.ClusterMap
}

// NewEventLogHandler creates a new event log handler
func NewEventLogHandler(ch *clickhouse.Client, clusterMap *mapping.ClusterMap) *EventLogHandler {
	return &EventLogHandler{
		ch:         ch,
		clusterMap: clusterMap,
	}
}

// GetEventLog retrieves event log records
func (h *EventLogHandler) GetEventLog(ctx context.Context, params EventLogParams) (string, error) {
	// Start span for the entire operation
	ctx, span := startSpan(ctx, "handlers.GetEventLog",
		attribute.String("handler", "event_log"),
		attribute.String("cluster_guid", params.ClusterGUID),
		attribute.String("infobase_guid", params.InfobaseGUID),
		attribute.String("mode", params.Mode),
		attribute.Int("limit", params.Limit),
		attribute.String("level", params.Level),
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

	// Set default mode if empty
	if params.Mode == "" {
		params.Mode = "minimal"
	}

	// Set default limit if not specified
	if params.Limit <= 0 {
		params.Limit = 1000
	}

	// Build query and scan results based on mode
	var jsonData []byte

	if params.Mode == "minimal" {
		// Minimal mode query
		query := `
			SELECT
				event_time,
				level,
				event_presentation,
				user_name,
				comment,
				metadata_presentation
			FROM logs.event_log
			WHERE cluster_guid = ? AND infobase_guid = ?
			  AND event_time BETWEEN ? AND ?
		`

		// Add level filter if specified
		if params.Level != "" {
			query += " AND level = ?"
		}

		query += " ORDER BY event_time DESC LIMIT ?"

		// Build args
		args := []interface{}{
			params.ClusterGUID,
			params.InfobaseGUID,
			params.From,
			params.To,
		}
		if params.Level != "" {
			args = append(args, params.Level)
		}
		args = append(args, params.Limit)

		// Execute query with span
		_, querySpan := startSpan(ctx, "clickhouse.query",
			attribute.String("query.mode", "minimal"),
			attribute.String("db.system", "clickhouse"),
			attribute.String("db.name", "logs"),
			attribute.String("db.sql.table", "event_log"),
		)
		
		rows, err := h.ch.Query(ctx, query, args...)
		if err != nil {
			endSpanWithError(querySpan, err, "query execution failed")
			endSpanWithError(span, err, "query failed")
			return "", fmt.Errorf("query failed: %w", err)
		}
		defer rows.Close()

		// Scan into typed structs
		var results []EventLogMinimal
		for rows.Next() {
			var record EventLogMinimal
			if err := rows.Scan(
				&record.EventTime,
				&record.Level,
				&record.EventPresentation,
				&record.UserName,
				&record.Comment,
				&record.MetadataPresentation,
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

		querySpan.SetAttributes(attribute.Int("db.rows.count", len(results)))
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
		// Full mode query
		query := `
			SELECT
				event_time,
				event_date,
				cluster_guid,
				cluster_name,
				infobase_guid,
				infobase_name,
				level,
				event,
				event_presentation,
				user_name,
				user_id,
				computer,
				application,
				application_presentation,
				session_id,
				connection_id,
				transaction_status,
				transaction_id,
				data_separation,
				metadata_name,
				metadata_presentation,
				comment,
				data,
				data_presentation,
				server,
				primary_port,
				secondary_port
			FROM logs.event_log
			WHERE cluster_guid = ? AND infobase_guid = ?
			  AND event_time BETWEEN ? AND ?
		`

		// Add level filter if specified
		if params.Level != "" {
			query += " AND level = ?"
		}

		query += " ORDER BY event_time DESC LIMIT ?"

		// Build args
		args := []interface{}{
			params.ClusterGUID,
			params.InfobaseGUID,
			params.From,
			params.To,
		}
		if params.Level != "" {
			args = append(args, params.Level)
		}
		args = append(args, params.Limit)

		// Execute query with span
		_, querySpan := startSpan(ctx, "clickhouse.query",
			attribute.String("query.mode", "full"),
			attribute.String("db.system", "clickhouse"),
			attribute.String("db.name", "logs"),
			attribute.String("db.sql.table", "event_log"),
		)
		
		rows, err := h.ch.Query(ctx, query, args...)
		if err != nil {
			endSpanWithError(querySpan, err, "query execution failed")
			endSpanWithError(span, err, "query failed")
			return "", fmt.Errorf("query failed: %w", err)
		}
		defer rows.Close()

		// Scan into typed structs
		var results []EventLogFull
		for rows.Next() {
			var record EventLogFull
			if err := rows.Scan(
				&record.EventTime,
				&record.EventDate,
				&record.ClusterGUID,
				&record.ClusterName,
				&record.InfobaseGUID,
				&record.InfobaseName,
				&record.Level,
				&record.Event,
				&record.EventPresentation,
				&record.UserName,
				&record.UserID,
				&record.Computer,
				&record.Application,
				&record.ApplicationPresentation,
				&record.SessionID,
				&record.ConnectionID,
				&record.TransactionStatus,
				&record.TransactionID,
				&record.DataSeparation,
				&record.MetadataName,
				&record.MetadataPresentation,
				&record.Comment,
				&record.Data,
				&record.DataPresentation,
				&record.Server,
				&record.PrimaryPort,
				&record.SecondaryPort,
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

		querySpan.SetAttributes(attribute.Int("db.rows.count", len(results)))
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
		
		span.SetAttributes(attribute.Int("result.records_count", len(results)))
	}

	endSpanSuccess(span)
	return string(jsonData), nil
}

// EventLogParams defines parameters for get_event_log tool
type EventLogParams struct {
	ClusterGUID  string
	InfobaseGUID string
	From         time.Time
	To           time.Time
	Level        string // Optional filter: Error, Warning, Information, Note
	Mode         string // minimal or full
	Limit        int    // Max records to return
}

// EventLogMinimal represents minimal mode output
type EventLogMinimal struct {
	EventTime            time.Time `json:"event_time"`
	Level                string    `json:"level"`
	EventPresentation    string    `json:"event_presentation"`
	UserName             string    `json:"user_name"`
	Comment              string    `json:"comment"`
	MetadataPresentation string    `json:"metadata_presentation"`
}

// EventLogFull represents full mode output
type EventLogFull struct {
	EventTime              time.Time `json:"event_time"`
	EventDate              time.Time `json:"event_date"`
	ClusterGUID            string    `json:"cluster_guid"`
	ClusterName            string    `json:"cluster_name"`
	InfobaseGUID           string    `json:"infobase_guid"`
	InfobaseName           string    `json:"infobase_name"`
	Level                  string    `json:"level"`
	Event                  string    `json:"event"`
	EventPresentation      string    `json:"event_presentation"`
	UserName               string    `json:"user_name"`
	UserID                 string    `json:"user_id"`
	Computer               string    `json:"computer"`
	Application            string    `json:"application"`
	ApplicationPresentation string   `json:"application_presentation"`
	SessionID              uint64    `json:"session_id"`
	ConnectionID           uint64    `json:"connection_id"`
	TransactionStatus      string    `json:"transaction_status"`
	TransactionID          string    `json:"transaction_id"`
	DataSeparation         string    `json:"data_separation"`
	MetadataName           string    `json:"metadata_name"`
	MetadataPresentation   string    `json:"metadata_presentation"`
	Comment                string    `json:"comment"`
	Data                   string    `json:"data"`
	DataPresentation       string    `json:"data_presentation"`
	Server                 string    `json:"server"`
	PrimaryPort            uint16    `json:"primary_port"`
	SecondaryPort          uint16    `json:"secondary_port"`
}

