package writer

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/1c-log-checker/internal/domain"
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
	conn clickhouse.Conn
	cfg  BatchConfig
	
	eventLogBatch []*domain.EventLogRecord
	techLogBatch  []*domain.TechLogRecord
	
	lastFlush time.Time
	stopCh    chan struct{}
}

// NewClickHouseWriter creates a new ClickHouse batch writer
func NewClickHouseWriter(conn clickhouse.Conn, cfg BatchConfig) *ClickHouseWriter {
	return &ClickHouseWriter{
		conn:          conn,
		cfg:           cfg,
		eventLogBatch: make([]*domain.EventLogRecord, 0, cfg.MaxSize),
		techLogBatch:  make([]*domain.TechLogRecord, 0, cfg.MaxSize),
		lastFlush:     time.Now(),
		stopCh:        make(chan struct{}),
	}
}

// WriteEventLog adds an event log record to the batch
func (w *ClickHouseWriter) WriteEventLog(ctx context.Context, record *domain.EventLogRecord) error {
	// CRITICAL: Make a copy of the record to avoid race conditions
	recordCopy := *record
	
	w.eventLogBatch = append(w.eventLogBatch, &recordCopy)
	batchSize := len(w.eventLogBatch)
	
	// Check if batch is full or timeout reached
	if batchSize >= w.cfg.MaxSize || time.Since(w.lastFlush).Milliseconds() >= w.cfg.FlushTimeout {
		// CRITICAL: Create a snapshot of the batch to avoid modification during flush
		batchSnapshot := make([]*domain.EventLogRecord, batchSize)
		copy(batchSnapshot, w.eventLogBatch)
		
		// Clear the original batch before flushing to prevent new records from being added
		w.eventLogBatch = w.eventLogBatch[:0]
		
		// Flush the snapshot
		return w.flushEventLogSnapshot(ctx, batchSnapshot)
	}
	
	return nil
}

