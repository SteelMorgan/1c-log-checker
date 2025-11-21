package writer

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/SteelMorgan/1c-log-checker/internal/domain"
	"github.com/SteelMorgan/1c-log-checker/internal/normalizer"
	"github.com/SteelMorgan/1c-log-checker/internal/retry"
	"github.com/rs/zerolog/log"
)

// ClickHouse DateTime64 valid range: 1925-01-01 to 2283-11-11
var (
	minClickHouseDateTime = time.Date(1925, 1, 1, 0, 0, 0, 0, time.UTC)
	maxClickHouseDateTime = time.Date(2283, 11, 11, 23, 59, 59, 999999999, time.UTC)
)

// ensureValidDateTime ensures the time value is within ClickHouse DateTime64 range
// Returns the input time if valid, or minClickHouseDateTime if out of range or zero
func ensureValidDateTime(t time.Time) time.Time {
	if t.IsZero() || t.Before(minClickHouseDateTime) || t.After(maxClickHouseDateTime) {
		return minClickHouseDateTime
	}
	return t
}

// ClickHouseWriter writes records to ClickHouse in batches
type ClickHouseWriter struct {
	conn     clickhouse.Conn
	cfg      BatchConfig
	retryCfg retry.Config
	
	eventLogBatch []*domain.EventLogRecord
	techLogBatch  []*domain.TechLogRecord
	
	lastFlush time.Time
	
	// Mutex to protect batch operations (append, flush)
	batchMutex sync.Mutex
	
	// Accumulated deduplication and writing metrics (for reporting)
	deduplicationTimeAccumulator time.Duration
	writingTimeAccumulator      time.Duration
	deduplicationRecordsCount   uint64
	writingRecordsCount         uint64
	metricsMutex                sync.Mutex
	
	// Error normalization (synchronous, during batch preparation)
	normalizer        *normalizer.CommentNormalizer
	techLogNormalizer *normalizer.TechLogNormalizer
}

// NewClickHouseWriter creates a new ClickHouse batch writer with default retry config
func NewClickHouseWriter(conn clickhouse.Conn, cfg BatchConfig) *ClickHouseWriter {
	return NewClickHouseWriterWithRetry(conn, cfg, retry.DefaultConfig())
}

// NewClickHouseWriterWithRetry creates a new ClickHouse batch writer with custom retry config
func NewClickHouseWriterWithRetry(conn clickhouse.Conn, cfg BatchConfig, retryCfg retry.Config) *ClickHouseWriter {
	writer := &ClickHouseWriter{
		conn:          conn,
		cfg:           cfg,
		retryCfg:      retryCfg,
		eventLogBatch: make([]*domain.EventLogRecord, 0, cfg.MaxSize),
		techLogBatch:  make([]*domain.TechLogRecord, 0, cfg.MaxSize),
		lastFlush:     time.Now(),
		deduplicationTimeAccumulator: 0,
		writingTimeAccumulator:       0,
		deduplicationRecordsCount:   0,
		writingRecordsCount:         0,
		normalizer:           normalizer.NewCommentNormalizer(),
		techLogNormalizer:    normalizer.NewTechLogNormalizer(),
	}
	
	return writer
}

// WriteEventLog adds an event log record to the batch
func (w *ClickHouseWriter) WriteEventLog(ctx context.Context, record *domain.EventLogRecord) error {
	// CRITICAL: Make a copy of the record to avoid race conditions
	recordCopy := *record
	
	w.batchMutex.Lock()
	w.eventLogBatch = append(w.eventLogBatch, &recordCopy)
	batchSize := len(w.eventLogBatch)
	shouldFlush := batchSize >= w.cfg.MaxSize || time.Since(w.lastFlush).Milliseconds() >= w.cfg.FlushTimeout
	
	if shouldFlush {
		// CRITICAL: Create a snapshot of the batch to avoid modification during flush
		batchSnapshot := make([]*domain.EventLogRecord, batchSize)
		copy(batchSnapshot, w.eventLogBatch)
		
		// Clear the original batch before flushing to prevent new records from being added
		w.eventLogBatch = w.eventLogBatch[:0]
		w.batchMutex.Unlock()
		
		// Flush the snapshot (outside of lock to avoid blocking writes)
		return w.flushEventLogSnapshot(ctx, batchSnapshot)
	}
	
	w.batchMutex.Unlock()
	return nil
}

// WriteTechLog adds a tech log record to the batch
func (w *ClickHouseWriter) WriteTechLog(ctx context.Context, record *domain.TechLogRecord) error {
	// CRITICAL: Make a copy of the record to avoid race conditions
	recordCopy := *record

	w.batchMutex.Lock()
	w.techLogBatch = append(w.techLogBatch, &recordCopy)
	batchSize := len(w.techLogBatch)
	shouldFlush := batchSize >= w.cfg.MaxSize || time.Since(w.lastFlush).Milliseconds() >= w.cfg.FlushTimeout

	if shouldFlush {
		// CRITICAL: Create a snapshot of the batch to avoid modification during flush
		batchSnapshot := make([]*domain.TechLogRecord, batchSize)
		copy(batchSnapshot, w.techLogBatch)

		// Clear the original batch before flushing to prevent new records from being added
		w.techLogBatch = w.techLogBatch[:0]
		w.batchMutex.Unlock()

		// Flush the snapshot (outside of lock to avoid blocking writes)
		return w.flushTechLogSnapshot(ctx, batchSnapshot)
	}

	w.batchMutex.Unlock()
	return nil
}

// Flush forces writing all pending records
func (w *ClickHouseWriter) Flush(ctx context.Context) error {
	if err := w.flushEventLog(ctx); err != nil {
		return err
	}
	return w.flushTechLog(ctx)
}

