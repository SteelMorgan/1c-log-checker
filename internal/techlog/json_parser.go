package techlog

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/SteelMorgan/1c-log-checker/internal/domain"
)

// ParseJSONLine parses a single line from JSON-format tech log
// Format: {"ts":"2023-08-02T08:48:05.982000","duration":"46998","name":"SCALL",...}
func ParseJSONLine(line string) (*domain.TechLogRecord, error) {
	if line == "" {
		return nil, fmt.Errorf("empty line")
	}

	// Remove BOM (Byte Order Mark) if present
	// UTF-8 BOM is "\ufeff" (EF BB BF in hex)
	line = strings.TrimPrefix(line, "\ufeff")
	line = strings.TrimPrefix(line, "\uFEFF")

	// Parse JSON into map
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(line), &data); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	
	record := &domain.TechLogRecord{
		RawLine:    line,
		Properties: make(map[string]string),
	}
	
	// Extract core fields
	if ts, ok := data["ts"].(string); ok {
		t, err := time.Parse("2006-01-02T15:04:05.000000", ts)
		if err != nil {
			return nil, fmt.Errorf("invalid timestamp: %w", err)
		}
		record.Timestamp = t
	}
	
	if dur, ok := data["duration"].(string); ok {
		duration, err := parseUint64(dur)
		if err != nil {
			return nil, fmt.Errorf("invalid duration: %w", err)
		}
		record.Duration = duration
	}
	
	if name, ok := data["name"].(string); ok {
		record.Name = name
	}
	
	if level, ok := data["level"].(string); ok {
		record.Level = level
	}
	
	if depth, ok := data["depth"].(string); ok {
		d, err := parseUint8(depth)
		if err == nil {
			record.Depth = d
		}
	}
	
	if process, ok := data["process"].(string); ok {
		record.Process = process
	}
	
	if osThread, ok := data["OSThread"].(string); ok {
		val, err := parseUint32(osThread)
		if err == nil {
			record.OSThread = val
		}
	}
	
	if clientID, ok := data["ClientID"].(string); ok {
		val, err := parseUint64(clientID)
		if err == nil {
			record.ClientID = val
		}
	}
	
	if sessionID, ok := data["SessionID"].(string); ok {
		record.SessionID = sessionID
	}
	
	if transID, ok := data["Trans"].(string); ok {
		record.TransactionID = transID
	} else if transID, ok := data["TransactionID"].(string); ok {
		record.TransactionID = transID
	}
	
	if usr, ok := data["Usr"].(string); ok {
		record.User = usr
	}
	
	if appID, ok := data["AppID"].(string); ok {
		record.ApplicationID = appID
	}
	
	if connID, ok := data["ConnID"].(string); ok {
		val, err := parseUint64(connID)
		if err == nil {
			record.ConnectionID = val
		}
	}
	
	if iface, ok := data["Interface"].(string); ok {
		record.Interface = iface
	}
	
	if method, ok := data["Method"].(string); ok {
		record.Method = method
	}
	
	if callID, ok := data["CallID"].(string); ok {
		val, err := parseUint64(callID)
		if err == nil {
			record.CallID = val
		}
	}
	
	// Extract all other fields using setRecordProperty for consistency
	// This ensures all fields are mapped correctly to struct fields
	coreFields := map[string]bool{
		"ts": true, "duration": true, "name": true, "level": true,
		"depth": true, "process": true, "OSThread": true,
	}
	
	for key, val := range data {
		if !coreFields[key] {
			// Convert to string and use setRecordProperty for mapping
			valStr := fmt.Sprintf("%v", val)
			setRecordProperty(record, key, valStr)
		}
	}
	
	return record, nil
}

// Helper functions for parsing numbers from strings
func parseUint64(s string) (uint64, error) {
	return strconv.ParseUint(s, 10, 64)
}

func parseUint32(s string) (uint32, error) {
	val, err := strconv.ParseUint(s, 10, 32)
	return uint32(val), err
}

func parseUint8(s string) (uint8, error) {
	val, err := strconv.ParseUint(s, 10, 8)
	return uint8(val), err
}

