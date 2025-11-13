package techlog

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/1c-log-checker/internal/domain"
)

// ParseTextLine parses a single line from text-format tech log
// Supports both hierarchical (mm:ss.micro-dur) and plain (ISO timestamp) formats
func ParseTextLine(line string) (*domain.TechLogRecord, error) {
	if line == "" {
		return nil, fmt.Errorf("empty line")
	}
	
	record := &domain.TechLogRecord{
		RawLine:    line,
		Properties: make(map[string]string),
	}
	
	// Determine format by checking for ISO timestamp
	isPlainFormat := strings.Contains(line, "T") && strings.Contains(line[0:25], "-")
	
	var remainder string
	var err error
	
	if isPlainFormat {
		// Plain format: 2023-08-01T15:01:45.259000-14998,SCALL,...
		remainder, err = parsePlainTimestamp(line, record)
	} else {
		// Hierarchical format: 45:31.831006-1,SCALL,...
		remainder, err = parseHierarchicalTimestamp(line, record)
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
	ts, err := time.Parse("2006-01-02T15:04:05.000000", tsStr)
	if err != nil {
		return "", fmt.Errorf("invalid timestamp: %w", err)
	}
	record.Timestamp = ts
	
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
// Note: Full date comes from file name (YYYYMMDD)
func parseHierarchicalTimestamp(line string, record *domain.TechLogRecord) (string, error) {
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
	
	// TODO: Extract hour from filename (YYYYMMDDHH)
	// For now, use current date as base
	now := time.Now()
	record.Timestamp = time.Date(
		now.Year(), now.Month(), now.Day(),
		0, minutes, seconds, microsec*1000,
		time.Local,
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

// setRecordProperty sets a property in the record, handling known core fields
func setRecordProperty(record *domain.TechLogRecord, key, value string) {
	switch key {
	case "level":
		record.Level = value
	case "process":
		record.Process = value
	case "OSThread":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.OSThread = uint32(val)
		}
	case "ClientID":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.ClientID = val
		}
	case "SessionID":
		record.SessionID = value
	case "Trans", "TransactionID":
		record.TransactionID = value
	case "Usr":
		record.User = value
	case "AppID":
		record.ApplicationID = value
	case "ConnID":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.ConnectionID = val
		}
	case "Interface":
		record.Interface = value
	case "Method":
		record.Method = value
	case "CallID":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.CallID = val
		}
	default:
		// Store in dynamic properties
		record.Properties[key] = value
	}
}