// WriteParserMetrics writes parser performance metrics to ClickHouse
// CRITICAL: Deletes old record for the same file before inserting new one to ensure only one record per file
func (w *ClickHouseWriter) WriteParserMetrics(ctx context.Context, metrics *domain.ParserMetrics) error {
	// Enrich metrics with accumulated values from writer if not already set
	// This allows metrics from reader (which doesn't have deduplication/writing times) to be enriched
	// Works for both event_log and tech_log parsers
	w.metricsMutex.Lock()
	
	// Save accumulated metrics to local variables before unlocking
	writingRecordsCount := w.writingRecordsCount
	writingTimeAccumulator := w.writingTimeAccumulator
	deduplicationRecordsCount := w.deduplicationRecordsCount
	deduplicationTimeAccumulator := w.deduplicationTimeAccumulator
	
	w.metricsMutex.Unlock()
	
	// Enrich DeduplicationTimeMs if not set and deduplication is enabled
	// CRITICAL: Use current accumulated metrics at the time of WriteParserMetrics call
	// This ensures we use metrics that have been accumulated up to this point
	if metrics.DeduplicationTimeMs == 0 && w.cfg.EnableDeduplication && deduplicationRecordsCount > 0 {
		// Calculate average deduplication time per record and scale to current batch
		// CRITICAL: Use nanoseconds to avoid precision loss when converting to milliseconds
		// Calculate: (total_nanoseconds / records_count) * records_parsed / 1_000_000
		avgDedupTimePerRecordNs := deduplicationTimeAccumulator.Nanoseconds() / int64(deduplicationRecordsCount)
		calculatedDedupTimeNs := avgDedupTimePerRecordNs * int64(metrics.RecordsParsed)
		calculatedDedupTime := uint64(calculatedDedupTimeNs / 1_000_000) // Convert nanoseconds to milliseconds

		log.Info().
			Str("file_path", metrics.FilePath).
			Uint64("deduplication_records_count", deduplicationRecordsCount).
			Dur("deduplication_time_accumulator", deduplicationTimeAccumulator).
			Int64("avg_dedup_time_per_record_ns", avgDedupTimePerRecordNs).
			Uint64("records_parsed", metrics.RecordsParsed).
			Int64("calculated_dedup_time_ns", calculatedDedupTimeNs).
			Uint64("calculated_dedup_time_ms", calculatedDedupTime).
			Msg("Enriching DeduplicationTimeMs from accumulated metrics")
		
		if calculatedDedupTime == 0 && metrics.RecordsParsed > 0 {
			// For very small deduplication times, use minimum 1ms to avoid zero
			metrics.DeduplicationTimeMs = 1
		} else {
			metrics.DeduplicationTimeMs = calculatedDedupTime
		}
	}
	
	// Enrich WritingTimeMs if not set and we have accumulated writing metrics
	// CRITICAL: Use current accumulated metrics at the time of WriteParserMetrics call
	// This ensures we use metrics that have been accumulated up to this point
	if metrics.WritingTimeMs == 0 {
		if writingRecordsCount > 0 {
			// Calculate average writing time per record and scale to current batch
			// CRITICAL: Use nanoseconds to avoid precision loss when converting to milliseconds
			// Calculate: (total_nanoseconds / records_count) * records_parsed / 1_000_000
			avgWritingTimePerRecordNs := writingTimeAccumulator.Nanoseconds() / int64(writingRecordsCount)
			calculatedWritingTimeNs := avgWritingTimePerRecordNs * int64(metrics.RecordsParsed)
			calculatedWritingTime := uint64(calculatedWritingTimeNs / 1_000_000) // Convert nanoseconds to milliseconds
			
			log.Info().
				Str("file_path", metrics.FilePath).
				Uint64("writing_records_count", writingRecordsCount).
				Dur("writing_time_accumulator", writingTimeAccumulator).
				Int64("avg_writing_time_per_record_ns", avgWritingTimePerRecordNs).
				Uint64("records_parsed", metrics.RecordsParsed).
				Int64("calculated_writing_time_ns", calculatedWritingTimeNs).
				Uint64("calculated_writing_time_ms", calculatedWritingTime).
				Msg("Enriching WritingTimeMs from accumulated metrics")
			if calculatedWritingTime == 0 && metrics.RecordsParsed > 0 {
				// For very small writing times, use minimum 1ms to avoid zero
				// This is acceptable for small files, but should not happen for large files
				if metrics.RecordsParsed < 1000 {
					metrics.WritingTimeMs = 1
					log.Debug().
						Str("file_path", metrics.FilePath).
						Uint64("records_parsed", metrics.RecordsParsed).
						Msg("Small file: calculated writing time is 0, using fallback 1ms")
				} else {
					// For large files, this is suspicious - log warning but use 1ms as fallback
					log.Warn().
						Str("file_path", metrics.FilePath).
						Uint64("records_parsed", metrics.RecordsParsed).
						Uint64("writing_records_count", writingRecordsCount).
						Int64("calculated_writing_time_ns", calculatedWritingTimeNs).
						Msg("WARNING: Large file but calculated writing time is 0 - using fallback 1ms")
					metrics.WritingTimeMs = 1
				}
			} else {
				metrics.WritingTimeMs = calculatedWritingTime
			}
		} else {
			// CRITICAL: No accumulated metrics - this means batches haven't been written yet
			// This is a problem - we should have metrics by the time WriteParserMetrics is called
			// For small files (< 1000 records), use fallback 1ms (acceptable)
			// For large files (>= 1000 records), keep 0 to indicate missing data (critical issue)
			if metrics.RecordsParsed < 1000 {
				// Small files: acceptable to use fallback
				metrics.WritingTimeMs = 1
				log.Debug().
					Str("file_path", metrics.FilePath).
					Uint64("records_parsed", metrics.RecordsParsed).
					Msg("Small file: using fallback 1ms for writing_time_ms (acceptable)")
			} else {
				// Large files: CRITICAL - should have real metrics
				log.Warn().
					Str("file_path", metrics.FilePath).
					Uint64("records_parsed", metrics.RecordsParsed).
					Uint64("parsing_time_ms", metrics.ParsingTimeMs).
					Uint64("writing_records_count", writingRecordsCount).
					Dur("writing_time_accumulator", writingTimeAccumulator).
					Msg("CRITICAL: Large file but no accumulated writing metrics found - batches may not have been written yet. WritingTimeMs will be 0.")
				// DO NOT set fallback value - keep it 0 to indicate missing data
			}
		}
	}
	
	// Enrich FileReadingTimeMs if not set or if it's a minimal placeholder value (1ms)
	// Calculate from ParsingTimeMs and RecordParsingTimeMs if available
	// CRITICAL: Always enrich if FileReadingTimeMs is 0 or 1 (minimal placeholder)
	// This ensures we have a proper value even when reading and parsing times are equal (streaming mode)
	if (metrics.FileReadingTimeMs == 0 || metrics.FileReadingTimeMs == 1) && metrics.ParsingTimeMs > 0 {
		if metrics.RecordParsingTimeMs > 0 && metrics.ParsingTimeMs > metrics.RecordParsingTimeMs {
			// FileReadingTime = TotalParsingTime - RecordParsingTime
			metrics.FileReadingTimeMs = metrics.ParsingTimeMs - metrics.RecordParsingTimeMs
		} else {
			// In streaming mode, reading and parsing are concurrent, so they're almost equal
			// Estimate file reading time as a percentage of total time (I/O overhead)
			// Use conservative estimate: 15% of parsing time for I/O operations
			// For very small files (< 10ms), use minimum 1ms to avoid zero
			estimatedReadingTime := uint64(float64(metrics.ParsingTimeMs) * 0.15)
			if estimatedReadingTime == 0 && metrics.ParsingTimeMs > 0 {
				// For tiny files, use at least 1ms
				metrics.FileReadingTimeMs = 1
			} else {
				metrics.FileReadingTimeMs = estimatedReadingTime
			}
		}
	}
	
	// CRITICAL: Verify time coverage - check that all times cover the entire process
	// Total time should be: FileReadingTimeMs + RecordParsingTimeMs + DeduplicationTimeMs + WritingTimeMs
	// But in streaming mode, FileReadingTimeMs and RecordParsingTimeMs overlap, so:
	// ParsingTimeMs ≈ max(FileReadingTimeMs, RecordParsingTimeMs) or FileReadingTimeMs + RecordParsingTimeMs
	// Total process time = ParsingTimeMs + DeduplicationTimeMs + WritingTimeMs (if sequential)
	// OR Total process time = max(ParsingTimeMs, DeduplicationTimeMs + WritingTimeMs) (if parallel)
	calculatedTotalTime := metrics.FileReadingTimeMs + metrics.RecordParsingTimeMs + metrics.DeduplicationTimeMs + metrics.WritingTimeMs
	parsingBasedTotalTime := metrics.ParsingTimeMs + metrics.DeduplicationTimeMs + metrics.WritingTimeMs
	
	// Log time coverage analysis
	log.Info().
		Str("file_path", metrics.FilePath).
		Uint64("parsing_time_ms", metrics.ParsingTimeMs).
		Uint64("file_reading_time_ms", metrics.FileReadingTimeMs).
		Uint64("record_parsing_time_ms", metrics.RecordParsingTimeMs).
		Uint64("deduplication_time_ms", metrics.DeduplicationTimeMs).
		Uint64("writing_time_ms", metrics.WritingTimeMs).
		Uint64("calculated_total_time", calculatedTotalTime).
		Uint64("parsing_based_total_time", parsingBasedTotalTime).
		Int64("time_coverage_diff", int64(calculatedTotalTime) - int64(parsingBasedTotalTime)).
		Msg("Time coverage analysis - verifying all times cover the entire process")
	
	// Warn if times don't make sense
	if metrics.ParsingTimeMs > 0 && calculatedTotalTime < metrics.ParsingTimeMs {
		log.Warn().
			Str("file_path", metrics.FilePath).
			Uint64("parsing_time_ms", metrics.ParsingTimeMs).
			Uint64("calculated_total_time", calculatedTotalTime).
			Msg("WARNING: Calculated total time is less than parsing time - times may not cover the entire process")
	}
	
	// In streaming mode, FileReadingTimeMs and RecordParsingTimeMs overlap
	// So FileReadingTimeMs + RecordParsingTimeMs can be > ParsingTimeMs
	// This is expected and correct
	if metrics.FileReadingTimeMs + metrics.RecordParsingTimeMs > metrics.ParsingTimeMs && metrics.ParsingTimeMs > 0 {
		log.Debug().
			Str("file_path", metrics.FilePath).
			Uint64("file_reading_time_ms", metrics.FileReadingTimeMs).
			Uint64("record_parsing_time_ms", metrics.RecordParsingTimeMs).
			Uint64("parsing_time_ms", metrics.ParsingTimeMs).
			Msg("FileReadingTimeMs + RecordParsingTimeMs > ParsingTimeMs (expected in streaming mode - times overlap)")
	}
	
	// CRITICAL: Delete old record for this file before inserting new one
	// ReplacingMergeTree replaces records only during merge (async), so we need to delete explicitly
	// to ensure only one record per file exists at any time
	// ORDER BY is (parser_type, cluster_guid, infobase_guid, file_path)
	deleteQuery := fmt.Sprintf(
		"ALTER TABLE logs.parser_metrics DELETE WHERE parser_type = '%s' AND cluster_guid = '%s' AND infobase_guid = '%s' AND file_path = '%s'",
		metrics.ParserType,
		metrics.ClusterGUID,
		metrics.InfobaseGUID,
		strings.ReplaceAll(metrics.FilePath, "'", "''"), // Escape single quotes
	)
	
	// Execute DELETE asynchronously (ClickHouse processes ALTER DELETE in background)
	// We don't wait for completion - new INSERT will work correctly
	if err := w.conn.Exec(ctx, deleteQuery); err != nil {
		// Log warning but continue - INSERT will still work, ReplacingMergeTree will handle duplicates on merge
		log.Warn().
			Err(err).
			Str("file_path", metrics.FilePath).
			Msg("Failed to delete old parser_metrics record, continuing with INSERT")
	}
	
	batch, err := w.conn.PrepareBatch(ctx, `INSERT INTO logs.parser_metrics (
		timestamp, parser_type, cluster_guid, cluster_name, infobase_guid, infobase_name,
		file_path, file_name, files_processed, records_parsed, parsing_time_ms, records_per_second,
		start_time, end_time, error_count, file_reading_time_ms, record_parsing_time_ms,
		deduplication_time_ms, writing_time_ms, updated_at
	)`)
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}
	
	err = batch.Append(
		metrics.Timestamp,
		metrics.ParserType,
		metrics.ClusterGUID,
		metrics.ClusterName,
		metrics.InfobaseGUID,
		metrics.InfobaseName,
		metrics.FilePath,
		metrics.FileName,
		metrics.FilesProcessed,
		metrics.RecordsParsed,
		metrics.ParsingTimeMs,
		metrics.RecordsPerSecond,
		metrics.StartTime,
		metrics.EndTime,
		metrics.ErrorCount,
		metrics.FileReadingTimeMs,
		metrics.RecordParsingTimeMs,
		metrics.DeduplicationTimeMs,
		metrics.WritingTimeMs,
		time.Now(), // updated_at for ReplacingMergeTree
	)
	if err != nil {
		return fmt.Errorf("failed to append to batch: %w", err)
	}
	
	// Send batch with retry logic
	if err := retry.Do(ctx, w.retryCfg, func() error {
		return batch.Send()
	}); err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}
	
	log.Debug().
		Str("parser_type", metrics.ParserType).
		Uint32("files_processed", metrics.FilesProcessed).
		Uint64("records_parsed", metrics.RecordsParsed).
		Float64("records_per_second", metrics.RecordsPerSecond).
		Msg("Parser metrics written to ClickHouse")
	
	return nil
}

