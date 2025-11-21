package techlog

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/SteelMorgan/1c-log-checker/internal/domain"
	"github.com/SteelMorgan/1c-log-checker/internal/offset"
	"github.com/rs/zerolog/log"
)

// TechLogOffsetStore is an interface for techlog-specific offset operations
type TechLogOffsetStore interface {
	GetTechLogOffset(ctx context.Context, filePath string) (*offset.TechLogOffset, error)
	SaveTechLogOffset(ctx context.Context, offset *offset.TechLogOffset) error
}

// Tailer tails tech log files with support for rotation and compression
type Tailer struct {
	dirPath              string
	isJSON               bool
	offsetStore          TechLogOffsetStore
	currentFile          *os.File
	reader               io.Reader
	scanner              *bufio.Scanner
	lastInode            uint64
	lastSize             int64
	currentOffset        *offset.TechLogOffset
	lineCount            int64
	batchSize            int // Number of records before saving offset
	historyProcessed     bool // Flag to track if historical files have been processed
	stopCh               chan struct{}
	onHistoricalComplete func() // Callback called after historical files processing completes
	maxWorkers           int // Max parallel workers for historical file processing
}

// NewTailer creates a new tech log tailer
func NewTailer(dirPath string, isJSON bool, offsetStore offset.OffsetStore, maxWorkers int) *Tailer {
	var techLogStore TechLogOffsetStore
	// Try to cast to BoltDBStore which implements TechLogOffsetStore
	if boltStore, ok := offsetStore.(*offset.BoltDBStore); ok {
		techLogStore = boltStore
	}

	if maxWorkers <= 0 {
		maxWorkers = 4 // Default
	}

	return &Tailer{
		dirPath:     dirPath,
		isJSON:      isJSON,
		offsetStore: techLogStore,
		batchSize:   500, // Save offset every 500 records
		stopCh:      make(chan struct{}),
		maxWorkers:  maxWorkers,
	}
}

// SetHistoricalCompleteCallback sets a callback to be called when historical files processing completes
func (t *Tailer) SetHistoricalCompleteCallback(callback func()) {
	t.onHistoricalComplete = callback
}

// Start starts tailing the tech log directory
func (t *Tailer) Start(ctx context.Context, handler func(*domain.TechLogRecord) error) error {
	log.Info().
		Str("dir", t.dirPath).
		Bool("json", t.isJSON).
		Msg("Starting tech log tailer")

	// Process historical files first (only once)
	if !t.historyProcessed {
		if err := t.processHistoricalFiles(ctx, handler); err != nil {
			log.Warn().Err(err).Msg("Error processing historical files, continuing with live tailing")
		}
		t.historyProcessed = true

		// Call completion callback if set
		if t.onHistoricalComplete != nil {
			log.Debug().Msg("Calling historical processing completion callback")
			t.onHistoricalComplete()
		}
	}

	// Switch to live tailing mode
	ticker := time.NewTicker(500 * time.Millisecond) // Poll every 500ms for <1s latency
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.stopCh:
			return nil
		case <-ticker.C:
			if err := t.processNewRecords(ctx, handler); err != nil {
				log.Warn().Err(err).Msg("Error processing tech log records")
				// Don't return error, continue tailing
			}
		}
	}
}

// Stop stops the tailer
func (t *Tailer) Stop() error {
	close(t.stopCh)
	if t.currentFile != nil {
		return t.currentFile.Close()
	}
	return nil
}

