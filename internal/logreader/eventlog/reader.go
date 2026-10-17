package eventlog

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SteelMorgan/1c-log-checker/internal/domain"
	"github.com/SteelMorgan/1c-log-checker/internal/logreader"
	"github.com/SteelMorgan/1c-log-checker/internal/offset"
	"github.com/SteelMorgan/1c-log-checker/internal/workerlimit"
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
	// Совместимость с существующей tail-настройкой; checkpoint теперь безопасен для всех источников.
	UseSafeCheckpoints bool
	basePath           string
	rescanInterval     time.Duration
	nameMu             sync.Mutex
	fileIdentities     sync.Map
	cancel             context.CancelFunc
	lgfPath            string
	lgpFiles           []string
	clusterGUID        string
	clusterName        string
	infobaseGUID       string
	infobaseName       string
	searchDirs         []string
	lgfReader          *LgfReader           // For resolving user_id, computer_id, etc.
	maxWorkers         int                  // Max parallel workers for .lgp file processing
	workerLimiter      *workerlimit.Limiter // Global limiter across all readers/tailers (optional)
	metricsCallback    FileMetricsCallback  // Optional callback for file metrics
	progressCallback   FileProgressCallback // Optional callback for file reading progress
	offsetStore        EventLogOffsetStore  // Optional offset store for resuming file reading
	filesProcessed     atomic.Int32         // Counter for processed files (thread-safe)

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
	return NewReaderWithMetrics(basePath, clusterGUID, infobaseGUID, clusterName, infobaseName, nil, maxWorkers, nil, nil, nil)
}

// NewReaderWithMetrics creates a new event log reader with metrics callback
func NewReaderWithMetrics(basePath, clusterGUID, infobaseGUID, clusterName, infobaseName string, searchDirs []string, maxWorkers int, metricsCallback FileMetricsCallback, offsetStore EventLogOffsetStore, workerLimiter *workerlimit.Limiter) (*Reader, error) {
	return NewReaderWithMetricsAndProgress(basePath, clusterGUID, infobaseGUID, clusterName, infobaseName, searchDirs, maxWorkers, metricsCallback, nil, offsetStore, workerLimiter)
}

// NewReaderWithMetricsAndProgress creates a new event log reader with metrics and progress callbacks
func NewReaderWithMetricsAndProgress(basePath, clusterGUID, infobaseGUID, clusterName, infobaseName string, searchDirs []string, maxWorkers int, metricsCallback FileMetricsCallback, progressCallback FileProgressCallback, offsetStore EventLogOffsetStore, workerLimiter *workerlimit.Limiter) (*Reader, error) {
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
		basePath:         basePath,
		lgfPath:          lgfPath,
		clusterGUID:      clusterGUID,
		clusterName:      clusterName,
		infobaseGUID:     infobaseGUID,
		infobaseName:     infobaseName,
		searchDirs:       searchDirs,
		lgfReader:        lgfReader,
		maxWorkers:       maxWorkers,
		workerLimiter:    workerLimiter,
		metricsCallback:  metricsCallback,
		progressCallback: progressCallback,
		offsetStore:      offsetStore,
		currentFileIdx:   0,
		recordChan:       make(chan *domain.EventLogRecord), // Buffered channel for records
		errChan:          make(chan error, 1),
		doneChan:         make(chan struct{}),
	}, nil
}

func (r *Reader) resolveNames() (clusterName, infobaseName string) {
	r.nameMu.Lock()
	defer r.nameMu.Unlock()
	if r.clusterGUID == "" || r.infobaseGUID == "" || len(r.searchDirs) == 0 {
		return r.clusterName, r.infobaseName
	}

	clusterName, infobaseName, err := logreader.GetClusterAndInfobaseNames(r.clusterGUID, r.infobaseGUID, r.searchDirs)
	if err != nil {
		log.Debug().
			Err(err).
			Str("cluster_guid", r.clusterGUID).
			Str("infobase_guid", r.infobaseGUID).
			Msg("Failed to resolve cluster and infobase names for event_log")
		return r.clusterName, r.infobaseName
	}

	if clusterName != "" {
		r.clusterName = clusterName
	}
	if infobaseName != "" {
		r.infobaseName = infobaseName
	}

	return r.clusterName, r.infobaseName
}