// WriteFileReadingProgress writes file reading progress to ClickHouse
// Mirrors offsets from BoltDB but with additional metadata for monitoring
// CRITICAL: Deletes old record for the same file before inserting new one to ensure only one record per file
func (w *ClickHouseWriter) WriteFileReadingProgress(ctx context.Context, progress *domain.FileReadingProgress) error {
	if progress == nil {
		return fmt.Errorf("progress cannot be nil")
	}
	
	// CRITICAL: Delete old record for this file before inserting new one
	// ReplacingMergeTree replaces records only during merge (async), so we need to delete explicitly
	// to ensure only one record per file exists at any time
	deleteQuery := fmt.Sprintf(
		"ALTER TABLE logs.file_reading_progress DELETE WHERE parser_type = '%s' AND cluster_guid = '%s' AND infobase_guid = '%s' AND file_path = '%s'",
		progress.ParserType,
		progress.ClusterGUID,
		progress.InfobaseGUID,
		strings.ReplaceAll(progress.FilePath, "'", "''"), // Escape single quotes
	)
	
	// Execute DELETE asynchronously (ClickHouse processes ALTER DELETE in background)
	// We don't wait for completion - new INSERT will work correctly
	if err := w.conn.Exec(ctx, deleteQuery); err != nil {
		// Log warning but continue - INSERT will still work, ReplacingMergeTree will handle duplicates on merge
		log.Warn().
			Err(err).
			Str("file_path", progress.FilePath).
			Msg("Failed to delete old file_reading_progress record, continuing with INSERT")
	}
	
	// Calculate progress percentage
	var progressPercent float64
	if progress.FileSizeBytes > 0 {
		progressPercent = float64(progress.OffsetBytes) / float64(progress.FileSizeBytes) * 100.0
	}
	
	batch, err := w.conn.PrepareBatch(ctx, "INSERT INTO logs.file_reading_progress")
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}
	
	err = batch.Append(
		progress.Timestamp,
		progress.ParserType,
		progress.ClusterGUID,
		progress.ClusterName,
		progress.InfobaseGUID,
		progress.InfobaseName,
		progress.FilePath,
		progress.FileName,
		progress.FileSizeBytes,
		progress.OffsetBytes,
		progress.RecordsParsed,
		progress.LastTimestamp,
		progressPercent,
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to append to batch: %w", err)
	}
	
	// Send batch with retry logic
	if err := retry.Do(ctx, w.retryCfg, func() error {
		return batch.Send()
	}); err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}
	
	log.Debug().
		Str("parser_type", progress.ParserType).
		Str("file_name", progress.FileName).
		Uint64("offset_bytes", progress.OffsetBytes).
		Uint64("file_size_bytes", progress.FileSizeBytes).
		Float64("progress_percent", progressPercent).
		Msg("File reading progress written to ClickHouse")
	
	return nil
}

// Close flushes and closes the writer
func (w *ClickHouseWriter) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Flush pending records
	if err := w.Flush(ctx); err != nil {
		log.Error().Err(err).Msg("Error flushing during close")
	}
	
	return nil
}

// flushEventLog writes event log batch to ClickHouse with deduplication
func (w *ClickHouseWriter) flushEventLog(ctx context.Context) error {
	w.batchMutex.Lock()
	if len(w.eventLogBatch) == 0 {
		w.batchMutex.Unlock()
		return nil
	}
	
	// CRITICAL: Create a snapshot to avoid modification during processing
	batchSnapshot := make([]*domain.EventLogRecord, len(w.eventLogBatch))
	copy(batchSnapshot, w.eventLogBatch)
	
	// Clear the original batch
	w.eventLogBatch = w.eventLogBatch[:0]
	w.batchMutex.Unlock()
	
	return w.flushEventLogSnapshot(ctx, batchSnapshot)
}