// processNewRecords processes new records from the latest log file
func (t *Tailer) processNewRecords(ctx context.Context, handler func(*domain.TechLogRecord) error) error {
	// Find the latest log file
	latestFile, err := t.findLatestLogFile()
	if err != nil {
		return fmt.Errorf("failed to find latest log file: %w", err)
	}
	
	if latestFile == "" {
		return nil // No log files yet
	}
	
	// Check if file rotated (different inode or size decreased)
	stat, err := os.Stat(latestFile)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}
	
	// Get inode (simplified - use ModTime as proxy on Windows)
	currentInode := uint64(stat.ModTime().Unix())
	currentSize := stat.Size()
	
	// If file rotated, close current and open new
	if t.currentFile != nil && (currentInode != t.lastInode || currentSize < t.lastSize) {
		log.Info().
			Str("old_file", t.currentFile.Name()).
			Str("new_file", latestFile).
			Msg("Log file rotated, reopening")
		
		t.currentFile.Close()
		t.currentFile = nil
	}
	
	// Open file if needed
	if t.currentFile == nil {
		if err := t.openFile(latestFile); err != nil {
			return err
		}
		t.lastInode = currentInode
		
		// Try to load offset from storage
		if t.offsetStore != nil {
			if storedOffset, err := t.offsetStore.GetTechLogOffset(ctx, latestFile); err == nil && storedOffset != nil {
				// Check if file size is still valid (file might have been rotated)
				if storedOffset.OffsetBytes <= currentSize {
					// Seek to saved offset
					if _, err := t.currentFile.Seek(storedOffset.OffsetBytes, io.SeekStart); err == nil {
						t.lastSize = storedOffset.OffsetBytes
						t.currentOffset = storedOffset
						t.lineCount = storedOffset.LastLine
						log.Info().
							Str("file", latestFile).
							Int64("offset_bytes", storedOffset.OffsetBytes).
							Int64("last_line", storedOffset.LastLine).
							Msg("Resumed from saved offset")
					} else {
						log.Warn().Err(err).Msg("Failed to seek to saved offset, starting from beginning")
						t.lastSize = 0
					}
				} else {
					log.Info().
						Str("file", latestFile).
						Int64("saved_offset", storedOffset.OffsetBytes).
						Int64("file_size", currentSize).
						Msg("File size changed, starting from beginning")
					t.lastSize = 0
				}
			} else {
				t.lastSize = 0 // Start from beginning
			}
		} else {
			t.lastSize = 0 // Start from beginning
		}
	}

	// Extract cluster_guid and infobase_guid from file path
	clusterGUID, infobaseGUID, err := ExtractGUIDsFromPath(latestFile)
	if err != nil {
		log.Warn().
			Err(err).
			Str("file", latestFile).
			Msg("Failed to extract GUIDs from path, records will have empty GUIDs")
		// Continue processing but GUIDs will be empty
		clusterGUID = ""
		infobaseGUID = ""
	} else {
		log.Debug().
			Str("file", latestFile).
			Str("cluster_guid", clusterGUID).
			Str("infobase_guid", infobaseGUID).
			Msg("Extracted GUIDs from file path")
	}

	// Extract timestamp from filename (for hierarchical text format)
	// For JSON format, timestamp is already in the data, so this is only used for text format
	fileTimestamp, err := ExtractTimestampFromFilename(latestFile)
	if err != nil {
		// If we can't extract timestamp from filename, use current time as fallback
		// This is OK for plain text format (has full timestamp) but not ideal for hierarchical
		log.Debug().
			Err(err).
			Str("file", latestFile).
			Msg("Failed to extract timestamp from filename, using current time as fallback")
		fileTimestamp = time.Now()
	}

	// Read new lines
	for t.scanner.Scan() {
		line := t.scanner.Text()
		t.lineCount++

		// Parse the line
		var record *domain.TechLogRecord
		var parseErr error

		if t.isJSON {
			record, parseErr = ParseJSONLine(line)
		} else {
			record, parseErr = ParseTextLine(line, fileTimestamp)
		}

		if parseErr != nil {
			log.Warn().
				Err(parseErr).
				Str("line", line[:min(len(line), 100)]).
				Msg("Failed to parse tech log line, skipping")
			continue
		}

		// Add cluster_guid and infobase_guid to record
		record.ClusterGUID = clusterGUID
		record.InfobaseGUID = infobaseGUID

		// Call handler
		if err := handler(record); err != nil {
			log.Error().
				Err(err).
				Msg("Handler failed")
			// Continue processing
		}
		
		// Save offset after each batch (every batchSize records)
		if t.offsetStore != nil && t.lineCount%int64(t.batchSize) == 0 {
			if err := t.saveOffset(ctx, latestFile, currentSize, record.Timestamp); err != nil {
				log.Warn().Err(err).Msg("Failed to save offset")
			}
		}
	}
	
	if err := t.scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}
	
	// Save offset at the end of processing cycle
	if t.offsetStore != nil {
		if err := t.saveOffset(ctx, latestFile, currentSize, time.Now()); err != nil {
			log.Warn().Err(err).Msg("Failed to save offset at end of cycle")
		}
	}
	
	t.lastSize = currentSize
	return nil
}

// findLatestLogFile finds the most recent .log file in the directory
func (t *Tailer) findLatestLogFile() (string, error) {
	files, err := t.findAllLogFiles()
	if err != nil {
		return "", err
	}
	
	if len(files) == 0 {
		return "", nil
	}
	
	// Return the last file (most recent)
	return files[len(files)-1], nil
}

