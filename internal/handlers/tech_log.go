package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/1c-log-checker/internal/clickhouse"
	"github.com/1c-log-checker/internal/mapping"
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
	
	// Execute query
	rows, err := h.ch.Query(ctx, query, args...)
	if err != nil {
		return "", fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var jsonData []byte

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

