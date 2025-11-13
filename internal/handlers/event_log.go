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
	// Build query based on mode
	var query string
	if params.Mode == "minimal" {
		query = `
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
	} else {
		// Full mode: all fields
		query = `
			SELECT *
			FROM logs.event_log
			WHERE cluster_guid = ? AND infobase_guid = ?
			  AND event_time BETWEEN ? AND ?
		`
	}
	
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
	
	// Collect results
	var results []map[string]interface{}
	for rows.Next() {
		var record map[string]interface{}
		if err := rows.ScanStruct(&record); err != nil {
			return "", fmt.Errorf("scan failed: %w", err)
		}
		results = append(results, record)
	}
	
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("rows error: %w", err)
	}
	
	// Convert to JSON
	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", fmt.Errorf("json marshal failed: %w", err)
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

