package techlog

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/SteelMorgan/1c-log-checker/internal/domain"
)

// ParseTextLine parses a single line from text-format tech log
// Supports both hierarchical (mm:ss.micro-dur) and plain (ISO timestamp) formats
// fileTimestamp is used for hierarchical format to extract full date from filename
func ParseTextLine(line string, fileTimestamp time.Time) (*domain.TechLogRecord, error) {
	if line == "" {
		return nil, fmt.Errorf("empty line")
	}
	
	record := &domain.TechLogRecord{
		RawLine:    line,
		Properties: make(map[string]string),
	}
	
	// Determine format by checking for ISO timestamp
	// Plain format: 2023-08-01T15:01:45.259000-14998,...
	// Hierarchical: 45:31.831006-1,...
	isPlainFormat := len(line) > 20 && strings.Contains(line[0:20], "T") && strings.Contains(line[0:10], "-")
	
	var remainder string
	var err error
	
	if isPlainFormat {
		// Plain format: 2023-08-01T15:01:45.259000-14998,SCALL,...
		remainder, err = parsePlainTimestamp(line, record)
	} else {
		// Hierarchical format: 45:31.831006-1,SCALL,...
		remainder, err = parseHierarchicalTimestamp(line, record, fileTimestamp)
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %w", err)
	}
	
	// Parse the rest: EVENT,depth,properties...
	parts := strings.SplitN(remainder, ",", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid format: insufficient parts")
	}
	
	// Event name
	record.Name = parts[0]
	
	// Depth
	depth, err := strconv.ParseUint(parts[1], 10, 8)
	if err != nil {
		return nil, fmt.Errorf("invalid depth: %w", err)
	}
	record.Depth = uint8(depth)
	
	// Properties (key=value pairs separated by commas)
	// Note: record.Name is already set, so setRecordProperty can use it for context
	if err := parseProperties(parts[2], record); err != nil {
		return nil, fmt.Errorf("failed to parse properties: %w", err)
	}
	
	return record, nil
}

// parsePlainTimestamp parses plain format timestamp
// Format: 2023-08-01T15:01:45.259000-14998
func parsePlainTimestamp(line string, record *domain.TechLogRecord) (string, error) {
	// Find first comma after timestamp
	commaIdx := strings.Index(line, ",")
	if commaIdx == -1 {
		return "", fmt.Errorf("no comma found")
	}
	
	timestampPart := line[:commaIdx]
	
	// Split by last dash (duration separator)
	lastDash := strings.LastIndex(timestampPart, "-")
	if lastDash == -1 {
		return "", fmt.Errorf("no duration separator")
	}
	
	tsStr := timestampPart[:lastDash]
	durStr := timestampPart[lastDash+1:]
	
	// Parse timestamp (ISO format)
	// 1C stores timestamps in local timezone (MSK for Russian installations)
	// We parse as UTC to match ClickHouse storage (same as Event Log parsing)
	// Note: time.Parse without timezone returns UTC by default
	ts, err := time.Parse("2006-01-02T15:04:05.000000", tsStr)
	if err != nil {
		return "", fmt.Errorf("invalid timestamp: %w", err)
	}
	// Ensure UTC timezone (time.Parse returns UTC by default, but be explicit)
	record.Timestamp = ts.UTC()
	
	// Parse duration (microseconds)
	duration, err := strconv.ParseUint(durStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid duration: %w", err)
	}
	record.Duration = duration
	
	return line[commaIdx+1:], nil
}

// parseHierarchicalTimestamp parses hierarchical format timestamp
// Format: 45:31.831006-1
// Note: Full date comes from file name (yymmddhh)
// fileTimestamp should be extracted from filename using ExtractTimestampFromFilename
func parseHierarchicalTimestamp(line string, record *domain.TechLogRecord, fileTimestamp time.Time) (string, error) {
	// Find first comma
	commaIdx := strings.Index(line, ",")
	if commaIdx == -1 {
		return "", fmt.Errorf("no comma found")
	}
	
	timestampPart := line[:commaIdx]
	
	// Split by dash (duration separator)
	parts := strings.Split(timestampPart, "-")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid hierarchical timestamp format")
	}
	
	// Parse time portion: mm:ss.microsec
	timeParts := strings.Split(parts[0], ":")
	if len(timeParts) != 2 {
		return "", fmt.Errorf("invalid time format")
	}
	
	minutes, err := strconv.Atoi(timeParts[0])
	if err != nil {
		return "", fmt.Errorf("invalid minutes: %w", err)
	}
	
	secParts := strings.Split(timeParts[1], ".")
	if len(secParts) != 2 {
		return "", fmt.Errorf("invalid seconds format")
	}
	
	seconds, err := strconv.Atoi(secParts[0])
	if err != nil {
		return "", fmt.Errorf("invalid seconds: %w", err)
	}
	
	microsec, err := strconv.Atoi(secParts[1])
	if err != nil {
		return "", fmt.Errorf("invalid microseconds: %w", err)
	}
	
	// Use timestamp from filename (extracted from yymmddhh pattern)
	// fileTimestamp already has year, month, day, hour set
	// 1C stores timestamps in local timezone (MSK for Russian installations)
	// We parse as UTC to match ClickHouse storage (same as Event Log parsing)
	record.Timestamp = time.Date(
		fileTimestamp.Year(), fileTimestamp.Month(), fileTimestamp.Day(),
		fileTimestamp.Hour(), minutes, seconds, microsec*1000,
		time.UTC, // Use UTC to match Event Log parsing behavior
	)
	
	// Parse duration
	duration, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid duration: %w", err)
	}
	record.Duration = duration
	
	return line[commaIdx+1:], nil
}

// parseProperties parses key=value properties from tech log line
// Handles quoted values with escaped quotes
func parseProperties(propsStr string, record *domain.TechLogRecord) error {
	// Properties are comma-separated: key=value,key2=value2,...
	// Values can be quoted if they contain commas or quotes
	
	i := 0
	for i < len(propsStr) {
		// Find key
		eqIdx := strings.Index(propsStr[i:], "=")
		if eqIdx == -1 {
			break
		}
		
		key := strings.TrimSpace(propsStr[i : i+eqIdx])
		i += eqIdx + 1
		
		// Find value (may be quoted)
		var value string
		if i < len(propsStr) && (propsStr[i] == '"' || propsStr[i] == '\'') {
			// Quoted value
			quote := propsStr[i]
			i++
			start := i
			
			for i < len(propsStr) {
				if propsStr[i] == quote {
					// Check if doubled (escaped)
					if i+1 < len(propsStr) && propsStr[i+1] == quote {
						i += 2
						continue
					}
					// End of quoted value
					value = propsStr[start:i]
					value = strings.ReplaceAll(value, string(quote)+string(quote), string(quote))
					i++
					break
				}
				i++
			}
		} else {
			// Unquoted value (until comma or end)
			start := i
			for i < len(propsStr) && propsStr[i] != ',' {
				i++
			}
			value = strings.TrimSpace(propsStr[start:i])
		}
		
		// Store property
		setRecordProperty(record, key, value)
		
		// Skip comma
		if i < len(propsStr) && propsStr[i] == ',' {
			i++
		}
	}
	
	return nil
}

// setRecordProperty is now in property_mapper.go
// This function is kept for backward compatibility but delegates to the new implementation

