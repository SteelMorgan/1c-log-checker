package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/1c-log-checker/internal/clickhouse"
	"github.com/1c-log-checker/internal/mapping"
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
	// Validate GUIDs
	if err := ValidateGUID(params.ClusterGUID, "cluster_guid"); err != nil {
		return "", err
	}
	if err := ValidateGUID(params.InfobaseGUID, "infobase_guid"); err != nil {
		return "", err
	}

	// Validate time range
	if err := ValidateTimeRange(params.From.Format(time.RFC3339), params.To.Format(time.RFC3339)); err != nil {
		return "", err
	}

	// Validate mode
	if err := ValidateMode(params.Mode); err != nil {
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

		// Execute query
		rows, err := h.ch.Query(ctx, query, args...)
		if err != nil {
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
				return "", fmt.Errorf("scan failed: %w", err)
			}
			results = append(results, record)
		}

		if err := rows.Err(); err != nil {
			return "", fmt.Errorf("rows error: %w", err)
		}

		// Convert to JSON
		jsonData, err = json.MarshalIndent(results, "", "  ")
		if err != nil {
			return "", fmt.Errorf("json marshal failed: %w", err)
		}

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

		// Execute query
		rows, err := h.ch.Query(ctx, query, args...)
		if err != nil {
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
				return "", fmt.Errorf("scan failed: %w", err)
			}
			results = append(results, record)
		}

		if err := rows.Err(); err != nil {
			return "", fmt.Errorf("rows error: %w", err)
		}

		// Convert to JSON
		jsonData, err = json.MarshalIndent(results, "", "  ")
		if err != nil {
			return "", fmt.Errorf("json marshal failed: %w", err)
		}
	}

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