// flushEventLogSnapshot processes a snapshot of the batch (thread-safe)
func (w *ClickHouseWriter) flushEventLogSnapshot(ctx context.Context, batchSnapshot []*domain.EventLogRecord) error {
	if len(batchSnapshot) == 0 {
		return nil
	}
	
	// Start timing
	startTime := time.Now()
	
	// Log batch start
	log.Debug().
		Int("batch_size", len(batchSnapshot)).
		Msg("Starting to process event log batch snapshot")
	
	// Calculate hashes for all records first
	// Use map to track unique hashes and their records (handle duplicates within batch)
	hashToRecords := make(map[string][]*domain.EventLogRecord, len(batchSnapshot))
	hashOrder := make([]string, 0, len(batchSnapshot)) // Preserve order for processing
	
	skippedCount := 0
	skippedReasons := make(map[string]int)
	skippedRecords := make([]map[string]interface{}, 0)
	processedCount := 0
	
	// CRITICAL: Store batch size at start
	initialBatchSize := len(batchSnapshot)
	
	// Step 1: Calculate all hashes and group by hash (handle duplicates within batch)
	for idx, record := range batchSnapshot {
		processedCount++
		
		// CRITICAL: Verify batch snapshot hasn't changed (should never happen, but check anyway)
		if len(batchSnapshot) != initialBatchSize {
			log.Error().
				Int("initial_size", initialBatchSize).
				Int("current_size", len(batchSnapshot)).
				Int("record_index", idx).
				Msg("CRITICAL: Batch snapshot size changed - this should never happen!")
		}
		
		hash, err := calculateEventLogHash(record)
		if err != nil {
			skippedCount++
			skippedReasons["hash_calculation_failed"]++
			skippedInfo := map[string]interface{}{
				"index":           idx,
				"reason":          "hash_calculation_failed",
				"error":           err.Error(),
				"event_time":      record.EventTime.Format("2006-01-02 15:04:05"),
				"transaction_datetime": record.TransactionDateTime.Format("2006-01-02 15:04:05"),
				"event":           record.Event,
				"level":           record.Level,
				"cluster_guid":    record.ClusterGUID,
				"infobase_guid":   record.InfobaseGUID,
			}
			skippedRecords = append(skippedRecords, skippedInfo)
			log.Error().
				Err(err).
				Int("record_index", idx).
				Str("event_time", record.EventTime.Format("2006-01-02 15:04:05")).
				Str("transaction_datetime", record.TransactionDateTime.Format("2006-01-02 15:04:05")).
				Str("event", record.Event).
				Str("level", record.Level).
				Str("cluster_guid", record.ClusterGUID).
				Str("infobase_guid", record.InfobaseGUID).
				Msg("CRITICAL: Failed to calculate hash, skipping record - RECORD WILL BE LOST")
			continue
		}
		
		// Add record to hash group (handle duplicates within batch)
		if _, exists := hashToRecords[hash]; !exists {
			hashOrder = append(hashOrder, hash)
			hashToRecords[hash] = make([]*domain.EventLogRecord, 0, 1)
		}
		hashToRecords[hash] = append(hashToRecords[hash], record)
	}
	
	// Get unique hashes for batch check
	recordHashes := hashOrder
	
	// Step 2: Batch check all hashes at once (if deduplication is enabled)
	var deduplicationTime time.Duration
	existingHashes := make(map[string]bool, len(recordHashes))
	
	if w.cfg.EnableDeduplication && len(recordHashes) > 0 {
		dedupCheckStart := time.Now()
		existing, err := w.checkHashesBatch(ctx, "event_log", recordHashes)
		deduplicationTime = time.Since(dedupCheckStart)
		
		if err != nil {
			log.Warn().Err(err).Int("hashes_count", len(recordHashes)).Msg("Failed to batch check hashes, will try to insert all records")
			// Continue with insert - if duplicates exist, ClickHouse will handle them
		} else {
			// Build map of existing hashes for fast lookup
			for _, hash := range existing {
				existingHashes[hash] = true
			}
		}
	}
	
	// Step 3: Filter records based on deduplication results
	recordsToWrite := make([]*domain.EventLogRecord, 0, len(batchSnapshot))
	hashes := make([]string, 0, len(batchSnapshot))
	
	for _, hash := range recordHashes {
		records := hashToRecords[hash]
		
		// Check if hash already exists in ClickHouse (deduplication)
		if w.cfg.EnableDeduplication && existingHashes[hash] {
			// All records with this hash are duplicates - skip all
			for _, record := range records {
				skippedCount++
				skippedReasons["duplicate"]++
				skippedInfo := map[string]interface{}{
					"reason":          "duplicate",
					"hash":            hash,
					"event_time":      record.EventTime.Format("2006-01-02 15:04:05"),
					"transaction_datetime": record.TransactionDateTime.Format("2006-01-02 15:04:05"),
					"event":           record.Event,
					"level":           record.Level,
				}
				skippedRecords = append(skippedRecords, skippedInfo)
				log.Debug().
					Str("hash", hash).
					Str("event_time", record.EventTime.Format("2006-01-02 15:04:05")).
					Str("event", record.Event).
					Msg("Skipping duplicate event log record")
			}
			continue
		}
		
		// Hash doesn't exist in ClickHouse - add all records with this hash
		// If there are duplicates within batch (same hash), add only first one
		// (duplicates within batch are also considered duplicates)
		if len(records) > 1 {
			// Multiple records with same hash in batch - skip duplicates
			// CRITICAL: Log detailed comparison to understand why records are considered duplicates
			firstRecord := records[0]
			log.Warn().
				Str("hash", hash).
				Int("duplicate_count", len(records)-1).
				Str("first_event_time", firstRecord.EventTime.Format("2006-01-02 15:04:05.000000")).
				Str("first_transaction_datetime", firstRecord.TransactionDateTime.Format("2006-01-02 15:04:05.000000")).
				Str("first_event", firstRecord.Event).
				Str("first_level", firstRecord.Level).
				Str("first_cluster_guid", firstRecord.ClusterGUID).
				Str("first_infobase_guid", firstRecord.InfobaseGUID).
				Str("first_user", firstRecord.UserName).
				Str("first_computer", firstRecord.Computer).
				Uint64("first_session_id", firstRecord.SessionID).
				Uint64("first_connection_id", firstRecord.ConnectionID).
				Str("first_transaction_id", firstRecord.TransactionID).
				Int64("first_transaction_number", firstRecord.TransactionNumber).
				Str("first_comment", firstRecord.Comment).
				Str("first_data", firstRecord.Data).
				Msg("CRITICAL: Found duplicate records in batch - comparing details")
			
			for i := 1; i < len(records); i++ {
				dupRecord := records[i]
				skippedCount++
				skippedReasons["duplicate_in_batch"]++
				skippedInfo := map[string]interface{}{
					"reason":          "duplicate_in_batch",
					"hash":            hash,
					"event_time":      dupRecord.EventTime.Format("2006-01-02 15:04:05"),
					"transaction_datetime": dupRecord.TransactionDateTime.Format("2006-01-02 15:04:05"),
					"event":           dupRecord.Event,
					"level":           dupRecord.Level,
				}
				skippedRecords = append(skippedRecords, skippedInfo)
				
				// Detailed comparison log
				log.Warn().
					Str("hash", hash).
					Int("duplicate_index", i).
					Str("event_time", dupRecord.EventTime.Format("2006-01-02 15:04:05.000000")).
					Str("transaction_datetime", dupRecord.TransactionDateTime.Format("2006-01-02 15:04:05.000000")).
					Str("event", dupRecord.Event).
					Str("level", dupRecord.Level).
					Str("cluster_guid", dupRecord.ClusterGUID).
					Str("infobase_guid", dupRecord.InfobaseGUID).
					Str("user", dupRecord.UserName).
					Str("computer", dupRecord.Computer).
					Uint64("session_id", dupRecord.SessionID).
					Uint64("connection_id", dupRecord.ConnectionID).
					Str("transaction_id", dupRecord.TransactionID).
					Int64("transaction_number", dupRecord.TransactionNumber).
					Str("comment", dupRecord.Comment).
					Str("data", dupRecord.Data).
					Str("data_presentation", dupRecord.DataPresentation).
					Str("event_presentation", dupRecord.EventPresentation).
					Str("metadata_name", dupRecord.MetadataName).
					Str("metadata_presentation", dupRecord.MetadataPresentation).
					Str("server", dupRecord.Server).
					Uint16("primary_port", dupRecord.PrimaryPort).
					Uint16("secondary_port", dupRecord.SecondaryPort).
					Str("application", dupRecord.Application).
					Str("data_separation", dupRecord.DataSeparation).
					Str("transaction_status", dupRecord.TransactionStatus).
					Interface("properties", dupRecord.Properties).
					Msg("CRITICAL: Duplicate record details - all hash fields")
			}
		}
		
		// Add first record with this hash (or only record if no duplicates in batch)
		record := records[0]
		recordsToWrite = append(recordsToWrite, record)
		hashes = append(hashes, hash)
		
		// Log every 50th record for debugging
		if len(recordsToWrite)%50 == 0 {
			log.Debug().
				Int("records_added", len(recordsToWrite)).
				Int("records_processed", processedCount).
				Int("records_skipped", skippedCount).
				Str("event_time", record.EventTime.Format("2006-01-02 15:04:05")).
				Str("event", record.Event).
				Msg("Progress: adding record to batch")
		}
	}
	
	// Calculate processing time
	processingTime := time.Since(startTime)
	
	// Log batch processing summary
	finalBatchSize := len(batchSnapshot)
	log.Debug().
		Int("initial_batch_size", initialBatchSize).
		Int("final_batch_size", finalBatchSize).
		Int("processed", processedCount).
		Int("added_to_write", len(recordsToWrite)).
		Int("skipped_in_loop", skippedCount).
		Int("difference", finalBatchSize-len(recordsToWrite)).
		Dur("processing_time_ms", processingTime).
		Float64("records_per_second", float64(processedCount)/processingTime.Seconds()).
		Msg("Batch processing complete")
	
	// CRITICAL: Verify all records were processed
	if processedCount != initialBatchSize {
		log.Error().
			Int("initial_batch_size", initialBatchSize).
			Int("processed_count", processedCount).
			Int("difference", initialBatchSize-processedCount).
			Msg("CRITICAL: Not all records were processed in the loop - RECORDS WILL BE LOST!")
	}
	
	// Log skipped records summary with details
	if skippedCount > 0 {
		log.Error().
			Int("skipped_count", skippedCount).
			Interface("reasons", skippedReasons).
			Interface("skipped_records", skippedRecords).
			Msg("CRITICAL: Some records were skipped and will not be written to ClickHouse")
	}
	
	// CRITICAL: Check if there's a discrepancy
	if finalBatchSize != len(recordsToWrite) && skippedCount == 0 {
		log.Error().
			Int("total_in_batch", finalBatchSize).
			Int("records_to_write", len(recordsToWrite)).
			Int("processed_count", processedCount).
			Int("difference", finalBatchSize-len(recordsToWrite)).
			Int("skipped_count", skippedCount).
			Msg("CRITICAL: Records are missing but skippedCount is 0 - INVESTIGATE!")
	}
	
	// CRITICAL: Final verification - all records must be accounted for
	if processedCount != len(recordsToWrite)+skippedCount {
		log.Error().
			Int("processed_count", processedCount).
			Int("records_to_write", len(recordsToWrite)).
			Int("skipped_count", skippedCount).
			Int("expected_sum", len(recordsToWrite)+skippedCount).
			Int("difference", processedCount-(len(recordsToWrite)+skippedCount)).
			Msg("CRITICAL: Record count mismatch - some records are unaccounted for!")
	}
	
	if len(recordsToWrite) == 0 {
		if w.cfg.EnableDeduplication {
			log.Debug().Int("total", len(batchSnapshot)).Msg("All event log records were duplicates, skipping batch")
		}
		// Log deduplication metrics even if no records to write
		if w.cfg.EnableDeduplication {
			deduplicationTimeMs := uint64(deduplicationTime.Milliseconds())
			log.Info().
				Int("batch_size", len(batchSnapshot)).
				Int("duplicates", skippedCount).
				Uint64("deduplication_time_ms", deduplicationTimeMs).
				Float64("deduplication_time_per_record_ms", float64(deduplicationTimeMs)/float64(len(batchSnapshot))).
				Msg("Deduplication metrics (all records were duplicates)")
		}
		return nil
	}
	
	// Normalize comments for error records synchronously before writing
	// Note: Level is stored in Russian ("Ошибка") in 1C Event Log
	normalizedComments := make([]string, len(recordsToWrite))
	for i, record := range recordsToWrite {
		if (record.Level == "Error" || record.Level == "Ошибка") && record.Comment != "" {
			normalizedComments[i] = w.normalizer.NormalizeComment(record.Comment)
		} else {
			normalizedComments[i] = ""
		}
	}
	
	// Measure writing time separately
	writingStartTime := time.Now()
	
	batch, err := w.conn.PrepareBatch(ctx, `INSERT INTO logs.event_log (
		event_time, cluster_guid, cluster_name, infobase_guid, infobase_name,
		level, event, event_presentation, user_name, user_id, computer,
		application, application_presentation, session_id, connection_id, connection,
		transaction_status, transaction_id, transaction_number, transaction_datetime,
		data_separation, metadata_name, metadata_presentation, comment, data,
		data_presentation, server, primary_port, secondary_port, props_key, props_value,
		record_hash, comment_normalized
	)`)
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}
	
	for i, record := range recordsToWrite {
		propKeys, propVals := mapToArrays(record.Properties)
		
		err := batch.Append(
			record.EventTime,
			record.ClusterGUID,
			record.ClusterName,
			record.InfobaseGUID,
			record.InfobaseName,
			record.Level,
			record.Event,
			record.EventPresentation,
			record.UserName,
			record.UserID,
			record.Computer,
			record.Application,
			record.ApplicationPresentation,
			record.SessionID,
			record.ConnectionID,
			record.Connection,
			record.TransactionStatus,
			record.TransactionID,
			record.TransactionNumber,
			record.TransactionDateTime,
			record.DataSeparation,
			record.MetadataName,
			record.MetadataPresentation,
			record.Comment,
			record.Data,
			record.DataPresentation,
			record.Server,
			record.PrimaryPort,
			record.SecondaryPort,
			propKeys,
			propVals,
			hashes[i], // record_hash
			normalizedComments[i], // comment_normalized - normalized synchronously
		)
		if err != nil {
			// CRITICAL: Log detailed information about the failed record
			log.Error().
				Err(err).
				Str("event_time", record.EventTime.Format("2006-01-02 15:04:05")).
				Str("transaction_datetime", record.TransactionDateTime.Format("2006-01-02 15:04:05")).
				Str("event", record.Event).
				Str("level", record.Level).
				Str("cluster_guid", record.ClusterGUID).
				Str("infobase_guid", record.InfobaseGUID).
				Int64("transaction_number", record.TransactionNumber).
				Uint64("connection_id", record.ConnectionID).
				Str("hash", hashes[i]).
				Msg("CRITICAL: Failed to append record to batch - RECORD WILL BE LOST")
			return fmt.Errorf("failed to append to batch (record index %d, event_time=%s, transaction_datetime=%s): %w", 
				i, record.EventTime.Format("2006-01-02 15:04:05"), record.TransactionDateTime.Format("2006-01-02 15:04:05"), err)
		}
	}
	
	// Send batch with retry logic
	if err := retry.Do(ctx, w.retryCfg, func() error {
		return batch.Send()
	}); err != nil {
		// CRITICAL: Log detailed information about the failed batch
		log.Error().
			Err(err).
			Int("total_in_batch", len(w.eventLogBatch)).
			Int("records_to_write", len(recordsToWrite)).
			Msg("CRITICAL: Failed to send batch to ClickHouse after retries - ALL RECORDS IN BATCH WILL BE LOST")
		return fmt.Errorf("failed to send batch (total=%d, to_write=%d): %w", len(w.eventLogBatch), len(recordsToWrite), err)
	}
	
	// Calculate times
	writingTime := time.Since(writingStartTime)
	totalTime := time.Since(startTime)
	deduplicationTimeMs := uint64(deduplicationTime.Milliseconds())
	writingTimeMs := uint64(writingTime.Milliseconds())
	
	// Accumulate metrics for reporting (thread-safe)
	w.metricsMutex.Lock()
	oldWritingRecordsCount := w.writingRecordsCount
	oldWritingTimeAccumulator := w.writingTimeAccumulator
	
	w.deduplicationTimeAccumulator += deduplicationTime
	w.writingTimeAccumulator += writingTime
	w.deduplicationRecordsCount += uint64(len(batchSnapshot))
	w.writingRecordsCount += uint64(len(recordsToWrite))
	
	// Log metrics accumulation for debugging
	log.Info().
		Int("records_written", len(recordsToWrite)).
		Dur("writing_time", writingTime).
		Uint64("old_writing_records_count", oldWritingRecordsCount).
		Uint64("new_writing_records_count", w.writingRecordsCount).
		Dur("old_writing_time_accumulator", oldWritingTimeAccumulator).
		Dur("new_writing_time_accumulator", w.writingTimeAccumulator).
		Dur("added_writing_time", writingTime).
		Msg("Accumulated writing metrics (event_log)")
	
	w.metricsMutex.Unlock()
	
	logEntry := log.Info().
		Int("total", len(batchSnapshot)).
		Int("written", len(recordsToWrite)).
		Dur("total_time_ms", totalTime).
		Dur("writing_time_ms", writingTime).
		Float64("records_per_second", float64(len(recordsToWrite))/totalTime.Seconds())
	if w.cfg.EnableDeduplication {
		dedupPct := 0.0
		if totalTime.Milliseconds() > 0 {
			dedupPct = float64(deduplicationTimeMs) / float64(totalTime.Milliseconds()) * 100
		}
		dedupPerRecord := 0.0
		if len(batchSnapshot) > 0 {
			dedupPerRecord = float64(deduplicationTimeMs) / float64(len(batchSnapshot))
		}
		logEntry = logEntry.
			Int("duplicates", len(batchSnapshot)-len(recordsToWrite)).
			Uint64("deduplication_time_ms", deduplicationTimeMs).
			Float64("deduplication_time_per_record_ms", dedupPerRecord).
			Float64("deduplication_percentage", dedupPct).
			Uint64("writing_time_ms", writingTimeMs)
		log.Info().
			Int("batch_size", len(batchSnapshot)).
			Int("duplicates", len(batchSnapshot)-len(recordsToWrite)).
			Uint64("deduplication_time_ms", deduplicationTimeMs).
			Uint64("writing_time_ms", writingTimeMs).
			Float64("deduplication_time_per_record_ms", dedupPerRecord).
			Float64("deduplication_percentage", dedupPct).
			Msg("Deduplication metrics (can be added to parser_metrics table)")
	}
	if len(batchSnapshot) != len(recordsToWrite) {
		// Log warning if some records were skipped (not duplicates)
		logEntry = logEntry.Int("skipped", len(batchSnapshot)-len(recordsToWrite))
	}
	logEntry.Msg("Flushed event log batch to ClickHouse")
	
	w.lastFlush = time.Now()
	
	return nil
}

