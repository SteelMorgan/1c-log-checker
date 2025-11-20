package service

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/rs/zerolog/log"
)

// NewErrorsWorker periodically updates mv_new_errors table with aggregated errors
// from both event_log and tech_log tables
type NewErrorsWorker struct {
	conn     clickhouse.Conn
	interval time.Duration
	hours    int // Time window for error analysis (default: 48 hours)
}

// NewNewErrorsWorker creates a new errors aggregation worker
func NewNewErrorsWorker(conn clickhouse.Conn, interval time.Duration, hours int) *NewErrorsWorker {
	if hours <= 0 {
		hours = 48
	}
	return &NewErrorsWorker{
		conn:     conn,
		interval: interval,
		hours:    hours,
	}
}

// Start starts the worker in a background goroutine
func (w *NewErrorsWorker) Start(ctx context.Context) {
	log.Info().
		Dur("interval", w.interval).
		Int("hours", w.hours).
		Msg("Starting new errors aggregation worker")

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Run immediately on start
	w.updateErrors(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("New errors worker context cancelled")
			return
		case <-ticker.C:
			w.updateErrors(ctx)
		}
	}
}

// updateErrors aggregates errors from both event_log and tech_log and updates mv_new_errors table
func (w *NewErrorsWorker) updateErrors(ctx context.Context) {
	startTime := time.Now()
	log.Debug().Msg("Starting new errors aggregation")

	// First, completely clear the table to store only one snapshot
	// Using TRUNCATE TABLE for complete cleanup
	truncateQuery := `TRUNCATE TABLE IF EXISTS logs.mv_new_errors`
	
	log.Debug().Msg("Clearing mv_new_errors table before inserting new snapshot")
	err := w.conn.Exec(ctx, truncateQuery)
	if err != nil {
		log.Warn().
			Err(err).
			Msg("Failed to clear table (continuing anyway)")
		// Continue with insert even if truncate failed
	} else {
		log.Debug().Msg("Successfully cleared mv_new_errors table")
	}

	// Query to aggregate errors from both sources
	// This matches the logic from handlers/new_errors.go
	query := `
		INSERT INTO logs.mv_new_errors (
			cluster_guid,
			cluster_name,
			infobase_guid,
			infobase_name,
			event_name,
			error_text,
			error_signature,
			occurrences,
			first_seen,
			last_seen,
			sample_lines,
			sources,
			updated_at
		)
		WITH event_log_errors AS (
			SELECT
				cluster_guid,
				cluster_name,
				infobase_guid,
				infobase_name,
				event_presentation AS event_name,
				comment AS error_text,
				sipHash64(concat(event_presentation, comment, COALESCE(metadata_presentation, ''))) AS error_signature,
				event_time AS last_seen,
				concat(toString(event_time), ': ', event_presentation, ' - ', comment) AS sample_line,
				'event_log' AS source
			FROM logs.event_log
			WHERE level = 'Error'
			  AND event_time >= now() - INTERVAL ? HOUR
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
				tl.ts AS last_seen,
				tl.raw_line AS sample_line,
				'tech_log' AS source
			FROM logs.tech_log tl
			LEFT JOIN (
				SELECT DISTINCT
					cluster_guid,
					infobase_guid,
					argMax(cluster_name, event_time) AS cluster_name,
					argMax(infobase_name, event_time) AS infobase_name
				FROM logs.event_log
				GROUP BY cluster_guid, infobase_guid
			) el ON tl.cluster_guid = el.cluster_guid AND tl.infobase_guid = el.infobase_guid
			WHERE tl.level IN ('ERROR', 'EXCP')
			  AND tl.ts >= now() - INTERVAL ? HOUR
		),
		combined_errors AS (
			SELECT * FROM event_log_errors
			UNION ALL
			SELECT * FROM tech_log_errors
		)
		SELECT
			cluster_guid,
			cluster_name,
			infobase_guid,
			infobase_name,
			event_name,
			error_text,
			error_signature,
			1 AS occurrences,
			last_seen AS first_seen,
			last_seen,
			[sample_line] AS sample_lines,
			[source] AS sources,
			now() AS updated_at
		FROM combined_errors
	`

	err = w.conn.Exec(ctx, query, w.hours, w.hours)
	if err != nil {
		log.Error().
			Err(err).
			Dur("duration", time.Since(startTime)).
			Msg("Failed to update new errors aggregation")
		return
	}

	duration := time.Since(startTime)
	log.Info().
		Dur("duration", duration).
		Msg("New errors aggregation completed successfully")
}