// Open opens the event log and prepares for reading
// Uses streaming approach - files are read on-demand, not loaded into memory
func (r *Reader) Open(ctx context.Context) error {
	ctx, r.cancel = context.WithCancel(ctx)
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

	fileChan := make(chan string, r.maxWorkers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	scheduled := make(map[string]bool)
	// Снимок делается до отправки задания: дописывание во время чтения
	// должно остаться видимым следующему сканированию.
	observed := make(map[string]os.FileInfo)
	defer func() { close(fileChan); wg.Wait() }()
	for w := 0; w < r.maxWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for path := range fileChan {
				if ctx.Err() != nil {
					return
				}
				if err := r.workerLimiter.Acquire(ctx); err != nil {
					return
				}
				r.processFile(ctx, path, workerID)
				r.workerLimiter.Release()
				mu.Lock()
				delete(scheduled, path)
				mu.Unlock()
			}
		}(w)
	}
	scan := func() bool {
		paths, err := filepath.Glob(filepath.Join(r.basePath, "*.lgp"))
		if err != nil {
			log.Warn().Err(err).Msg("Failed to scan LGP files")
			return true
		}
		sort.Strings(paths)
		for _, path := range paths {
			info, err := os.Stat(path)
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			previous, seen := observed[path]
			if seen && os.SameFile(previous, info) && previous.Size() == info.Size() && previous.ModTime().Equal(info.ModTime()) {
				continue
			}
			mu.Lock()
			busy := scheduled[path]
			if !busy {
				scheduled[path] = true
			}
			mu.Unlock()
			if busy {
				continue
			}
			observed[path] = info
			select {
			case <-ctx.Done():
				return false
			case fileChan <- path:
			}
		}
		return true
	}
	if !scan() {
		return
	}
	interval := r.rescanInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !scan() {
				return
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

	// Increment files processed counter at the start of file processing
	// This ensures the counter is accurate during regular metric updates
	r.filesProcessed.Add(1)

	stat, err := file.Stat()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to stat opened LGP")
		return
	}
	var startOffset, recordsParsedFromOffset int64
	var generation uint64
	var initialTail bool
	var legacyCheckpoint bool
	previousInfo, seenFile := r.fileIdentities.LoadOrStore(filePath, stat)
	replaced := seenFile && !os.SameFile(previousInfo.(os.FileInfo), stat)
	r.fileIdentities.Store(filePath, stat)
	if r.offsetStore != nil {
		stored, err := r.offsetStore.GetEventLogOffset(ctx, filePath)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to load LGP checkpoint")
			return
		}
		if stored != nil {
			generation = stored.Generation
			initialTail = stored.InitialTail
			valid := !replaced && stored.OffsetBytes <= stat.Size() && stored.FileSize <= stat.Size()
			if valid && stored.CheckpointHash != "" {
				hash, err := checkpointHash(file, stored.OffsetBytes)
				if err != nil {
					log.Warn().Err(err).Msg("Failed to validate LGP checkpoint")
					return
				}
				valid = hash == stored.CheckpointHash
			}
			if valid {
				startOffset, recordsParsedFromOffset = stored.OffsetBytes, stored.RecordsParsed
				legacyCheckpoint = startOffset > 0 && stored.CheckpointHash == ""
				log.Info().Str("file", filePath).Int64("offset_bytes", startOffset).Msg("Resuming from saved offset")
			} else {
				generation++
				initialTail = false
				log.Info().Str("file", filePath).Uint64("generation", generation).Msg("LGP rotated or truncated, resetting checkpoint")
			}
		}
	}

	if initialTail || legacyCheckpoint {
		next, found, err := findRecordStart(ctx, file, startOffset)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to align initial LGP tail")
			return
		}
		startOffset = next
		initialTail = !found
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

	// Resolve cluster/infobase names once for the entire file
	resolvedClusterName, resolvedInfobaseName := r.resolveNames()

	// Measure file reading time (opening file)
	fileOpenStartTime := time.Now()

	// Parse file using streaming mode (avoids loading all records into memory)
	parsingStartTime := time.Now()
	parser := NewLgpParser(r.clusterGUID, r.infobaseGUID, resolvedClusterName, resolvedInfobaseName, r.lgfReader)

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
				Generation:    generation,
				InitialTail:   initialTail,
				FileSize:      stat.Size(),
				OffsetBytes:   currentOffset,
				LastTimestamp: lastTimestamp,
				RecordsParsed: callbackRecordsCount + recordsParsedFromOffset,
			}
			hash, hashErr := checkpointHash(file, currentOffset)
			if hashErr != nil {
				return hashErr
			}
			offset.CheckpointHash = hash
			if err := r.offsetStore.SaveEventLogOffset(ctx, offset); err != nil {
				return err
			}

			// Write progress to ClickHouse if callback is provided
			// Always update when offset is saved (every 1000 records)
			if r.progressCallback != nil {
				progressClusterName, progressInfobaseName := resolvedClusterName, resolvedInfobaseName
				progress := &domain.FileReadingProgress{
					Timestamp:     time.Now(),
					ParserType:    "event_log",
					ClusterGUID:   r.clusterGUID,
					ClusterName:   progressClusterName,
					InfobaseGUID:  r.infobaseGUID,
					InfobaseName:  progressInfobaseName,
					FilePath:      filePath,
					FileName:      filepath.Base(filePath),
					FileSizeBytes: fileSizeBytes,
					OffsetBytes:   uint64(currentOffset),
					RecordsParsed: uint64(callbackRecordsCount + recordsParsedFromOffset),
					LastTimestamp: lastTimestamp,
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
				currentRecordsCount := uint64(parser.recordsParsed)
				var recordsPerSecond float64
				if elapsedTimeMs > 0 && currentRecordsCount > 0 {
					recordsPerSecond = float64(currentRecordsCount) / (float64(elapsedTimeMs) / 1000.0)
				}

				metricsClusterName, metricsInfobaseName := resolvedClusterName, resolvedInfobaseName
				metrics := &domain.ParserMetrics{
					Timestamp:           currentTime,
					ParserType:          "event_log",
					ClusterGUID:         r.clusterGUID,
					ClusterName:         metricsClusterName,
					InfobaseGUID:        r.infobaseGUID,
					InfobaseName:        metricsInfobaseName,
					FilePath:            filePath,
					FileName:            filepath.Base(filePath),
					FilesProcessed:      uint32(r.filesProcessed.Load()),
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
	err = parser.ParseStream(ctx, file, r.recordChan, offsetCallback, startOffset)

	parsingEndTime := time.Now()
	recordParsingTime := parsingEndTime.Sub(parsingStartTime)

	finalCount := uint64(parser.recordsParsed)

	readingEndTime := time.Now()
	readingDuration := readingEndTime.Sub(fileOpenStartTime)

	// Save final offset if file was processed successfully (or partially)
	if r.offsetStore != nil {
		// Get current file position (even if there was an error, save progress)
		currentPos := parser.checkpoint
		hash, seekErr := checkpointHash(file, currentPos)
		if seekErr == nil {
			var lastTimestamp time.Time
			if finalCount > 0 {
				// Get last record timestamp from stored value
				lastTimestamp = parser.lastTimestamp
			} else {
				lastTimestamp = readingEndTime
			}

			finalOffset := &offset.EventLogOffset{
				Generation:     generation,
				InitialTail:    initialTail,
				CheckpointHash: hash,
				FileSize:       stat.Size(),
				FilePath:       filePath,
				OffsetBytes:    currentPos,
				LastTimestamp:  lastTimestamp,
				RecordsParsed:  int64(finalCount) + recordsParsedFromOffset,
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

					progressClusterName, progressInfobaseName := resolvedClusterName, resolvedInfobaseName
					progress := &domain.FileReadingProgress{
						Timestamp:     time.Now(),
						ParserType:    "event_log",
						ClusterGUID:   r.clusterGUID,
						ClusterName:   progressClusterName,
						InfobaseGUID:  r.infobaseGUID,
						InfobaseName:  progressInfobaseName,
						FilePath:      filePath,
						FileName:      filepath.Base(filePath),
						FileSizeBytes: fileSizeBytes,
						OffsetBytes:   uint64(currentPos),
						RecordsParsed: uint64(finalCount) + uint64(recordsParsedFromOffset),
						LastTimestamp: lastTimestamp,
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
		metricsClusterName, metricsInfobaseName := resolvedClusterName, resolvedInfobaseName
		metrics := &domain.ParserMetrics{
			Timestamp:           time.Now(),
			ParserType:          "event_log",
			ClusterGUID:         r.clusterGUID,
			ClusterName:         metricsClusterName,
			InfobaseGUID:        r.infobaseGUID,
			InfobaseName:        metricsInfobaseName,
			FilePath:            filePath,
			FileName:            filepath.Base(filePath),
			FilesProcessed:      uint32(r.filesProcessed.Load()),
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
	if r.cancel != nil {
		r.cancel()
		<-r.doneChan
	}
	// Close current file if open
	if r.currentFile != nil {
		r.currentFile.Close()
		r.currentFile = nil
	}
	return nil
}

// Хеш небольшого фрагмента перед checkpoint обнаруживает замену файла даже
// после усечения и повторного роста за прежний offset; inode SSHFS нестабилен.
func checkpointHash(file *os.File, position int64) (string, error) {
	if position == 0 {
		return "", nil
	}
	size := int64(64)
	if position < size {
		size = position
	}
	var data [64]byte
	if _, err := file.ReadAt(data[:size], position-size); err != nil {
		return "", err
	}
	sum := sha256.Sum256(data[:size])
	return fmt.Sprintf("%x", sum), nil
}