// flushTechLog writes tech log batch to ClickHouse with deduplication
func (w *ClickHouseWriter) flushTechLog(ctx context.Context) error {
	w.batchMutex.Lock()
	if len(w.techLogBatch) == 0 {
		w.batchMutex.Unlock()
		return nil
	}

	// CRITICAL: Create a snapshot to avoid modification during processing
	batchSnapshot := make([]*domain.TechLogRecord, len(w.techLogBatch))
	copy(batchSnapshot, w.techLogBatch)

	// Clear the original batch
	w.techLogBatch = w.techLogBatch[:0]
	w.batchMutex.Unlock()

	return w.flushTechLogSnapshot(ctx, batchSnapshot)
}

// flushTechLogSnapshot processes a snapshot of the batch (thread-safe)
func (w *ClickHouseWriter) flushTechLogSnapshot(ctx context.Context, batchSnapshot []*domain.TechLogRecord) error {
	if len(batchSnapshot) == 0 {
		return nil
	}

	// Start timing
	startTime := time.Now()

	// Calculate hashes for all records first
	// Use map to track unique hashes and their records (handle duplicates within batch)
	hashToRecords := make(map[string][]*domain.TechLogRecord, len(batchSnapshot))
	hashOrder := make([]string, 0, len(batchSnapshot)) // Preserve order for processing
	
	// Step 1: Calculate all hashes and group by hash (handle duplicates within batch)
	for _, record := range batchSnapshot {
		hash, err := calculateTechLogHash(record)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to calculate hash, skipping record")
			continue
		}
		
		// Add record to hash group (handle duplicates within batch)
		if _, exists := hashToRecords[hash]; !exists {
			hashOrder = append(hashOrder, hash)
			hashToRecords[hash] = make([]*domain.TechLogRecord, 0, 1)
		}
		hashToRecords[hash] = append(hashToRecords[hash], record)
	}
	
	// Get unique hashes for batch check
	recordHashes := hashOrder
	
	// Step 2: Batch check all hashes at once (if deduplication is enabled)
	var deduplicationTime time.Duration
	existingHashes := make(map[string]bool, len(recordHashes))
	
	if w.cfg.EnableDeduplication && len(recordHashes) > 0 {
		dedupCheckStart := time.Now()
		existing, err := w.checkHashesBatch(ctx, "tech_log", recordHashes)
		deduplicationTime = time.Since(dedupCheckStart)
		
		if err != nil {
			log.Warn().Err(err).Int("hashes_count", len(recordHashes)).Msg("Failed to batch check hashes, will try to insert all records")
			// Continue with insert - if duplicates exist, ClickHouse will handle them
		} else {
			// Build map of existing hashes for fast lookup
			for _, hash := range existing {
				existingHashes[hash] = true
			}
		}
	}
	
	// Step 3: Filter records based on deduplication results
	recordsToWrite := make([]*domain.TechLogRecord, 0, len(batchSnapshot))
	hashes := make([]string, 0, len(batchSnapshot))
	
	for _, hash := range recordHashes {
		records := hashToRecords[hash]
		
		// Check if hash already exists in ClickHouse (deduplication)
		if w.cfg.EnableDeduplication && existingHashes[hash] {
			// All records with this hash are duplicates - skip all
			log.Debug().
				Str("hash", hash).
				Int("duplicates_count", len(records)).
				Msg("Skipping duplicate tech log records")
			continue
		}
		
		// Hash doesn't exist in ClickHouse - add all records with this hash
		// If there are duplicates within batch (same hash), add only first one
		// (duplicates within batch are also considered duplicates)
		if len(records) > 1 {
			// Multiple records with same hash in batch - skip duplicates
			for i := 1; i < len(records); i++ {
				log.Debug().
					Str("hash", hash).
					Int("duplicate_index", i).
					Msg("Skipping duplicate tech log record within batch")
			}
		}
		
		// Add first record with this hash (or only record if no duplicates in batch)
		record := records[0]
		recordsToWrite = append(recordsToWrite, record)
		hashes = append(hashes, hash)
	}
	
	if len(recordsToWrite) == 0 {
		if w.cfg.EnableDeduplication {
			log.Debug().Int("total", len(batchSnapshot)).Msg("All tech log records were duplicates, skipping batch")
			// Log deduplication metrics even if no records to write
			deduplicationTimeMs := uint64(deduplicationTime.Milliseconds())
			log.Info().
				Int("batch_size", len(batchSnapshot)).
				Uint64("deduplication_time_ms", deduplicationTimeMs).
				Float64("deduplication_time_per_record_ms", float64(deduplicationTimeMs)/float64(len(batchSnapshot))).
				Msg("Deduplication metrics (all records were duplicates)")
		}
		return nil
	}
	
	// Normalize raw_line for all records synchronously before writing
	normalizedRawLines := make([]string, len(recordsToWrite))
	for i, record := range recordsToWrite {
		if record.RawLine != "" {
			normalizedRawLines[i] = w.techLogNormalizer.NormalizeRawLine(record.RawLine)
		} else {
			normalizedRawLines[i] = ""
		}
	}
	
	// Measure writing time separately
	writingStartTime := time.Now()
	
	batch, err := w.conn.PrepareBatch(ctx, "INSERT INTO logs.tech_log")
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}
	
	for i, record := range recordsToWrite {
		propKeys, propVals := mapToArrays(record.Properties)
		
		err := batch.Append(
			// Core fields
			record.Timestamp,
			record.Duration,
			record.Name,
			record.Level,
			record.Depth,
			record.Process,
			record.OSThread,
			record.ClientID,
			record.SessionID,
			record.TransactionID,
			record.User,
			record.ApplicationID,
			record.ConnectionID,
			record.Interface,
			record.Method,
			record.CallID,
			record.ClusterGUID,
			record.ClusterName,
			record.InfobaseGUID,
			record.InfobaseName,
			record.RawLine,
			// SQL event properties (DBMSSQL, DBPOSTGRS, DBORACLE, DB2, DBV8DBENG, DBDA, EDS)
			record.SQL,
			record.PlanSQLText,
			record.Rows,
			record.RowsAffected,
			record.DBMS,
			record.Database,
			record.Dbpid,
			record.DBCopy,
			record.NParams,
			record.MDX,
			record.DBConnID,
			record.DBConnStr,
			record.DBUsr,
			// SDBL query properties
			record.Query,
			record.Sdbl,
			record.QueryFields,
			// Exception properties (EXCP, EXCPCNTX)
			record.Exception,
			record.ExceptionDescr,
			record.ExceptionContext,
			record.Func,
			record.Line,
			record.File,
			record.Module,
			record.OSException,
			// Lock properties (TLOCK, TTIMEOUT, TDEADLOCK)
			record.Locks,
			record.Regions,
			record.WaitConnections,
			record.Lka,
			record.Lkp,
			record.Lkpid,
			record.Lkaid,
			record.Lksrc,
			record.Lkpto,
			record.Lkato,
			record.DeadlockConnectionIntersections,
			// Connection properties (CONN)
			record.Server,
			record.Port,
			record.SyncPort,
			record.Connection,
			record.HResultOLEDB,
			record.HResultNC2005,
			record.HResultNC2008,
			record.HResultNC2012,
			// Session properties (SESN)
			record.SessionNmb,
			record.SeanceID,
			// Process properties (PROC)
			record.ProcID,
			record.PID,
			record.ProcessName,
			record.PProcessName,
			record.SrcProcessName,
			record.Finish,
			record.ExitCode,
			record.RunAs,
			// Call properties (CALL, SCALL)
			record.MName,
			record.IName,
			record.DstClientID,
			record.RetExcp,
			record.Memory,
			record.MemoryPeak,
			// Cluster properties (CLSTR)
			record.ClusterEvent,
			record.Cluster,
			record.IB,
			record.Ref,
			record.Connections,
			record.ConnLimit,
			record.Infobases,
			record.IBLimit,
			record.DstAddr,
			record.DstId,
			record.DstPid,
			record.DstSrv,
			record.SrcAddr,
			record.SrcId,
			record.SrcPid,
			record.SrcSrv,
			record.SrcURL,
			record.MyVer,
			record.SrcVer,
			record.Registered,
			record.Obsolete,
			record.Released,
			record.Reason,
			record.Request,
			record.ServiceName,
			record.ApplicationExt,
			record.NeedResync,
			record.NewServiceDataDirectory,
			record.OldServiceDataDirectory,
			// Server context properties (SCOM)
			record.ServerComputerName,
			record.ProcURL,
			record.AgentURL,
			// Admin properties (ADMIN)
			record.Admin,
			record.Action,
			// Memory properties (MEM, LEAKS, ATTN)
			record.Sz,
			record.Szd,
			record.Cn,
			record.Cnd,
			record.MemoryLimits,
			record.ExcessDurationSec,
			ensureValidDateTime(record.ExcessStartTime),
			record.FreeMemory,
			record.TotalMemory,
			record.SafeLimit,
			record.AttnInfo,
			record.AttnPID,
			record.AttnProcessID,
			record.AttnServerID,
			record.AttnURL,
			// License properties (LIC, HASP)
			record.LicRes,
			record.HaspID,
			// Full-text search properties (FTEXTUPD, FTS, FTEXTCHECK, INPUTBYSTRING)
			record.FtextState,
			record.AvMem,
			record.BackgroundJobCreated,
			record.MemoryUsed,
			record.FailedJobsCount,
			record.TotalJobsCount,
			record.JobCanceledByLoadLimit,
			record.MinDataID,
			record.FtextFiles,
			record.FtextFilesCount,
			record.FtextFilesTotalSize,
			record.FtextFolder,
			record.FtextTime,
			record.FtextFile,
			record.FtextInfo,
			record.FtextResult,
			record.FtextSeparation,
			record.FtextSepID,
			record.FtextWord,
			record.FindByString,
			record.InputText,
			record.FindTicks,
			record.FtextTicks,
			record.FtextSearchCount,
			record.FtextResultCount,
			record.SearchByMask,
			record.TooManyResults,
			record.FillRefsPresent,
			record.FtsJobID,
			record.FtsLogFrom,
			record.FtsLogTo,
			record.FtsFixedState,
			record.FtsRecordCount,
			record.FtsTotalRecords,
			record.FtsTableCount,
			record.FtsTableName,
			record.FtsTableCode,
			record.FtsTableRef,
			record.FtsMetadataID,
			record.FtsRecordRef,
			record.FtsFullKey,
			record.FtsReindexCount,
			record.FtsSkippedRecords,
			record.FtsParallelism,
			// Storage properties (STORE)
			record.StoreID,
			record.StoreSize,
			record.StorageGUID,
			record.BackupFileName,
			record.BackupBaseFileName,
			record.BackupType,
			record.MinimalWriteSize,
			record.ReadOnlyMode,
			record.UseMode,
			// Garbage collector properties (SDGC)
			record.SDGCInstanceID,
			record.SDGCMethod,
			record.SDGCFilesSize,
			record.SDGCUsedSize,
			record.SDGCCopyBytes,
			record.SDGCLockDuration,
			// Add-in properties (ADDIN)
			record.AddinClasses,
			record.AddinLocation,
			record.AddinMethodName,
			record.AddinMessage,
			record.AddinSource,
			record.AddinType,
			record.AddinResult,
			record.AddinCrashed,
			record.AddinErrorDescr,
			// System event properties (SYSTEM)
			record.SystemClass,
			record.SystemComponent,
			record.SystemFile,
			record.SystemLine,
			record.SystemTxt,
			// Event log properties (EVENTLOG)
			record.EventlogFileName,
			record.EventlogCPUTime,
			record.EventlogOSThread,
			record.EventlogPacketCount,
			// Video properties (VIDEOCALL, VIDEOCONN, VIDEOSTATS)
			record.VideoConnection,
			record.VideoStatus,
			record.VideoStreamType,
			record.VideoValue,
			record.VideoCPU,
			record.VideoQueueLength,
			record.VideoInMessage,
			record.VideoOutMessage,
			record.VideoDirection,
			record.VideoType,
			// Speech recognition properties (STT, STTAdm)
			record.SttID,
			record.SttKey,
			record.SttModelID,
			record.SttPath,
			record.SttAudioEncoding,
			record.SttFrames,
			record.SttContexts,
			record.SttContextsOnly,
			record.SttRecording,
			record.SttStatus,
			record.SttPhrase,
			record.SttRxAcoustic,
			record.SttRxGrammar,
			record.SttRxLanguage,
			record.SttRxLocation,
			record.SttRxSampleRate,
			record.SttRxVersion,
			record.SttTxAcoustic,
			record.SttTxGrammar,
			record.SttTxLanguage,
			record.SttTxLocation,
			record.SttTxSampleRate,
			record.SttTxVersion,
			// Web service properties (VRSREQUEST, VRSRESPONSE)
			record.VrsURI,
			record.VrsMethod,
			record.VrsHeaders,
			record.VrsBody,
			record.VrsStatus,
			record.VrsPhrase,
			// Integration properties (SINTEG, EDS)
			record.SintegSrvcName,
			record.SintegExtSrvcURL,
			record.SintegExtSrvcUsr,
			// Mail properties (MAILPARSEERR)
			record.MailMessageUID,
			record.MailMethod,
			// Certificate properties (WINCERT)
			record.WinCertCertificate,
			record.WinCertErrorCode,
			// Data history properties (DHIST)
			record.DhistDescription,
			// Config load properties (CONFLOADFROMFILES)
			record.ConfLoadAction,
			// Background job properties
			record.Report,
			// Client properties (t: prefix)
			record.TApplicationName,
			record.TClientID,
			record.TComputerName,
			record.TConnectID,
			// Additional properties
			record.Host,
			record.Val,
			record.Err,
			record.Calls,
			record.InBytes,
			record.OutBytes,
			record.DurationUs,
			// Dynamic properties
			propKeys,
			propVals,
			hashes[i], // record_hash
			normalizedRawLines[i], // raw_line_normalized - normalized synchronously
		)
	if err != nil {
		return fmt.Errorf("failed to append to batch: %w", err)
	}
}

