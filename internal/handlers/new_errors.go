package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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

// GetNewErrorsAggregated retrieves aggregated error statistics from last N hours
func (h *NewErrorsHandler) GetNewErrorsAggregated(ctx context.Context, params NewErrorsParams) (string, error) {
	// Validate GUIDs
	if err := ValidateGUID(params.ClusterGUID, "cluster_guid"); err != nil {
		return "", err
	}
	if err := ValidateGUID(params.InfobaseGUID, "infobase_guid"); err != nil {
		return "", err
	}

	// Set defaults
	if params.Limit <= 0 {
		params.Limit = 100
	}
	if params.Hours <= 0 {
		params.Hours = 48
	}

	// Query aggregated errors from last N hours
	query := `
		SELECT
			cluster_guid,
			infobase_guid,
			event_name,
			error_text,
			error_signature,
			occurrences,
			first_seen,
			last_seen,
			sample_lines
		FROM logs.mv_new_errors
		WHERE cluster_guid = ?
		  AND infobase_guid = ?
		  AND last_seen >= now() - INTERVAL ? HOUR
		ORDER BY occurrences DESC, last_seen DESC
		LIMIT ?
	`

	args := []interface{}{
		params.ClusterGUID,
		params.InfobaseGUID,
		params.Hours,
		params.Limit,
	}
	
	// Execute query
	rows, err := h.ch.Query(ctx, query, args...)
	if err != nil {
		return "", fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	// Collect results
	var results []AggregatedError
	for rows.Next() {
		var record AggregatedError
		if err := rows.Scan(
			&record.ClusterGUID,
			&record.InfobaseGUID,
			&record.EventName,
			&record.ErrorText,
			&record.ErrorSignature,
			&record.Occurrences,
			&record.FirstSeen,
			&record.LastSeen,
			&record.SampleLines,
		); err != nil {
			return "", fmt.Errorf("scan failed: %w", err)
		}

		// Enrich with cluster/infobase names
		record.ClusterName = h.clusterMap.GetClusterName(record.ClusterGUID)
		record.InfobaseName = h.clusterMap.GetInfobaseName(record.InfobaseGUID)

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

// NewErrorsParams defines parameters for get_new_errors_aggregated tool
type NewErrorsParams struct {
	ClusterGUID  string
	InfobaseGUID string
	Hours        int // Time window in hours (default: 48)
	Limit        int // Max errors to return (default: 100)
}

// AggregatedError represents an aggregated error record
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
}

