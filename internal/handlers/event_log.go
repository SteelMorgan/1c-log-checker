package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/SteelMorgan/1c-log-checker/internal/clickhouse"
	"github.com/SteelMorgan/1c-log-checker/internal/mapping"
	"github.com/rs/zerolog/log"
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

// getLevelVariants returns both Russian and English variants for a level
// International 1C versions may store levels in English, Russian versions in Russian
// Returns slice with both variants to search for both in database
func getLevelVariants(level string) []string {
	switch level {
	case "Error", "Ошибка":
		return []string{"Ошибка", "Error"}
	case "Warning", "Предупреждение":
		return []string{"Предупреждение", "Warning"}
	case "Information", "Информация":
		return []string{"Информация", "Information"}
	case "Note", "Примечание":
		return []string{"Примечание", "Note"}
	default:
		// If unknown, return as-is (single value)
		return []string{level}
	}
}

// GetEventLog retrieves event log records
func (h *EventLogHandler) GetEventLog(ctx context.Context, params EventLogParams) (string, error) {
	// Force output to stderr
	fmt.Fprintf(os.Stderr, "[STEP 9] GetEventLog: ENTRY - cluster=%s, infobase=%s, level=%s, mode=%s, from=%v, to=%v, limit=%d\n",
		params.ClusterGUID, params.InfobaseGUID, params.Level, params.Mode, params.From, params.To, params.Limit)

	// Log entry point - this MUST appear in logs
	log.Info().
		Str("step", "STEP 9").
		Str("handler", "GetEventLog").
		Str("cluster_guid", params.ClusterGUID).
		Str("infobase_guid", params.InfobaseGUID).
		Str("mode", params.Mode).
		Str("level", params.Level).
		Int("limit", params.Limit).
		Time("from", params.From).
		Time("to", params.To).
		Msg("GetEventLog: ENTRY")
	log.Info().
		Str("handler", "GetEventLog").
		Str("cluster_guid", params.ClusterGUID).
		Str("infobase_guid", params.InfobaseGUID).
		Str("mode", params.Mode).
		Str("level", params.Level).
		Int("limit", params.Limit).
		Time("from", params.From).
		Time("to", params.To).
		Msg("GetEventLog handler called")

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

	// Validate time range only when both bounds are specified
	hasTimeRange := !params.From.IsZero() && !params.To.IsZero()
	if hasTimeRange {
		if err := ValidateTimeRange(params.From.Format(time.RFC3339), params.To.Format(time.RFC3339)); err != nil {
			endSpanWithError(span, err, "validation failed")
			return "", err
		}
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

	// Track whether limit was explicitly provided
	limitWasDefault := params.Limit <= 0
	if limitWasDefault {
		if hasTimeRange {
			params.Limit = 1000
		} else {
			params.Limit = 100
		}
	}

	// Validate limit
	if err := ValidateLimit(params.Limit); err != nil {
		endSpanWithError(span, err, "validation failed")
		return "", err
	}

	// Build query and scan results based on mode
	var jsonData []byte
	// Initialize with empty array JSON to avoid nil/empty string issues
	jsonData = []byte("[]")

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
		`

		// Build args
		args := []interface{}{
			params.ClusterGUID,
			params.InfobaseGUID,
		}

		// Add time range filter only when specified
		if hasTimeRange {
			query += " AND event_time BETWEEN ? AND ?"
			args = append(args, params.From, params.To)
		}

		// Add level filter if specified
		// Search for both Russian and English variants (international 1C versions may use English)
		if params.Level != "" {
			levelVariants := getLevelVariants(params.Level)
			if len(levelVariants) == 1 {
				query += " AND level = ?"
			} else {
				placeholders := ""
				for i := range levelVariants {
					if i > 0 {
						placeholders += ", "
					}
					placeholders += "?"
				}
				query += " AND level IN (" + placeholders + ")"
			}
			for _, variant := range getLevelVariants(params.Level) {
				args = append(args, variant)
			}
		}

		query += " ORDER BY event_time DESC LIMIT ?"
		args = append(args, params.Limit)

		// Execute query with span
		_, querySpan := startSpan(ctx, "clickhouse.query",
			attribute.String("query.mode", "minimal"),
			attribute.String("db.system", "clickhouse"),
			attribute.String("db.name", "logs"),
			attribute.String("db.sql.table", "event_log"),
		)

		// Log query and args for debugging
		log.Debug().
			Str("query", query).
			Interface("args", args).
			Str("cluster_guid", params.ClusterGUID).
			Str("infobase_guid", params.InfobaseGUID).
			Str("level", params.Level).
			Time("from", params.From).
			Time("to", params.To).
			Int("limit", params.Limit).
			Msg("Executing event_log query (minimal mode)")

		fmt.Fprintf(os.Stderr, "[STEP 9.2] Calling h.ch.Query...\n")
		log.Info().Str("step", "STEP 9.2").Msg("Calling h.ch.Query")
		rows, err := h.ch.Query(ctx, query, args...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[STEP 9.2 ERROR] Query failed: %v\n", err)
			log.Error().Str("step", "STEP 9.2").Err(err).Msg("Query failed")
			endSpanWithError(querySpan, err, "query execution failed")
			endSpanWithError(span, err, "query failed")
			return "", fmt.Errorf("query failed: %w", err)
		}
		fmt.Fprintf(os.Stderr, "[STEP 9.2 OK] Query executed successfully\n")
		log.Info().Str("step", "STEP 9.2").Msg("Query executed successfully")
		defer rows.Close()

		// Scan into typed structs
		fmt.Fprintf(os.Stderr, "[STEP 9.3] Scanning rows...\n")
		log.Info().Str("step", "STEP 9.3").Msg("Scanning rows")
		var results []EventLogMinimal
		rowCount := 0
		for rows.Next() {
			rowCount++
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

		// Log results count
		log.Info().
			Int("results_count", len(results)).
			Str("mode", "minimal").
			Msg("Query completed")

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

		// Log JSON result preview
		if len(jsonData) > 0 {
			previewLen := 200
			if len(jsonData) < previewLen {
				previewLen = len(jsonData)
			}
			log.Debug().
				Int("json_size", len(jsonData)).
				Str("json_preview", string(jsonData[:previewLen])).
				Msg("JSON result generated")
		} else {
			log.Warn().Msg("Empty JSON result (no records found)")
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
		`

		// Build args
		args := []interface{}{
			params.ClusterGUID,
			params.InfobaseGUID,
		}

		// Add time range filter only when specified
		if hasTimeRange {
			query += " AND event_time BETWEEN ? AND ?"
			args = append(args, params.From, params.To)
		}

		// Add level filter if specified
		if params.Level != "" {
			levelVariants := getLevelVariants(params.Level)
			if len(levelVariants) == 1 {
				query += " AND level = ?"
			} else {
				placeholders := ""
				for i := range levelVariants {
					if i > 0 {
						placeholders += ", "
					}
					placeholders += "?"
				}
				query += " AND level IN (" + placeholders + ")"
			}
			for _, variant := range getLevelVariants(params.Level) {
				args = append(args, variant)
			}
		}

		query += " ORDER BY event_time DESC LIMIT ?"
		args = append(args, params.Limit)

		// Execute query with span
		_, querySpan := startSpan(ctx, "clickhouse.query",
			attribute.String("query.mode", "full"),
			attribute.String("db.system", "clickhouse"),
			attribute.String("db.name", "logs"),
			attribute.String("db.sql.table", "event_log"),
		)

		// Log query and args for debugging
		log.Debug().
			Str("query", query).
			Interface("args", args).
			Str("cluster_guid", params.ClusterGUID).
			Str("infobase_guid", params.InfobaseGUID).
			Str("level", params.Level).
			Time("from", params.From).
			Time("to", params.To).
			Int("limit", params.Limit).
			Msg("Executing event_log query (full mode)")

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

		// Log results count
		log.Info().
			Int("results_count", len(results)).
			Str("mode", "full").
			Msg("Query completed")

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

		// Log JSON result preview
		if len(jsonData) > 0 {
			previewLen := 200
			if len(jsonData) < previewLen {
				previewLen = len(jsonData)
			}
			log.Debug().
				Int("json_size", len(jsonData)).
				Str("json_preview", string(jsonData[:previewLen])).
				Msg("JSON result generated")
		} else {
			log.Warn().Msg("Empty JSON result (no records found)")
		}

		span.SetAttributes(attribute.Int("result.records_count", len(results)))
	}

	endSpanSuccess(span)

	fmt.Fprintf(os.Stderr, "[STEP 9.5] Finalizing result...\n")
	log.Info().Str("step", "STEP 9.5").Msg("Finalizing result")
	// Ensure we always return valid JSON (empty array if no results)
	if len(jsonData) == 0 || string(jsonData) == "null" {
		fmt.Fprintf(os.Stderr, "[STEP 9.5 WARN] jsonData is empty or null, setting to []\n")
		log.Warn().Str("step", "STEP 9.5").Str("jsonData", string(jsonData)).Msg("jsonData is empty or null, setting to []")
		jsonData = []byte("[]")
		log.Warn().Msg("jsonData was empty or null, returning empty array")
	}

	resultStr := string(jsonData)
	fmt.Fprintf(os.Stderr, "[STEP 9.5 OK] Final result: len=%d, preview=%.100s\n", len(resultStr), resultStr)
	log.Info().
		Str("step", "STEP 9.5").
		Int("result_len", len(resultStr)).
		Str("result_preview", func() string {
			if len(resultStr) > 100 {
				return resultStr[:100]
			}
			return resultStr
		}()).
		Msg("Final result")
	log.Info().
		Int("json_length", len(jsonData)).
		Str("json_preview", resultStr[:min(len(resultStr), 100)]).
		Msg("GetEventLog returning result")

	fmt.Fprintf(os.Stderr, "[STEP 9 OK] GetEventLog returning successfully\n")
	log.Info().Str("step", "STEP 9").Msg("GetEventLog returning successfully")

	// Add warning when limit was not explicitly provided
	if limitWasDefault && !hasTimeRange {
		return fmt.Sprintf("⚠️ Параметр limit не был передан — по умолчанию выдано до %d записей по запрошенному фильтру.\n\n%s", params.Limit, resultStr), nil
	}

	return resultStr, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// EventLogParams defines parameters for get_event_log tool
type EventLogParams struct {
	ClusterGUID  string
	InfobaseGUID string
	From         time.Time
	To           time.Time
	Level        string // Optional filter: Error/Ошибка, Warning/Предупреждение, Information/Информация, Note/Примечание (searches for both Russian and English variants)
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
	EventTime               time.Time `json:"event_time"`
	EventDate               time.Time `json:"event_date"`
	ClusterGUID             string    `json:"cluster_guid"`
	ClusterName             string    `json:"cluster_name"`
	InfobaseGUID            string    `json:"infobase_guid"`
	InfobaseName            string    `json:"infobase_name"`
	Level                   string    `json:"level"`
	Event                   string    `json:"event"`
	EventPresentation       string    `json:"event_presentation"`
	UserName                string    `json:"user_name"`
	UserID                  string    `json:"user_id"`
	Computer                string    `json:"computer"`
	Application             string    `json:"application"`
	ApplicationPresentation string    `json:"application_presentation"`
	SessionID               uint64    `json:"session_id"`
	ConnectionID            uint64    `json:"connection_id"`
	TransactionStatus       string    `json:"transaction_status"`
	TransactionID           string    `json:"transaction_id"`
	DataSeparation          string    `json:"data_separation"`
	MetadataName            string    `json:"metadata_name"`
	MetadataPresentation    string    `json:"metadata_presentation"`
	Comment                 string    `json:"comment"`
	Data                    string    `json:"data"`
	DataPresentation        string    `json:"data_presentation"`
	Server                  string    `json:"server"`
	PrimaryPort             uint16    `json:"primary_port"`
	SecondaryPort           uint16    `json:"secondary_port"`
}
