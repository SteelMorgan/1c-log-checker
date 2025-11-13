package logreader

import (
	"context"

	"github.com/1c-log-checker/internal/domain"
)

// EventLogReader reads and parses 1C Event Log files (.lgf/.lgp)
type EventLogReader interface {
	// Open opens the event log (path is set during reader creation)
	Open(ctx context.Context) error
	
	// Read reads the next event log record
	// Returns io.EOF when no more records are available
	Read(ctx context.Context) (*domain.EventLogRecord, error)
	
	// Seek seeks to a specific position in the log
	Seek(ctx context.Context, offset int64) error
	
	// Close closes the reader and releases resources
	Close() error
}

// TechLogReader reads and parses 1C Tech Log files (.log)
type TechLogReader interface {
	// Open opens the tech log at the given path
	Open(ctx context.Context, path string) error
	
	// Read reads the next tech log record
	// Returns io.EOF when no more records are available
	Read(ctx context.Context) (*domain.TechLogRecord, error)
	
	// Close closes the reader and releases resources
	Close() error
}

// LogTailer tails log files and handles rotation
type LogTailer interface {
	// Start starts tailing the log file
	Start(ctx context.Context, path string, handler TailHandler) error
	
	// Stop stops tailing
	Stop() error
}

// TailHandler is called for each new log line
type TailHandler interface {
	Handle(ctx context.Context, record interface{}) error
}

