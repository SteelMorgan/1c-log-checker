package eventlog

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

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
	
	currentFile *os.File
	currentIdx  int
	records     []*domain.EventLogRecord
	recordIdx   int
}

// NewReader creates a new event log reader
func NewReader(basePath, clusterGUID, infobaseGUID, clusterName, infobaseName string) (*Reader, error) {
	lgfPath := filepath.Join(basePath, "1Cv8.lgf")
	
	// Check if .lgf file exists
	if _, err := os.Stat(lgfPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("lgf file not found: %s", lgfPath)
	}
	
	// Create LGF reader for resolving user_id, computer_id, etc.
	lgfReader := NewLgfReader(lgfPath)
	
	return &Reader{
		basePath:     basePath,
		lgfPath:      lgfPath,
		clusterGUID:  clusterGUID,
		clusterName:  clusterName,
		infobaseGUID: infobaseGUID,
		infobaseName: infobaseName,
		lgfReader:    lgfReader,
		records:      make([]*domain.EventLogRecord, 0),
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
		Msg("Event log reader opened")
	
	return nil
}

// Read reads the next event log record
func (r *Reader) Read(ctx context.Context) (*domain.EventLogRecord, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	
	// If no records loaded, parse next file
	if len(r.records) == 0 || r.recordIdx >= len(r.records) {
		if r.currentFile != nil {
			r.currentFile.Close()
			r.currentFile = nil
		}
		
		if r.currentIdx >= len(r.lgpFiles) {
			return nil, io.EOF
		}
		
		if err := r.parseNextFile(); err != nil {
			return nil, err
		}
		
		// If still no records after parsing, try next file
		if len(r.records) == 0 {
			return r.Read(ctx)
		}
	}
	
	// Return next record
	if r.recordIdx < len(r.records) {
		record := r.records[r.recordIdx]
		r.recordIdx++
		return record, nil
	}
	
	// No more records in current file, try next (but limit recursion depth)
	if r.currentIdx < len(r.lgpFiles) {
		return r.Read(ctx)
	}
	
	return nil, io.EOF
}

// Seek seeks to a specific position (file index)
func (r *Reader) Seek(ctx context.Context, fileIndex int64) error {
	if fileIndex < 0 || int(fileIndex) >= len(r.lgpFiles) {
		return fmt.Errorf("invalid file index: %d", fileIndex)
	}
	
	// Close current file if open
	if r.currentFile != nil {
		r.currentFile.Close()
		r.currentFile = nil
	}
	
	// Reset state
	r.currentIdx = int(fileIndex)
	r.records = make([]*domain.EventLogRecord, 0)
	r.recordIdx = 0
	
	return nil // File will be parsed on next Read()
}

// Close closes the reader
func (r *Reader) Close() error {
	if r.currentFile != nil {
		return r.currentFile.Close()
	}
	return nil
}

// parseNextFile parses the next .lgp file
func (r *Reader) parseNextFile() error {
	if r.currentIdx >= len(r.lgpFiles) {
		return io.EOF
	}
	
	filePath := r.lgpFiles[r.currentIdx]
	
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open lgp file: %w", err)
	}
	
	r.currentFile = file
	
	// Create parser and parse file (pass LGF reader for resolution)
	parser := NewLgpParser(r.clusterGUID, r.infobaseGUID, r.clusterName, r.infobaseName, r.lgfReader)
	records, err := parser.Parse(file)
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to parse lgp file: %w", err)
	}
	
	r.records = records
	r.recordIdx = 0
	r.currentIdx++
	
	log.Info().
		Str("file", filepath.Base(filePath)).
		Int("index", r.currentIdx-1).
		Int("total_files", len(r.lgpFiles)).
		Int("records", len(records)).
		Msg("Parsed lgp file")
	
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

