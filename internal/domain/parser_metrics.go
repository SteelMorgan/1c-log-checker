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
	FilesProcessed   uint32
	RecordsParsed    uint64
	ParsingTimeMs    uint64
	RecordsPerSecond float64
	StartTime        time.Time
	EndTime          time.Time
	ErrorCount       uint32
}

