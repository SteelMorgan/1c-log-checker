package writer

import (
	"context"

	"github.com/SteelMorgan/1c-log-checker/internal/domain"
)

// BatchWriter writes records to ClickHouse in batches
type BatchWriter interface {
	// WriteEventLog writes an event log record to the batch
	WriteEventLog(ctx context.Context, record *domain.EventLogRecord) error
	
	// WriteTechLog writes a tech log record to the batch
	WriteTechLog(ctx context.Context, record *domain.TechLogRecord) error
	
	// WriteParserMetrics writes parser performance metrics to ClickHouse
	// For event_log: writes metrics for each file (FilesProcessed=1 per record)
	// For tech_log: writes aggregated metrics
	WriteParserMetrics(ctx context.Context, metrics *domain.ParserMetrics) error
	
	// WriteFileReadingProgress writes file reading progress to ClickHouse
	// Mirrors offsets from BoltDB but with additional metadata for monitoring
	WriteFileReadingProgress(ctx context.Context, progress *domain.FileReadingProgress) error
	
	// Flush forces writing all pending records to ClickHouse
	Flush(ctx context.Context) error
	
	// Close flushes pending records and closes the writer
	Close() error
}

// BatchConfig configures batch behavior
type BatchConfig struct {
	MaxSize            int   // Maximum records per batch
	FlushTimeout       int64 // Maximum milliseconds to wait before flush
	EnableDeduplication bool // Enable deduplication check (slower but prevents duplicates)
}

