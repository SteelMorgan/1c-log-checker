package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/1c-log-checker/internal/clickhouse"
	"github.com/1c-log-checker/internal/mapping"
)

// NewErrorsHandler handles get_new_errors MCP tool
type NewErrorsHandler struct {
	ch         *clickhouse.Client
	clusterMap *mapping.ClusterMap
}

// NewNewErrorsHandler creates a new errors handler
func NewNewErrorsHandler(ch *clickhouse.Client, clusterMap *mapping.ClusterMap) *NewErrorsHandler {
	return &NewErrorsHandler{
		ch:         ch,
		clusterMap: clusterMap,
	}
}

// GetNewErrors retrieves errors unique in last N hours
func (h *NewErrorsHandler) GetNewErrors(ctx context.Context, params NewErrorsParams) (string, error) {
	// Validate GUIDs
	if err := ValidateGUID(params.ClusterGUID, "cluster_guid"); err != nil {
		return "", err
	}
	if err := ValidateGUID(params.InfobaseGUID, "infobase_guid"); err != nil {
		return "", err
	}

	// Set default limit
	if params.Limit <= 0 {
		params.Limit = 100
	}

	// Query for new errors using materialized view
	query := `
		SELECT 
			cluster_guid,
			infobase_guid,
			event_name,
			error_text,
			occurrences,
			first_seen,
			last_seen,
			sample_lines
		FROM logs.mv_new_errors
		WHERE error_date = today()
		  AND cluster_guid = ?
		  AND infobase_guid = ?
		  AND error_signature NOT IN (
			  SELECT DISTINCT error_signature
			  FROM logs.mv_new_errors
			  WHERE error_date = today() - 1
				AND cluster_guid = ?
				AND infobase_guid = ?
		  )
		ORDER BY last_seen DESC
		LIMIT ?
	`
	
	args := []interface{}{
		params.ClusterGUID,
		params.InfobaseGUID,
		params.ClusterGUID,
		params.InfobaseGUID,
		params.Limit,
	}
	
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
		
		// Enrich with cluster/infobase names
		if clusterGUID, ok := record["cluster_guid"].(string); ok {
			record["cluster_name"] = h.clusterMap.GetClusterName(clusterGUID)
		}
		if infobaseGUID, ok := record["infobase_guid"].(string); ok {
			record["infobase_name"] = h.clusterMap.GetInfobaseName(infobaseGUID)
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

// NewErrorsParams defines parameters for get_new_errors tool
type NewErrorsParams struct {
	ClusterGUID  string
	InfobaseGUID string
	Limit        int // Max errors to return
}

