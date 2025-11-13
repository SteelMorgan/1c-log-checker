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

// NewTechLogHandler creates a new tech log handler
func NewTechLogHandler(ch *clickhouse.Client, clusterMap *mapping.ClusterMap) *TechLogHandler {
	return &TechLogHandler{
		ch:         ch,
		clusterMap: clusterMap,
	}
}

// GetTechLog retrieves tech log records
func (h *TechLogHandler) GetTechLog(ctx context.Context, params TechLogParams) (string, error) {
	// Build query
	var query string
	if params.Mode == "minimal" {
		query = `
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
				property_value
			FROM logs.tech_log
			WHERE cluster_guid = ? AND infobase_guid = ?
			  AND ts BETWEEN ? AND ?
		`
	} else {
		// Full mode: all fields including raw_line
		query = `
			SELECT *
			FROM logs.tech_log
			WHERE cluster_guid = ? AND infobase_guid = ?
			  AND ts BETWEEN ? AND ?
		`
	}
	
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

