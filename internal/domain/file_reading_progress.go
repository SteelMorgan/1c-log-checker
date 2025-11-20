package domain

import "time"

// FileReadingProgress represents the current reading progress of a log file
type FileReadingProgress struct {
	Timestamp      time.Time
	ParserType     string    // "event_log" or "tech_log"
	ClusterGUID    string
	ClusterName    string
	InfobaseGUID   string
	InfobaseName   string
	FilePath       string    // Full path to the file
	FileName       string    // Just filename for easier queries
	FileSizeBytes  uint64    // Total file size
	OffsetBytes    uint64    // Current reading position
	RecordsParsed  uint64    // Number of records parsed so far
	LastTimestamp  time.Time // Timestamp of last parsed record
}