// Send batch with retry logic
if err := retry.Do(ctx, w.retryCfg, func() error {
	return batch.Send()
}); err != nil {
	return fmt.Errorf("failed to send batch: %w", err)
}
	
	// Calculate times
	writingTime := time.Since(writingStartTime)
	totalTime := time.Since(startTime)
	deduplicationTimeMs := uint64(deduplicationTime.Milliseconds())
	writingTimeMs := uint64(writingTime.Milliseconds())
	
	// Accumulate metrics for reporting (thread-safe)
	w.metricsMutex.Lock()
	w.deduplicationTimeAccumulator += deduplicationTime
	w.writingTimeAccumulator += writingTime
	w.deduplicationRecordsCount += uint64(len(batchSnapshot))
	w.writingRecordsCount += uint64(len(recordsToWrite))
	w.metricsMutex.Unlock()
	
	logEntry := log.Info().
		Int("total", len(batchSnapshot)).
		Int("written", len(recordsToWrite)).
		Dur("total_time_ms", totalTime).
		Dur("writing_time_ms", writingTime).
		Float64("records_per_second", float64(len(recordsToWrite))/totalTime.Seconds())
	if w.cfg.EnableDeduplication {
		dedupPct := 0.0
		if totalTime.Milliseconds() > 0 {
			dedupPct = float64(deduplicationTimeMs) / float64(totalTime.Milliseconds()) * 100
		}
		dedupPerRecord := 0.0
		if len(batchSnapshot) > 0 {
			dedupPerRecord = float64(deduplicationTimeMs) / float64(len(batchSnapshot))
		}
		logEntry = logEntry.
			Int("duplicates", len(batchSnapshot)-len(recordsToWrite)).
			Uint64("deduplication_time_ms", deduplicationTimeMs).
			Uint64("writing_time_ms", writingTimeMs).
			Float64("deduplication_time_per_record_ms", dedupPerRecord).
			Float64("deduplication_percentage", dedupPct)
		log.Info().
			Int("batch_size", len(batchSnapshot)).
			Int("duplicates", len(batchSnapshot)-len(recordsToWrite)).
			Uint64("deduplication_time_ms", deduplicationTimeMs).
			Uint64("writing_time_ms", writingTimeMs).
			Float64("deduplication_time_per_record_ms", dedupPerRecord).
			Float64("deduplication_percentage", dedupPct).
			Msg("Tech log deduplication metrics (can be added to parser_metrics table)")
	}
	logEntry.Msg("Flushed tech log batch to ClickHouse")

	w.lastFlush = time.Now()

	return nil
}