// findAllLogFiles finds all .log files in the directory (recursively) and sorts them by timestamp
// Returns files sorted chronologically (oldest first)
func (t *Tailer) findAllLogFiles() ([]string, error) {
	var files []fileWithTimestamp

	// Walk directory tree recursively
	err := filepath.WalkDir(t.dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// Log error but continue walking
			log.Warn().Err(err).Str("path", path).Msg("Error accessing path")
			return nil
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Only process .log files
		if !strings.HasSuffix(path, ".log") {
			return nil
		}

		// Skip .zip files
		if strings.HasSuffix(path, ".zip") {
			return nil
		}

		// Try to extract timestamp from filename
		timestamp, err := ExtractTimestampFromFilename(path)
		if err != nil {
			// If we can't extract timestamp, use modification time as fallback
			info, err := d.Info()
			if err != nil {
				log.Warn().Err(err).Str("file", path).Msg("Failed to get file info")
				return nil
			}
			timestamp = info.ModTime()
		}

		files = append(files, fileWithTimestamp{
			Path:      path,
			Timestamp: timestamp,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	// Sort by timestamp (oldest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].Timestamp.Before(files[j].Timestamp)
	})

	// Extract just the paths
	result := make([]string, len(files))
	for i, f := range files {
		result[i] = f.Path
	}

	return result, nil
}

// fileWithTimestamp represents a log file with its timestamp
type fileWithTimestamp struct {
	Path      string
	Timestamp time.Time
}

// processHistoricalFiles processes all historical log files with parallel workers
func (t *Tailer) processHistoricalFiles(ctx context.Context, handler func(*domain.TechLogRecord) error) error {
	log.Info().Str("dir", t.dirPath).Msg("Processing historical log files")

	files, err := t.findAllLogFiles()
	if err != nil {
		return fmt.Errorf("failed to find log files: %w", err)
	}

	if len(files) == 0 {
		log.Info().Str("dir", t.dirPath).Msg("No historical files found")
		return nil
	}

	log.Info().
		Str("dir", t.dirPath).
		Int("files_count", len(files)).
		Int("max_workers", t.maxWorkers).
		Msg("Found historical files to process")

	// Create channels for work distribution
	fileChan := make(chan string, len(files))
	errChan := make(chan error, len(files))
	doneChan := make(chan struct{})

	// Start worker pool
	var wg sync.WaitGroup
	for w := 0; w < t.maxWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for filePath := range fileChan {
				select {
				case <-ctx.Done():
					return
				case <-t.stopCh:
					return
				default:
				}

				log.Debug().
					Str("file", filePath).
					Int("worker", workerID).
					Msg("Worker processing file")

				if err := t.processFile(ctx, filePath, handler); err != nil {
					log.Warn().
						Err(err).
						Str("file", filePath).
						Int("worker", workerID).
						Msg("Failed to process file")
					errChan <- err
				}
			}
		}(w)
	}

	// Send files to workers
	go func() {
		for i, file := range files {
			select {
			case <-ctx.Done():
				close(fileChan)
				return
			case <-t.stopCh:
				close(fileChan)
				return
			case fileChan <- file:
				log.Info().
					Str("file", file).
					Int("file_num", i+1).
					Int("total_files", len(files)).
					Msg("Queued file for processing")
			}
		}
		close(fileChan)
	}()

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	// Collect errors
	var errors []error
	errCollector := func() {
		for err := range errChan {
			errors = append(errors, err)
		}
	}
	go errCollector()

	// Wait for completion or context cancellation
	select {
	case <-doneChan:
		close(errChan)
		// Give error collector time to finish
		time.Sleep(100 * time.Millisecond)
	case <-ctx.Done():
		return ctx.Err()
	case <-t.stopCh:
		return nil
	}

	log.Info().
		Str("dir", t.dirPath).
		Int("files_processed", len(files)).
		Int("errors", len(errors)).
		Msg("Finished processing historical files")

	if len(errors) > 0 {
		return fmt.Errorf("encountered %d errors during processing", len(errors))
	}

	return nil
}

