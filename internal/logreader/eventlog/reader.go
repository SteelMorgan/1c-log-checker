package eventlog

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SteelMorgan/1c-log-checker/internal/domain"
	"github.com/SteelMorgan/1c-log-checker/internal/offset"
	"github.com/rs/zerolog/log"
)

// FileMetricsCallback is called after parsing each file with metrics
// Metrics are written to parser_metrics table with parser_type='event_log'
type FileMetricsCallback func(metrics *domain.ParserMetrics) error

// FileProgressCallback is called when file reading progress is updated
// Progress is written to file_reading_progress table for monitoring
type FileProgressCallback func(progress *domain.FileReadingProgress) error

// EventLogOffsetStore is an interface for eventlog-specific offset operations
type EventLogOffsetStore interface {
	GetEventLogOffset(ctx context.Context, filePath string) (*offset.EventLogOffset, error)
	SaveEventLogOffset(ctx context.Context, offset *offset.EventLogOffset) error
}

// Reader reads 1C Event Log files (.lgf/.lgp)
// Uses streaming approach to avoid loading all records into memory
type Reader struct {
	basePath        string
	lgfPath         string
	lgpFiles        []string
	clusterGUID     string
	clusterName     string
	infobaseGUID    string
	infobaseName    string
	lgfReader       *LgfReader          // For resolving user_id, computer_id, etc.
	maxWorkers      int                 // Max parallel workers for .lgp file processing
	metricsCallback FileMetricsCallback // Optional callback for file metrics
	progressCallback FileProgressCallback // Optional callback for file reading progress
	offsetStore     EventLogOffsetStore // Optional offset store for resuming file reading

	// Streaming state
	currentFileIdx int                         // Current file being read
	currentFile    *os.File                    // Current file handle
	currentParser  *LgpParser                  // Current parser
	recordChan     chan *domain.EventLogRecord // Channel for streaming records
	errChan        chan error                  // Channel for errors
	doneChan       chan struct{}               // Channel to signal completion
	ctx            context.Context             // Context for cancellation
}

// NewReader creates a new event log reader
func NewReader(basePath, clusterGUID, infobaseGUID, clusterName, infobaseName string, maxWorkers int) (*Reader, error) {
	return NewReaderWithMetrics(basePath, clusterGUID, infobaseGUID, clusterName, infobaseName, maxWorkers, nil, nil)
}

// NewReaderWithMetrics creates a new event log reader with metrics callback
func NewReaderWithMetrics(basePath, clusterGUID, infobaseGUID, clusterName, infobaseName string, maxWorkers int, metricsCallback FileMetricsCallback, offsetStore EventLogOffsetStore) (*Reader, error) {
	return NewReaderWithMetricsAndProgress(basePath, clusterGUID, infobaseGUID, clusterName, infobaseName, maxWorkers, metricsCallback, nil, offsetStore)
}

// NewReaderWithMetricsAndProgress creates a new event log reader with metrics and progress callbacks
func NewReaderWithMetricsAndProgress(basePath, clusterGUID, infobaseGUID, clusterName, infobaseName string, maxWorkers int, metricsCallback FileMetricsCallback, progressCallback FileProgressCallback, offsetStore EventLogOffsetStore) (*Reader, error) {
	lgfPath := filepath.Join(basePath, "1Cv8.lgf")

	// Check if .lgf file exists
	if _, err := os.Stat(lgfPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("lgf file not found: %s", lgfPath)
	}

	// Create LGF reader for resolving user_id, computer_id, etc.
	lgfReader := NewLgfReader(lgfPath)

	if maxWorkers <= 0 {
		maxWorkers = 4 // Default
	}

	return &Reader{
		basePath:        basePath,
		lgfPath:         lgfPath,
		clusterGUID:     clusterGUID,
		clusterName:     clusterName,
		infobaseGUID:    infobaseGUID,
		infobaseName:    infobaseName,
		lgfReader:       lgfReader,
		maxWorkers:      maxWorkers,
		metricsCallback: metricsCallback,
		progressCallback: progressCallback,
		offsetStore:     offsetStore,
		currentFileIdx:  0,
		recordChan:      make(chan *domain.EventLogRecord, 1000), // Buffered channel for records
		errChan:         make(chan error, 1),
		doneChan:        make(chan struct{}),
	}, nil
}

