package domain

import "time"

// ParserMetrics represents parser performance metrics
type ParserMetrics struct {
	Timestamp        time.Time
	ParserType       string    // "event_log" or "tech_log"
	ClusterGUID      string
	ClusterName      string
	InfobaseGUID     string
	InfobaseName     string
	FilePath         string    // Full path to the file being processed (for incremental updates)
	FileName         string    // Just filename for easier queries
	FilesProcessed   uint32
	RecordsParsed    uint64
	ParsingTimeMs    uint64    // Total parsing time (file reading + parsing)
	RecordsPerSecond float64
	StartTime        time.Time
	EndTime          time.Time
	ErrorCount       uint32
	
	// Detailed timing breakdown
	FileReadingTimeMs    uint64 // Time spent reading file from disk
	RecordParsingTimeMs  uint64 // Time spent parsing records from file
	DeduplicationTimeMs  uint64 // Time spent checking for duplicates (if enabled)
	WritingTimeMs        uint64 // Time spent writing to ClickHouse
}