// processFile processes a single log file from start to end (or from offset if exists)
func (t *Tailer) processFile(ctx context.Context, filePath string, handler func(*domain.TechLogRecord) error) error {
	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	
	// Check if file is compressed
	var reader io.Reader = file
	if strings.HasSuffix(filePath, ".gz") || strings.HasSuffix(filePath, ".zip") {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}
	
	// Check for saved offset
	var startOffset int64 = 0
	var lineCount int64 = 0
	if t.offsetStore != nil {
		if storedOffset, err := t.offsetStore.GetTechLogOffset(ctx, filePath); err == nil && storedOffset != nil {
			stat, err := os.Stat(filePath)
			if err == nil && storedOffset.OffsetBytes <= stat.Size() {
				startOffset = storedOffset.OffsetBytes
				lineCount = storedOffset.LastLine
				log.Debug().
					Str("file", filePath).
					Int64("offset", startOffset).
					Int64("line_count", lineCount).
					Msg("Resuming from saved offset")
			}
		}
	}
	
	// Seek to start offset
	if startOffset > 0 {
		if _, err := file.Seek(startOffset, io.SeekStart); err != nil {
			log.Warn().Err(err).Msg("Failed to seek to offset, starting from beginning")
			startOffset = 0
		}
	}
	
	// Create scanner
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 1MB max line
	
	// Extract cluster_guid and infobase_guid from file path
	clusterGUID, infobaseGUID, err := ExtractGUIDsFromPath(filePath)
	if err != nil {
		log.Warn().
			Err(err).
			Str("file", filePath).
			Msg("Failed to extract GUIDs from path")
		clusterGUID = ""
		infobaseGUID = ""
	}
	
	// Extract timestamp from filename
	fileTimestamp, err := ExtractTimestampFromFilename(filePath)
	if err != nil {
		log.Debug().
			Err(err).
			Str("file", filePath).
			Msg("Failed to extract timestamp from filename, using current time")
		fileTimestamp = time.Now()
	}
	
	// Process lines
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.stopCh:
			return nil
		default:
		}
		
		line := scanner.Text()
		lineCount++
		
		// Parse the line
		var record *domain.TechLogRecord
		var parseErr error
		
		if t.isJSON {
			record, parseErr = ParseJSONLine(line)
		} else {
			record, parseErr = ParseTextLine(line, fileTimestamp)
		}
		
		if parseErr != nil {
			log.Warn().
				Err(parseErr).
				Str("line", line[:min(len(line), 100)]).
				Msg("Failed to parse tech log line, skipping")
			continue
		}
		
		// Add cluster_guid and infobase_guid to record
		record.ClusterGUID = clusterGUID
		record.InfobaseGUID = infobaseGUID
		
		// Call handler
		if err := handler(record); err != nil {
			log.Error().
				Err(err).
				Msg("Handler failed")
			// Continue processing
		}
		
		// Save offset after each batch
		if t.offsetStore != nil && lineCount%int64(t.batchSize) == 0 {
			currentPos, _ := file.Seek(0, io.SeekCurrent)
			offset := &offset.TechLogOffset{
				FilePath:      filePath,
				OffsetBytes:   currentPos,
				LastTimestamp: record.Timestamp,
				LastLine:      lineCount,
			}
			if err := t.offsetStore.SaveTechLogOffset(ctx, offset); err != nil {
				log.Warn().Err(err).Msg("Failed to save offset")
			}
		}
	}
	
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}
	
	// Save final offset
	if t.offsetStore != nil {
		stat, err := os.Stat(filePath)
		if err == nil {
			offset := &offset.TechLogOffset{
				FilePath:      filePath,
				OffsetBytes:   stat.Size(),
				LastTimestamp: time.Now(),
				LastLine:      lineCount,
			}
			if err := t.offsetStore.SaveTechLogOffset(ctx, offset); err != nil {
				log.Warn().Err(err).Msg("Failed to save final offset")
			}
		}
	}
	
	return nil
}

// openFile opens a log file (supports .zip compression)
func (t *Tailer) openFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	
	t.currentFile = file
	
	// Check if file is gzip-compressed
	var reader io.Reader = file
	if strings.HasSuffix(filePath, ".gz") || strings.HasSuffix(filePath, ".zip") {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			file.Close()
			return fmt.Errorf("failed to create gzip reader: %w", err)
		}
		reader = gzReader
	}
	
	t.reader = reader
	t.scanner = bufio.NewScanner(reader)
	
	// Increase buffer size for large lines
	buf := make([]byte, 0, 64*1024)
	t.scanner.Buffer(buf, 1024*1024) // 1MB max line
	
	log.Info().
		Str("file", filePath).
		Msg("Opened tech log file")
	
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// saveOffset saves the current offset to storage
func (t *Tailer) saveOffset(ctx context.Context, filePath string, fileSize int64, lastTimestamp time.Time) error {
	if t.offsetStore == nil {
		return nil
	}
	
	// Get current file position
	currentPos, err := t.currentFile.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("failed to get current file position: %w", err)
	}
	
	offset := &offset.TechLogOffset{
		FilePath:      filePath,
		OffsetBytes:   currentPos,
		LastTimestamp: lastTimestamp,
		LastLine:      t.lineCount,
	}
	
	if err := t.offsetStore.SaveTechLogOffset(ctx, offset); err != nil {
		return fmt.Errorf("failed to save offset: %w", err)
	}
	
	t.currentOffset = offset
	return nil
}

