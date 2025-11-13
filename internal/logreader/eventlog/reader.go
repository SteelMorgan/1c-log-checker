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
	basePath     string
	lgfPath      string
	lgpFiles     []string
	clusterGUID  string
	infobaseGUID string
	
	currentFile *os.File
	currentIdx  int
	records     []*domain.EventLogRecord
	recordIdx   int
	
	// For deduplication
	seenEvents map[string]bool
}

// NewReader creates a new event log reader
func NewReader(basePath, clusterGUID, infobaseGUID string) (*Reader, error) {
	lgfPath := filepath.Join(basePath, "1Cv8.lgf")
	
	// Check if .lgf file exists
	if _, err := os.Stat(lgfPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("lgf file not found: %s", lgfPath)
	}
	
	return &Reader{
		basePath:     basePath,
		lgfPath:      lgfPath,
		clusterGUID:  clusterGUID,
		infobaseGUID: infobaseGUID,
		seenEvents:   make(map[string]bool),
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
	}
	
	// Return next record
	for r.recordIdx < len(r.records) {
		record := r.records[r.recordIdx]
		r.recordIdx++
		
		// Check for duplicates
		eventKey := fmt.Sprintf("%s_%d_%d",
			record.EventTime.Format("2006-01-02T15:04:05.000000"),
			record.SessionID,
			record.ConnectionID)
		
		if r.seenEvents[eventKey] {
			continue // Skip duplicate
		}
		
		r.seenEvents[eventKey] = true
		return record, nil
	}
	
	// No more records in current file, try next
	return r.Read(ctx)
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
	
	// Create parser and parse file
	parser := NewLgpParser(r.clusterGUID, r.infobaseGUID)
	records, err := parser.Parse(file)
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to parse lgp file: %w", err)
	}
	
	r.records = records
	r.recordIdx = 0
	r.currentIdx++
	
	log.Debug().
		Str("file", filePath).
		Int("index", r.currentIdx-1).
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

