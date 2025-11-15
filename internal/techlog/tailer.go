package techlog

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/1c-log-checker/internal/domain"
	"github.com/rs/zerolog/log"
)

// Tailer tails tech log files with support for rotation and compression
type Tailer struct {
	dirPath     string
	isJSON      bool
	currentFile *os.File
	reader      io.Reader
	scanner     *bufio.Scanner
	lastInode   uint64
	lastSize    int64
	stopCh      chan struct{}
}

// NewTailer creates a new tech log tailer
func NewTailer(dirPath string, isJSON bool) *Tailer {
	return &Tailer{
		dirPath: dirPath,
		isJSON:  isJSON,
		stopCh:  make(chan struct{}),
	}
}

// Start starts tailing the tech log directory
func (t *Tailer) Start(ctx context.Context, handler func(*domain.TechLogRecord) error) error {
	log.Info().
		Str("dir", t.dirPath).
		Bool("json", t.isJSON).
		Msg("Starting tech log tailer")
	
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
		t.lastSize = 0 // Start from beginning
	}

	// Extract cluster_guid and infobase_guid from file path
	clusterGUID, infobaseGUID, err := extractGUIDsFromPath(latestFile)
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

	// Read new lines
	for t.scanner.Scan() {
		line := t.scanner.Text()

		// Parse the line
		var record *domain.TechLogRecord
		var parseErr error

		if t.isJSON {
			record, parseErr = ParseJSONLine(line)
		} else {
			record, parseErr = ParseTextLine(line)
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
	}
	
	if err := t.scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}
	
	t.lastSize = currentSize
	return nil
}

// findLatestLogFile finds the most recent .log file in the directory
func (t *Tailer) findLatestLogFile() (string, error) {
	pattern := filepath.Join(t.dirPath, "*.log")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	
	if len(matches) == 0 {
		return "", nil
	}
	
	// Find newest file by modification time
	var latestFile string
	var latestTime time.Time
	
	for _, file := range matches {
		// Skip .zip files
		if strings.HasSuffix(file, ".zip") {
			continue
		}
		
		stat, err := os.Stat(file)
		if err != nil {
			continue
		}
		
		if latestFile == "" || stat.ModTime().After(latestTime) {
			latestFile = file
			latestTime = stat.ModTime()
		}
	}
	
	return latestFile, nil
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