// checkHashExists checks if a record with given hash already exists in ClickHouse
// DEPRECATED: Use checkHashesBatch for better performance
func (w *ClickHouseWriter) checkHashExists(ctx context.Context, tableName, hash string) (bool, error) {
	var count uint64
	// ClickHouse uses {hash:String} syntax for parameters, but for simplicity we'll use direct substitution
	// Hash is already hex-encoded, so it's safe to use in query
	query := fmt.Sprintf("SELECT count() FROM logs.%s WHERE record_hash = '%s' LIMIT 1", tableName, hash)
	
	rows, err := w.conn.Query(ctx, query)
	if err != nil {
		return false, fmt.Errorf("failed to check hash: %w", err)
	}
	defer rows.Close()
	
	if !rows.Next() {
		return false, nil
	}
	
	if err := rows.Scan(&count); err != nil {
		return false, fmt.Errorf("failed to scan count: %w", err)
	}
	
	return count > 0, nil
}

// checkHashesBatch checks multiple hashes at once using a single query
// This is MUCH faster than checking hashes one by one (100-1000x speedup)
func (w *ClickHouseWriter) checkHashesBatch(ctx context.Context, tableName string, hashes []string) ([]string, error) {
	if len(hashes) == 0 {
		return []string{}, nil
	}
	
	// Build IN clause with escaped hashes
	// ClickHouse supports up to 10,000 values in IN clause, but we'll use batches of 1000 for safety
	const batchSize = 1000
	existingHashes := make([]string, 0, len(hashes))
	
	for i := 0; i < len(hashes); i += batchSize {
		end := i + batchSize
		if end > len(hashes) {
			end = len(hashes)
		}
		
		batch := hashes[i:end]
		
		// Build IN clause with escaped hashes
		hashList := make([]string, 0, len(batch))
		for _, hash := range batch {
			// Escape single quotes in hash (shouldn't be needed for hex, but be safe)
			escapedHash := strings.ReplaceAll(hash, "'", "''")
			hashList = append(hashList, "'"+escapedHash+"'")
		}
		
		query := fmt.Sprintf("SELECT DISTINCT record_hash FROM logs.%s WHERE record_hash IN (%s)", 
			tableName, strings.Join(hashList, ","))
		
		rows, err := w.conn.Query(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("failed to batch check hashes: %w", err)
		}
		
		for rows.Next() {
			var hash string
			if err := rows.Scan(&hash); err != nil {
				rows.Close()
				return nil, fmt.Errorf("failed to scan hash: %w", err)
			}
			existingHashes = append(existingHashes, hash)
		}
		rows.Close()
	}
	
	return existingHashes, nil
}

// mapToArrays converts a map to two arrays (keys, values) for ClickHouse Nested type
func mapToArrays(m map[string]string) ([]string, []string) {
	if len(m) == 0 {
		return []string{}, []string{}
	}
	
	keys := make([]string, 0, len(m))
	values := make([]string, 0, len(m))
	
	for k, v := range m {
		keys = append(keys, k)
		values = append(values, v)
	}
	
	return keys, values
}

