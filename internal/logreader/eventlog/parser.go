package eventlog

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/1c-log-checker/internal/domain"
	"github.com/google/uuid"
)

// ParseEventLogLine parses a single line from .lgp file
// Format is based on 1C Event Log structure (tab-separated or custom format)
// TODO: Implement actual parsing based on .lgp file format documentation
func ParseEventLogLine(line string) (*domain.EventLogRecord, error) {
	if line == "" {
		return nil, fmt.Errorf("empty line")
	}
	
	// Split by tabs or commas (need to determine actual format from .lgp files)
	fields := strings.Split(line, "\t")
	
	if len(fields) < 10 {
		return nil, fmt.Errorf("insufficient fields: %d", len(fields))
	}
	
	record := &domain.EventLogRecord{
		Properties: make(map[string]string),
	}
	
	// Parse timestamp (field 0)
	// Format example: "13.11.2025 14:42:28" or ISO format
	eventTime, err := parseTimestamp(fields[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %w", err)
	}
	record.EventTime = eventTime
	record.EventDate = time.Date(eventTime.Year(), eventTime.Month(), eventTime.Day(), 0, 0, 0, 0, eventTime.Location())
	
	// Parse other fields based on position
	// TODO: Map fields according to actual .lgp format
	// This is a placeholder implementation
	
	if len(fields) > 1 {
		record.Level = fields[1]
	}
	if len(fields) > 2 {
		record.Event = fields[2]
	}
	if len(fields) > 3 {
		record.UserName = fields[3]
	}
	if len(fields) > 4 {
		record.Computer = fields[4]
	}
	if len(fields) > 5 {
		record.Application = fields[5]
	}
	if len(fields) > 6 {
		sessionID, _ := strconv.ParseUint(fields[6], 10, 64)
		record.SessionID = sessionID
	}
	if len(fields) > 7 {
		record.MetadataName = fields[7]
	}
	if len(fields) > 8 {
		record.Comment = fields[8]
	}
	
	return record, nil
}

// parseTimestamp parses various timestamp formats from event log
func parseTimestamp(s string) (time.Time, error) {
	// Try different formats
	formats := []string{
		"02.01.2006 15:04:05",          // DD.MM.YYYY HH:MM:SS
		"2006-01-02T15:04:05",          // ISO format
		"2006-01-02 15:04:05",          // YYYY-MM-DD HH:MM:SS
		"02.01.2006 15:04:05.000000",   // With microseconds
		"2006-01-02T15:04:05.000000",   // ISO with microseconds
	}
	
	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}
	
	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", s)
}

// parseUUID parses a UUID string
func parseUUID(s string) (uuid.UUID, error) {
	if s == "" {
		return uuid.Nil, nil
	}
	return uuid.Parse(s)
}

