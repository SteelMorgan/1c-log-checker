package eventlog

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/1c-log-checker/internal/domain"
	"github.com/rs/zerolog/log"
)

// Reader reads 1C Event Log files (.lgf/.lgp)
type Reader struct {
	basePath      string
	lgfPath       string
	lgpFiles      []string
	clusterGUID   string
	clusterName   string
	infobaseGUID  string
	infobaseName  string
	lgfReader     *LgfReader // For resolving user_id, computer_id, etc.
	maxWorkers    int        // Max parallel workers for .lgp file processing

	allRecords []*domain.EventLogRecord // All records from all files (loaded in parallel)
	recordIdx  int                      // Current position in allRecords
}

// NewReader creates a new event log reader
func NewReader(basePath, clusterGUID, infobaseGUID, clusterName, infobaseName string, maxWorkers int) (*Reader, error) {
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
		basePath:     basePath,
		lgfPath:      lgfPath,
		clusterGUID:  clusterGUID,
		clusterName:  clusterName,
		infobaseGUID: infobaseGUID,
		infobaseName: infobaseName,
		lgfReader:    lgfReader,
		maxWorkers:   maxWorkers,
		allRecords:   make([]*domain.EventLogRecord, 0),
	}, nil
}

// Open opens the event log and prepares for reading
func (r *Reader) Open(ctx context.Context) error {
	// Find all .lgp files in the directory
	lgpPattern := filepath.Join(r.basePath, "*.lgp")
	matches, err := filepath.Glob(lgpPattern)
	if err != nil {
		return fmt.Errorf("failed to find lgp files: %w", err)
	}

	r.lgpFiles = matches

	log.Info().
		Str("base_path", r.basePath).
		Int("lgp_count", len(r.lgpFiles)).
		Int("max_workers", r.maxWorkers).
		Msg("Event log reader opened, loading files in parallel")

	// Load all files in parallel
	if err := r.loadAllFilesParallel(ctx); err != nil {
		return fmt.Errorf("failed to load files in parallel: %w", err)
	}

	log.Info().
		Str("base_path", r.basePath).
		Int("total_records", len(r.allRecords)).
		Msg("All event log files loaded")

	return nil
}

// loadAllFilesParallel loads all .lgp files in parallel
func (r *Reader) loadAllFilesParallel(ctx context.Context) error {
	if len(r.lgpFiles) == 0 {
		return nil
	}

	// Create channels for work distribution
	fileChan := make(chan string, len(r.lgpFiles))
	resultChan := make(chan fileResult, len(r.lgpFiles))

	// Start worker pool
	var wg sync.WaitGroup
	for w := 0; w < r.maxWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for filePath := range fileChan {
				// Check context cancellation
				select {
				case <-ctx.Done():
					resultChan <- fileResult{err: ctx.Err()}
					return
				default:
				}

				// Parse file
				file, err := os.Open(filePath)
				if err != nil {
					resultChan <- fileResult{
						filePath: filePath,
						err:      fmt.Errorf("failed to open file: %w", err),
					}
					continue
				}

				parser := NewLgpParser(r.clusterGUID, r.infobaseGUID, r.clusterName, r.infobaseName, r.lgfReader)
				records, err := parser.Parse(file)
				file.Close()

				if err != nil {
					resultChan <- fileResult{
						filePath: filePath,
						err:      fmt.Errorf("failed to parse file: %w", err),
					}
					continue
				}

				resultChan <- fileResult{
					filePath: filePath,
					records:  records,
				}

				log.Debug().
					Str("file", filepath.Base(filePath)).
					Int("worker", workerID).
					Int("records", len(records)).
					Msg("Parsed lgp file")
			}
		}(w)
	}

	// Send files to workers
	for _, filePath := range r.lgpFiles {
		fileChan <- filePath
	}
	close(fileChan)

	// Wait for all workers to finish in a separate goroutine
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	results := make([]fileResult, 0, len(r.lgpFiles))
	for result := range resultChan {
		if result.err != nil {
			log.Warn().
				Err(result.err).
				Str("file", result.filePath).
				Msg("Failed to parse file, skipping")
			continue
		}
		results = append(results, result)
	}

	// Sort results by file path to maintain chronological order
	sort.Slice(results, func(i, j int) bool {
		return results[i].filePath < results[j].filePath
	})

	// Combine all records
	totalRecords := 0
	for _, result := range results {
		totalRecords += len(result.records)
	}
	r.allRecords = make([]*domain.EventLogRecord, 0, totalRecords)
	for _, result := range results {
		r.allRecords = append(r.allRecords, result.records...)
	}

	return nil
}

// fileResult holds the result of parsing a single file
type fileResult struct {
	filePath string
	records  []*domain.EventLogRecord
	err      error
}

// Read reads the next event log record
func (r *Reader) Read(ctx context.Context) (*domain.EventLogRecord, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Return next record from pre-loaded buffer
	if r.recordIdx < len(r.allRecords) {
		record := r.allRecords[r.recordIdx]
		r.recordIdx++
		return record, nil
	}

	return nil, io.EOF
}

// Seek seeks to a specific position (record index)
func (r *Reader) Seek(ctx context.Context, recordIndex int64) error {
	if recordIndex < 0 || int(recordIndex) >= len(r.allRecords) {
		return fmt.Errorf("invalid record index: %d", recordIndex)
	}

	r.recordIdx = int(recordIndex)
	return nil
}

// Close closes the reader
func (r *Reader) Close() error {
	// No files are kept open anymore (all processed in parallel upfront)
	return nil
}