// WriteTechLog adds a tech log record to the batch
func (w *ClickHouseWriter) WriteTechLog(ctx context.Context, record *domain.TechLogRecord) error {
	// CRITICAL: Make a copy of the record to avoid race conditions
	recordCopy := *record

	w.techLogBatch = append(w.techLogBatch, &recordCopy)
	batchSize := len(w.techLogBatch)

	// Check if batch is full or timeout reached
	if batchSize >= w.cfg.MaxSize || time.Since(w.lastFlush).Milliseconds() >= w.cfg.FlushTimeout {
		// CRITICAL: Create a snapshot of the batch to avoid modification during flush
		batchSnapshot := make([]*domain.TechLogRecord, batchSize)
		copy(batchSnapshot, w.techLogBatch)

		// Clear the original batch before flushing to prevent new records from being added
		w.techLogBatch = w.techLogBatch[:0]

		// Flush the snapshot
		return w.flushTechLogSnapshot(ctx, batchSnapshot)
	}

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
func (w *ClickHouseWriter) WriteParserMetrics(ctx context.Context, metrics *domain.ParserMetrics) error {
	batch, err := w.conn.PrepareBatch(ctx, "INSERT INTO logs.parser_metrics")
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
		metrics.FilesProcessed,
		metrics.RecordsParsed,
		metrics.ParsingTimeMs,
		metrics.RecordsPerSecond,
		metrics.StartTime,
		metrics.EndTime,
		metrics.ErrorCount,
	)
	if err != nil {
		return fmt.Errorf("failed to append to batch: %w", err)
	}
	
	if err := batch.Send(); err != nil {
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

// Close flushes and closes the writer
func (w *ClickHouseWriter) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	return w.Flush(ctx)
}

// flushEventLog writes event log batch to ClickHouse with deduplication
func (w *ClickHouseWriter) flushEventLog(ctx context.Context) error {
	if len(w.eventLogBatch) == 0 {
		return nil
	}
	
	// CRITICAL: Create a snapshot to avoid modification during processing
	batchSnapshot := make([]*domain.EventLogRecord, len(w.eventLogBatch))
	copy(batchSnapshot, w.eventLogBatch)
	
	// Clear the original batch
	w.eventLogBatch = w.eventLogBatch[:0]
	
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
	
	// Calculate hashes and filter duplicates (if enabled)
	recordsToWrite := make([]*domain.EventLogRecord, 0, len(batchSnapshot))
	hashes := make([]string, 0, len(batchSnapshot))
	
	skippedCount := 0
	skippedReasons := make(map[string]int)
	skippedRecords := make([]map[string]interface{}, 0)
	processedCount := 0
	
	// CRITICAL: Store batch size at start
	initialBatchSize := len(batchSnapshot)
	
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
		
		// Check if hash already exists in ClickHouse (only if deduplication is enabled)
		if w.cfg.EnableDeduplication {
			exists, err := w.checkHashExists(ctx, "event_log", hash)
			if err != nil {
				log.Warn().Err(err).Str("hash", hash).Msg("Failed to check hash existence, will try to insert")
				// Continue with insert - if it's a duplicate, ClickHouse will handle it
			} else if exists {
				skippedCount++
				skippedReasons["duplicate"]++
				skippedInfo := map[string]interface{}{
					"index":           idx,
					"reason":          "duplicate",
					"hash":            hash,
					"event_time":      record.EventTime.Format("2006-01-02 15:04:05"),
					"transaction_datetime": record.TransactionDateTime.Format("2006-01-02 15:04:05"),
					"event":           record.Event,
					"level":           record.Level,
				}
				skippedRecords = append(skippedRecords, skippedInfo)
				log.Warn().
					Int("record_index", idx).
					Str("hash", hash).
					Str("event_time", record.EventTime.Format("2006-01-02 15:04:05")).
					Str("event", record.Event).
					Msg("CRITICAL: Skipping duplicate event log record - RECORD WILL BE LOST")
				continue
			}
		}
		
		// Record passed all checks - add to batch
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
		return nil
	}
	
	batch, err := w.conn.PrepareBatch(ctx, "INSERT INTO logs.event_log")
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
	
	if err := batch.Send(); err != nil {
		// CRITICAL: Log detailed information about the failed batch
		log.Error().
			Err(err).
			Int("total_in_batch", len(w.eventLogBatch)).
			Int("records_to_write", len(recordsToWrite)).
			Msg("CRITICAL: Failed to send batch to ClickHouse - ALL RECORDS IN BATCH WILL BE LOST")
		return fmt.Errorf("failed to send batch (total=%d, to_write=%d): %w", len(w.eventLogBatch), len(recordsToWrite), err)
	}
	
	// Calculate total time including ClickHouse write
	totalTime := time.Since(startTime)
	
	logEntry := log.Info().
		Int("total", len(batchSnapshot)).
		Int("written", len(recordsToWrite)).
		Dur("total_time_ms", totalTime).
		Float64("records_per_second", float64(len(recordsToWrite))/totalTime.Seconds())
	if w.cfg.EnableDeduplication {
		logEntry = logEntry.Int("duplicates", len(batchSnapshot)-len(recordsToWrite))
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
	if len(w.techLogBatch) == 0 {
		return nil
	}

	// CRITICAL: Create a snapshot to avoid modification during processing
	batchSnapshot := make([]*domain.TechLogRecord, len(w.techLogBatch))
	copy(batchSnapshot, w.techLogBatch)

	// Clear the original batch
	w.techLogBatch = w.techLogBatch[:0]

	return w.flushTechLogSnapshot(ctx, batchSnapshot)
}

// flushTechLogSnapshot processes a snapshot of the batch (thread-safe)
func (w *ClickHouseWriter) flushTechLogSnapshot(ctx context.Context, batchSnapshot []*domain.TechLogRecord) error {
	if len(batchSnapshot) == 0 {
		return nil
	}

	// Calculate hashes and filter duplicates (if enabled)
	recordsToWrite := make([]*domain.TechLogRecord, 0, len(batchSnapshot))
	hashes := make([]string, 0, len(batchSnapshot))

	for _, record := range batchSnapshot {
		hash, err := calculateTechLogHash(record)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to calculate hash, skipping record")
			continue
		}
		
		// Check if hash already exists in ClickHouse (only if deduplication is enabled)
		if w.cfg.EnableDeduplication {
			exists, err := w.checkHashExists(ctx, "tech_log", hash)
			if err != nil {
				log.Warn().Err(err).Str("hash", hash).Msg("Failed to check hash existence, will try to insert")
				// Continue with insert - if it's a duplicate, ClickHouse will handle it
			} else if exists {
				log.Debug().Str("hash", hash).Msg("Skipping duplicate tech log record")
				continue
			}
		}
		
		recordsToWrite = append(recordsToWrite, record)
		hashes = append(hashes, hash)
	}
	
	if len(recordsToWrite) == 0 {
		if w.cfg.EnableDeduplication {
			log.Debug().Int("total", len(batchSnapshot)).Msg("All tech log records were duplicates, skipping batch")
		}
		return nil
	}
	
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
			record.InfobaseGUID,
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
		)
		if err != nil {
			return fmt.Errorf("failed to append to batch: %w", err)
		}
	}
	
	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}
	
	logEntry := log.Info().
		Int("total", len(batchSnapshot)).
		Int("written", len(recordsToWrite))
	if w.cfg.EnableDeduplication {
		logEntry = logEntry.Int("duplicates", len(batchSnapshot)-len(recordsToWrite))
	}
	logEntry.Msg("Flushed tech log batch to ClickHouse")

	w.lastFlush = time.Now()

	return nil
}

// checkHashExists checks if a record with given hash already exists in ClickHouse
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

