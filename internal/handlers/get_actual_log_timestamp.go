package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/1c-log-checker/internal/clickhouse"
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
	// Validate base_id (infobase_guid)
	if err := ValidateGUID(baseID, "base_id"); err != nil {
		return "", err
	}

	// Query for maximum timestamp
	query := `
		SELECT MAX(ts) as max_timestamp
		FROM logs.tech_log
		WHERE infobase_guid = ?
	`

	// Execute query
	rows, err := h.ch.Query(ctx, query, baseID)
	if err != nil {
		return "", fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	// Read result - use nullable time.Time to handle NULL values
	var maxTimestamp *time.Time
	if rows.Next() {
		if err := rows.Scan(&maxTimestamp); err != nil {
			return "", fmt.Errorf("scan failed: %w", err)
		}
	}

	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("rows error: %w", err)
	}

	// Build response
	response := map[string]interface{}{
		"base_id": baseID,
	}

	// Check if result is NULL or empty
	if maxTimestamp == nil {
		// No records found
		response["max_timestamp"] = nil
		response["has_data"] = false
	} else {
		// Format timestamp
		response["max_timestamp"] = maxTimestamp.Format(time.RFC3339Nano)
		response["has_data"] = true
	}

	// Convert to JSON
	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return "", fmt.Errorf("json marshal failed: %w", err)
	}

	return string(jsonData), nil
}