// Open opens the event log and prepares for reading
// Uses streaming approach - files are read on-demand, not loaded into memory
func (r *Reader) Open(ctx context.Context) error {
	r.ctx = ctx

	// Find all .lgp files in the directory
	lgpPattern := filepath.Join(r.basePath, "*.lgp")
	matches, err := filepath.Glob(lgpPattern)
	if err != nil {
		return fmt.Errorf("failed to find lgp files: %w", err)
	}

	// Sort files by name to maintain chronological order
	sort.Strings(matches)
	r.lgpFiles = matches

	log.Info().
		Str("base_path", r.basePath).
		Int("lgp_count", len(r.lgpFiles)).
		Msg("Event log reader opened, using streaming mode (files will be read on-demand)")

	// Start streaming goroutine
	go r.streamFiles(ctx)

	return nil
}

// streamFiles streams records from all files in parallel using worker pool
// Each file is processed by one worker, but multiple files can be processed simultaneously
// Periodically rescans directory for new files
func (r *Reader) streamFiles(ctx context.Context) {
	defer close(r.recordChan)
	defer close(r.doneChan)

	if len(r.lgpFiles) == 0 {
		return
	}

	// Create worker pool for parallel file processing
	fileChan := make(chan string, len(r.lgpFiles))
	var wg sync.WaitGroup

	// Start workers
	for w := 0; w < r.maxWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for filePath := range fileChan {
				// Check context cancellation
				select {
				case <-ctx.Done():
					return
				default:
				}

				r.processFile(ctx, filePath, workerID)
			}
		}(w)
	}

	// Track processed files to avoid reprocessing
	processedFiles := make(map[string]bool)
	
	// Send initial files to workers
	for _, filePath := range r.lgpFiles {
		select {
		case <-ctx.Done():
			close(fileChan)
			wg.Wait()
			return
		case fileChan <- filePath:
			processedFiles[filePath] = true
		}
	}

	// Periodically rescan directory for new files (every 30 seconds)
	rescanTicker := time.NewTicker(30 * time.Second)
	defer rescanTicker.Stop()
	
	// Wait for all initial workers to finish, then continue monitoring for new files
	initialDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(initialDone)
	}()

	// Monitor for new files
	for {
		select {
		case <-ctx.Done():
			close(fileChan)
			wg.Wait()
			return
		case <-initialDone:
			// Initial files processed, continue monitoring
			initialDone = nil // Prevent re-entering this case
		case <-rescanTicker.C:
			// Rescan directory for new files
			lgpPattern := filepath.Join(r.basePath, "*.lgp")
			matches, err := filepath.Glob(lgpPattern)
			if err != nil {
				log.Warn().Err(err).Str("path", r.basePath).Msg("Failed to rescan directory for new files")
				continue
			}
			
			// Sort files by name
			sort.Strings(matches)
			
			// Find new files
			newFiles := []string{}
			for _, filePath := range matches {
				if !processedFiles[filePath] {
					newFiles = append(newFiles, filePath)
					processedFiles[filePath] = true
				}
			}
			
			if len(newFiles) > 0 {
				log.Info().
					Str("base_path", r.basePath).
					Int("new_files", len(newFiles)).
					Msg("Found new lgp files, adding to processing queue")
				
				// Send new files to workers
				for _, filePath := range newFiles {
					select {
					case <-ctx.Done():
						return
					case fileChan <- filePath:
					}
				}
			}
		}
	}
}

