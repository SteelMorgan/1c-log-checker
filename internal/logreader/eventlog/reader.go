package eventlog

import (
	"bufio"
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
	basePath string
	lgfPath  string
	lgpFiles []string
	
	currentFile *os.File
	currentIdx  int
	scanner     *bufio.Scanner
	
	// For deduplication
	seenEvents map[string]bool
}

// NewReader creates a new event log reader
func NewReader(basePath string) (*Reader, error) {
	lgfPath := filepath.Join(basePath, "1Cv8.lgf")
	
	// Check if .lgf file exists
	if _, err := os.Stat(lgfPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("lgf file not found: %s", lgfPath)
	}
	
	return &Reader{
		basePath:   basePath,
		lgfPath:    lgfPath,
		seenEvents: make(map[string]bool),
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
	// If no current file, open the first one
	if r.currentFile == nil {
		if r.currentIdx >= len(r.lgpFiles) {
			return nil, io.EOF
		}
		
		if err := r.openNextFile(); err != nil {
			return nil, err
		}
	}
	
	// Try to read from current file
	for {
		if r.scanner.Scan() {
			line := r.scanner.Text()
			
			// Parse the line
			record, err := ParseEventLogLine(line)
			if err != nil {
				log.Warn().
					Err(err).
					Str("line", line[:min(len(line), 100)]).
					Msg("Failed to parse event log line, skipping")
				continue
			}
			
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
		
		// Check for scanner error
		if err := r.scanner.Err(); err != nil {
			return nil, fmt.Errorf("scanner error: %w", err)
		}
		
		// End of current file, try next
		r.currentFile.Close()
		r.currentFile = nil
		r.currentIdx++
		
		if r.currentIdx >= len(r.lgpFiles) {
			return nil, io.EOF
		}
		
		if err := r.openNextFile(); err != nil {
			return nil, err
		}
	}
}

// Seek seeks to a specific position (file index)
func (r *Reader) Seek(ctx context.Context, fileIndex int) error {
	if fileIndex < 0 || fileIndex >= len(r.lgpFiles) {
		return fmt.Errorf("invalid file index: %d", fileIndex)
	}
	
	// Close current file if open
	if r.currentFile != nil {
		r.currentFile.Close()
		r.currentFile = nil
	}
	
	r.currentIdx = fileIndex
	return r.openNextFile()
}

// Close closes the reader
func (r *Reader) Close() error {
	if r.currentFile != nil {
		return r.currentFile.Close()
	}
	return nil
}

// openNextFile opens the next .lgp file
func (r *Reader) openNextFile() error {
	if r.currentIdx >= len(r.lgpFiles) {
		return io.EOF
	}
	
	filePath := r.lgpFiles[r.currentIdx]
	
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open lgp file: %w", err)
	}
	
	r.currentFile = file
	r.scanner = bufio.NewScanner(file)
	
	// Increase scanner buffer for large lines
	buf := make([]byte, 0, 64*1024)
	r.scanner.Buffer(buf, 1024*1024) // 1MB max line size
	
	log.Debug().
		Str("file", filePath).
		Int("index", r.currentIdx).
		Msg("Opened lgp file")
	
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