// processFile processes a single file using streaming mode
func (r *Reader) processFile(ctx context.Context, filePath string, workerID int) {
	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		log.Warn().
			Err(err).
			Str("file", filePath).
			Int("worker", workerID).
			Msg("Failed to open file, skipping")
		return
	}
	defer file.Close()

	// Check for saved offset
	var startOffset int64 = 0
	var recordsParsedFromOffset int64 = 0
	if r.offsetStore != nil {
		if storedOffset, err := r.offsetStore.GetEventLogOffset(ctx, filePath); err == nil && storedOffset != nil {
			stat, err := os.Stat(filePath)
			if err == nil && storedOffset.OffsetBytes <= stat.Size() {
				startOffset = storedOffset.OffsetBytes
				recordsParsedFromOffset = storedOffset.RecordsParsed
				log.Info().
					Str("file", filepath.Base(filePath)).
					Int64("offset_bytes", startOffset).
					Int64("records_parsed", recordsParsedFromOffset).
					Time("last_timestamp", storedOffset.LastTimestamp).
					Msg("Resuming from saved offset")
			} else {
				log.Debug().
					Str("file", filepath.Base(filePath)).
					Int64("saved_offset", storedOffset.OffsetBytes).
					Msg("File size changed or stat failed, starting from beginning")
			}
		}
	}

	// Seek to start offset if exists
	if startOffset > 0 {
		if _, err := file.Seek(startOffset, io.SeekStart); err != nil {
			log.Warn().
				Err(err).
				Str("file", filePath).
				Int64("offset", startOffset).
				Msg("Failed to seek to offset, starting from beginning")
			startOffset = 0
			recordsParsedFromOffset = 0
		}
	}

	// Measure file reading time (opening file)
	fileOpenStartTime := time.Now()

	// Parse file using streaming mode (avoids loading all records into memory)
	parsingStartTime := time.Now()
	parser := NewLgpParser(r.clusterGUID, r.infobaseGUID, r.clusterName, r.infobaseName, r.lgfReader)

	// Count records as they are streamed (using atomic counter to avoid race conditions)
	var recordsCount atomic.Uint64
	var lastRecordTimestamp atomic.Value // Store last record timestamp for final offset

	// Create a wrapper channel to count records before forwarding
	countChan := make(chan *domain.EventLogRecord, 1000)
	doneCounting := make(chan struct{})

	// Start goroutine to count and forward records
	go func() {
		defer close(doneCounting)
		for record := range countChan {
			recordsCount.Add(1)
			lastRecordTimestamp.Store(record.EventTime) // Store timestamp for final offset
			select {
			case <-ctx.Done():
				return
			case r.recordChan <- record:
				// Record forwarded
			}
		}
	}()

	// Get file size for progress tracking
	var fileSizeBytes uint64
	if stat, err := os.Stat(filePath); err == nil {
		fileSizeBytes = uint64(stat.Size())
	}
	
	// Track current offset for progress updates (triggered by offsetCallback only)
	lastProgressOffset := atomic.Int64{}
	lastProgressOffset.Store(startOffset)
	
	// Create offset callback for periodic saving
	var offsetCallback func(int64, int64, time.Time) error
	if r.offsetStore != nil {
		offsetCallback = func(currentOffset int64, callbackRecordsCount int64, lastTimestamp time.Time) error {
			// Update tracked offset
			lastProgressOffset.Store(currentOffset)
			
			offset := &offset.EventLogOffset{
				FilePath:      filePath,
				OffsetBytes:   currentOffset,
				LastTimestamp: lastTimestamp,
				RecordsParsed: callbackRecordsCount + recordsParsedFromOffset,
			}
			if err := r.offsetStore.SaveEventLogOffset(ctx, offset); err != nil {
				return err
			}
			
			// Write progress to ClickHouse if callback is provided
			// Always update when offset is saved (every 1000 records)
			if r.progressCallback != nil {
				progress := &domain.FileReadingProgress{
					Timestamp:     time.Now(),
					ParserType:    "event_log",
					ClusterGUID:    r.clusterGUID,
					ClusterName:    r.clusterName,
					InfobaseGUID:   r.infobaseGUID,
					InfobaseName:   r.infobaseName,
					FilePath:       filePath,
					FileName:       filepath.Base(filePath),
					FileSizeBytes:  fileSizeBytes,
					OffsetBytes:    uint64(currentOffset),
					RecordsParsed:  uint64(callbackRecordsCount + recordsParsedFromOffset),
					LastTimestamp:  lastTimestamp,
				}
				if err := r.progressCallback(progress); err != nil {
					log.Warn().Err(err).Str("file", filePath).Msg("Failed to write file reading progress")
				}
			}
			
			// Write incremental metrics to ClickHouse if callback is provided
			// This allows tracking metrics during file reading, not just at the end
			if r.metricsCallback != nil {
				currentTime := time.Now()
				elapsedTime := currentTime.Sub(parsingStartTime)
				totalElapsedTime := currentTime.Sub(fileOpenStartTime)
				elapsedTimeMs := uint64(totalElapsedTime.Milliseconds())
				recordParsingTimeMs := uint64(elapsedTime.Milliseconds()) // Time from parsing start to now
				// In streaming mode: FileReadingTime = TotalTime - RecordParsingTime
				var fileReadingTimeMs uint64
				if elapsedTimeMs > recordParsingTimeMs {
					fileReadingTimeMs = elapsedTimeMs - recordParsingTimeMs
				} else {
					fileReadingTimeMs = 0
				}
				
				// Use atomic recordsCount to get current value (from reader, not callback parameter)
				currentRecordsCount := recordsCount.Load()
				var recordsPerSecond float64
				if elapsedTimeMs > 0 && currentRecordsCount > 0 {
					recordsPerSecond = float64(currentRecordsCount) / (float64(elapsedTimeMs) / 1000.0)
				}
				
				metrics := &domain.ParserMetrics{
					Timestamp:           currentTime,
					ParserType:          "event_log",
					ClusterGUID:         r.clusterGUID,
					ClusterName:         r.clusterName,
					InfobaseGUID:        r.infobaseGUID,
					InfobaseName:        r.infobaseName,
					FilePath:            filePath,
					FileName:            filepath.Base(filePath),
					FilesProcessed:      1,
					RecordsParsed:       currentRecordsCount + uint64(recordsParsedFromOffset),
					ParsingTimeMs:       elapsedTimeMs,
					RecordsPerSecond:    recordsPerSecond,
					StartTime:           fileOpenStartTime,
					EndTime:             currentTime,
					ErrorCount:          0,
					FileReadingTimeMs:   fileReadingTimeMs,
					RecordParsingTimeMs: recordParsingTimeMs,
					// DeduplicationTimeMs and WritingTimeMs will be enriched by writer if available
					DeduplicationTimeMs: 0,
					WritingTimeMs:       0,
				}
				if err := r.metricsCallback(metrics); err != nil {
					log.Warn().Err(err).Str("file", filePath).Msg("Failed to write incremental parser metrics")
				}
			}
			
			return nil
		}
	}

	// Progress updates are now only triggered by offsetCallback (every N records)
	// Removed periodic time-based updates to reduce ClickHouse load
	
	// Parse streamingly - records will be sent to counting channel
	// Pass startOffset to parser so it knows if we're resuming
	err = parser.ParseStream(ctx, file, countChan, offsetCallback, startOffset)
	close(countChan) // Close counting channel to signal completion

	// Wait for counting goroutine to finish
	<-doneCounting

	parsingEndTime := time.Now()
	recordParsingTime := parsingEndTime.Sub(parsingStartTime)

	finalCount := recordsCount.Load()

	readingEndTime := time.Now()
	readingDuration := readingEndTime.Sub(fileOpenStartTime)

	// Save final offset if file was processed successfully (or partially)
	if r.offsetStore != nil {
		// Get current file position (even if there was an error, save progress)
		currentPos, seekErr := file.Seek(0, io.SeekCurrent)
		if seekErr == nil {
			var lastTimestamp time.Time
			if finalCount > 0 {
				// Get last record timestamp from stored value
				if storedTimestamp := lastRecordTimestamp.Load(); storedTimestamp != nil {
					lastTimestamp = storedTimestamp.(time.Time)
				} else {
					lastTimestamp = readingEndTime
				}
			} else {
				lastTimestamp = readingEndTime
			}

			finalOffset := &offset.EventLogOffset{
				FilePath:      filePath,
				OffsetBytes:   currentPos,
				LastTimestamp: lastTimestamp,
				RecordsParsed: int64(finalCount) + recordsParsedFromOffset,
			}
			if saveErr := r.offsetStore.SaveEventLogOffset(ctx, finalOffset); saveErr != nil {
				log.Warn().Err(saveErr).Str("file", filePath).Msg("Failed to save final offset")
			} else {
				log.Debug().
					Str("file", filepath.Base(filePath)).
					Int64("final_offset", currentPos).
					Int64("total_records", finalOffset.RecordsParsed).
					Time("last_timestamp", lastTimestamp).
					Msg("Saved offset for file")
				
				// Write progress to ClickHouse if callback is provided
				if r.progressCallback != nil {
					// Get file size if not already set
					if fileSizeBytes == 0 {
						if stat, err := os.Stat(filePath); err == nil {
							fileSizeBytes = uint64(stat.Size())
						}
					}
					
					progress := &domain.FileReadingProgress{
						Timestamp:     time.Now(),
						ParserType:    "event_log",
						ClusterGUID:    r.clusterGUID,
						ClusterName:    r.clusterName,
						InfobaseGUID:   r.infobaseGUID,
						InfobaseName:   r.infobaseName,
						FilePath:       filePath,
						FileName:       filepath.Base(filePath),
						FileSizeBytes:  fileSizeBytes,
						OffsetBytes:    uint64(currentPos),
						RecordsParsed:  uint64(finalCount) + uint64(recordsParsedFromOffset),
						LastTimestamp:  lastTimestamp,
					}
					if err := r.progressCallback(progress); err != nil {
						log.Warn().Err(err).Str("file", filePath).Msg("Failed to write file reading progress")
					}
				}
			}
		}
	}

	if err != nil {
		log.Warn().
			Err(err).
			Str("file", filePath).
			Int("worker", workerID).
			Msg("Failed to parse file, skipping")
		return
	}

	readingDurationMs := uint64(readingDuration.Milliseconds())
	recordParsingTimeMs := uint64(recordParsingTime.Milliseconds())
	
	// In streaming mode, reading and parsing happen simultaneously
	// FileReadingTime = TotalTime - RecordParsingTime
	// This represents the time spent on I/O operations (reading from disk) vs CPU (parsing records)
	var fileReadingTimeMs uint64
	if readingDurationMs > recordParsingTimeMs {
		fileReadingTimeMs = readingDurationMs - recordParsingTimeMs
	} else {
		// In streaming mode, reading and parsing are concurrent, so they're almost equal
		// Estimate file reading time as a percentage of total time based on I/O overhead
		// For large files, I/O typically takes 10-30% of total time in streaming mode
		// Use conservative estimate: 15% of parsing time for I/O operations
		// For very small files (< 10ms), use minimum 1ms to avoid zero
		if readingDurationMs > 0 {
			estimatedReadingTime := uint64(float64(readingDurationMs) * 0.15)
			if estimatedReadingTime == 0 {
				// For tiny files, use at least 1ms
				fileReadingTimeMs = 1
			} else {
				fileReadingTimeMs = estimatedReadingTime
			}
		} else {
			fileReadingTimeMs = 0
		}
	}

	var recordsPerSecond float64
	if readingDurationMs > 0 && finalCount > 0 {
		recordsPerSecond = float64(finalCount) / (float64(readingDurationMs) / 1000.0)
	}

	// Call metrics callback if provided (final metrics after file completion)
	if r.metricsCallback != nil {
		metrics := &domain.ParserMetrics{
			Timestamp:           time.Now(),
			ParserType:          "event_log",
			ClusterGUID:         r.clusterGUID,
			ClusterName:         r.clusterName,
			InfobaseGUID:        r.infobaseGUID,
			InfobaseName:        r.infobaseName,
			FilePath:            filePath,
			FileName:            filepath.Base(filePath),
			FilesProcessed:      1,
			RecordsParsed:       finalCount,
			ParsingTimeMs:       readingDurationMs, // Total time (file reading + parsing)
			RecordsPerSecond:    recordsPerSecond,
			StartTime:           fileOpenStartTime,
			EndTime:             readingEndTime,
			ErrorCount:          0,
			FileReadingTimeMs:   fileReadingTimeMs,
			RecordParsingTimeMs: recordParsingTimeMs,
			// DeduplicationTimeMs and WritingTimeMs will be set by writer
			DeduplicationTimeMs: 0,
			WritingTimeMs:       0,
		}
		if err := r.metricsCallback(metrics); err != nil {
			log.Warn().
				Err(err).
				Str("file", filePath).
				Int("worker", workerID).
				Msg("Failed to write file metrics")
		}
	}

	log.Debug().
		Str("file", filepath.Base(filePath)).
		Int("worker", workerID).
		Int("total_files", len(r.lgpFiles)).
		Dur("duration", readingDuration).
		Float64("records_per_second", recordsPerSecond).
		Uint64("records", finalCount).
		Msg("Parsed lgp file (streaming mode, parallel)")
}

// Read reads the next event log record from stream
func (r *Reader) Read(ctx context.Context) (*domain.EventLogRecord, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case record, ok := <-r.recordChan:
		if !ok {
			// Channel closed, all records read
			return nil, io.EOF
		}
		return record, nil
	case err := <-r.errChan:
		return nil, err
	}
}

// Seek seeks to a specific position (record index)
// NOTE: In streaming mode, Seek is not supported efficiently
// This would require re-reading all files up to the target index
func (r *Reader) Seek(ctx context.Context, recordIndex int64) error {
	return fmt.Errorf("Seek is not supported in streaming mode (would require re-reading files)")
}

// Close closes the reader
func (r *Reader) Close() error {
	// Close current file if open
	if r.currentFile != nil {
		r.currentFile.Close()
		r.currentFile = nil
	}
	return nil
}
