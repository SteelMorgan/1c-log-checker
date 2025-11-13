package eventlog

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/1c-log-checker/internal/domain"
	"github.com/rs/zerolog/log"
)

// LgpParser parses .lgp files in 1C Event Log format
// Format: 1CV8LOG(ver 2.0)
//         <infobase_guid>
//         {timestamp,level,{transaction},session,user,computer,event,record,app,"comment",data_sep,{metadata},"metadata_pres",...,{props}}
type LgpParser struct {
	infobaseGUID string
	clusterGUID  string
}

// NewLgpParser creates a new parser for .lgp files
func NewLgpParser(clusterGUID, infobaseGUID string) *LgpParser {
	return &LgpParser{
		clusterGUID:  clusterGUID,
		infobaseGUID: infobaseGUID,
	}
}

// Parse reads and parses .lgp file
func (p *LgpParser) Parse(r io.Reader) ([]*domain.EventLogRecord, error) {
	scanner := bufio.NewScanner(r)
	
	var records []*domain.EventLogRecord
	lineNum := 0
	
	// Read header
	if !scanner.Scan() {
		return nil, fmt.Errorf("empty file")
	}
	header := strings.TrimSpace(scanner.Text())
	// Remove BOM (Byte Order Mark) if present
	header = strings.TrimPrefix(header, "\ufeff")
	if !strings.HasPrefix(header, "1CV8LOG") {
		return nil, fmt.Errorf("invalid header: %s", header)
	}
	lineNum++
	
	// Read infobase GUID (if not already set)
	if !scanner.Scan() {
		return nil, fmt.Errorf("missing infobase GUID")
	}
	guid := strings.TrimSpace(scanner.Text())
	if p.infobaseGUID == "" {
		p.infobaseGUID = guid
	}
	lineNum++
	
	// Skip empty line
	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			// This might be the first record, process it
			record, err := p.parseRecord(line)
			if err != nil {
				log.Warn().Err(err).Int("line", lineNum).Msg("Failed to parse record, skipping")
			} else {
				records = append(records, record)
			}
		}
		lineNum++
	}
	
	// Read all records
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		
		// Remove trailing comma if present
		line = strings.TrimSuffix(line, ",")
		
		record, err := p.parseRecord(line)
		if err != nil {
			log.Warn().Err(err).Int("line", lineNum).Str("line_preview", truncate(line, 100)).Msg("Failed to parse record, skipping")
			continue
		}
		
		records = append(records, record)
		lineNum++
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}
	
	log.Info().Int("records", len(records)).Msg("Parsed .lgp file")
	return records, nil
}

// parseRecord parses a single record from .lgp file
// Format: {timestamp,level,{transaction},session,user,computer,event,record,app,"comment",data_sep,{metadata},"metadata_pres",...,{props}}
func (p *LgpParser) parseRecord(line string) (*domain.EventLogRecord, error) {
	// Remove outer braces
	line = strings.TrimPrefix(line, "{")
	line = strings.TrimSuffix(line, "}")
	
	record := &domain.EventLogRecord{
		ClusterGUID:  p.clusterGUID,
		InfobaseGUID: p.infobaseGUID,
		Properties:   make(map[string]string),
	}
	
	// Parse fields using tokenizer that handles nested structures
	tokens, err := tokenizeRecord(line)
	if err != nil {
		return nil, fmt.Errorf("tokenization failed: %w", err)
	}
	
	if len(tokens) < 11 {
		return nil, fmt.Errorf("insufficient tokens: %d", len(tokens))
	}
	
	// Field 0: timestamp (YYYYMMDDHHMMSS)
	timestamp, err := parseLgpTimestamp(tokens[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %w", err)
	}
	record.EventTime = timestamp
	record.EventDate = time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), 0, 0, 0, 0, timestamp.Location())
	
	// Field 1: level (N, C, I, W, E)
	record.Level = mapLgpLevel(tokens[1])
	
	// Field 2: transaction {transaction_id,connection_id}
	transactionID, connectionID, err := parseTransaction(tokens[2])
	if err == nil {
		record.TransactionID = transactionID
		record.ConnectionID = connectionID
	}
	
	// Field 3: session_id
	if sessionID, err := strconv.ParseUint(tokens[3], 10, 64); err == nil {
		record.SessionID = sessionID
	}
	
	// Field 4: user_id (number, not UUID in this format)
	// Field 5: computer_id (number)
	// Field 6: event_id (number)
	// Field 7: record_id (number)
	
	// Field 8: application code
	if len(tokens) > 8 {
		record.Application = mapLgpApplication(tokens[8])
	}
	
	// Field 9: comment (quoted string)
	if len(tokens) > 9 {
		record.Comment = unquoteString(tokens[9])
	}
	
	// Field 10: data_separation
	if len(tokens) > 10 {
		record.DataSeparation = tokens[10]
	}
	
	// Field 11: metadata array {array}
	if len(tokens) > 11 {
		metadata, err := parseMetadataArray(tokens[11])
		if err == nil && len(metadata) > 0 {
			record.MetadataName = metadata[0]
		}
	}
	
	// Field 12: metadata presentation (quoted string)
	if len(tokens) > 12 {
		record.MetadataPresentation = unquoteString(tokens[12])
	}
	
	// Remaining fields: properties and other data
	// TODO: Parse properties structure if present
	
	return record, nil
}

// tokenizeRecord tokenizes a record line, handling nested structures
func tokenizeRecord(line string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	depth := 0
	inQuotes := false
	escape := false
	
	for i, r := range line {
		if escape {
			current.WriteRune(r)
			escape = false
			continue
		}
		
		switch r {
		case '\\':
			escape = true
			current.WriteRune(r)
		case '"':
			inQuotes = !inQuotes
			current.WriteRune(r)
		case '{':
			if !inQuotes {
				depth++
			}
			current.WriteRune(r)
		case '}':
			if !inQuotes {
				depth--
			}
			current.WriteRune(r)
		case ',':
			if !inQuotes && depth == 0 {
				// End of token
				token := strings.TrimSpace(current.String())
				if token != "" {
					tokens = append(tokens, token)
				}
				current.Reset()
			} else {
				current.WriteRune(r)
			}
		default:
			current.WriteRune(r)
		}
		
		// Safety check
		if i > 10000 {
			return nil, fmt.Errorf("line too long or malformed")
		}
	}
	
	// Add last token
	token := strings.TrimSpace(current.String())
	if token != "" {
		tokens = append(tokens, token)
	}
	
	return tokens, nil
}

// parseLgpTimestamp parses timestamp in format YYYYMMDDHHMMSS
func parseLgpTimestamp(s string) (time.Time, error) {
	if len(s) != 14 {
		return time.Time{}, fmt.Errorf("invalid timestamp length: %d", len(s))
	}
	
	year, _ := strconv.Atoi(s[0:4])
	month, _ := strconv.Atoi(s[4:6])
	day, _ := strconv.Atoi(s[6:8])
	hour, _ := strconv.Atoi(s[8:10])
	min, _ := strconv.Atoi(s[10:12])
	sec, _ := strconv.Atoi(s[12:14])
	
	return time.Date(year, time.Month(month), day, hour, min, sec, 0, time.Local), nil
}

// parseTransaction parses transaction field {transaction_id,connection_id}
func parseTransaction(s string) (string, uint64, error) {
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid transaction format")
	}
	
	transactionID := strings.TrimSpace(parts[0])
	connectionID, err := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil {
		return "", 0, err
	}
	
	return transactionID, connectionID, nil
}

// parseMetadataArray parses metadata array {item1,item2,...}
func parseMetadataArray(s string) ([]string, error) {
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	
	if s == "" {
		return []string{}, nil
	}
	
	// Split by comma, handling quoted strings
	var items []string
	var current strings.Builder
	inQuotes := false
	
	for _, r := range s {
		switch r {
		case '"':
			inQuotes = !inQuotes
			current.WriteRune(r)
		case ',':
			if !inQuotes {
				item := strings.TrimSpace(current.String())
				item = unquoteString(item)
				if item != "" {
					items = append(items, item)
				}
				current.Reset()
			} else {
				current.WriteRune(r)
			}
		default:
			current.WriteRune(r)
		}
	}
	
	// Add last item
	item := strings.TrimSpace(current.String())
	item = unquoteString(item)
	if item != "" {
		items = append(items, item)
	}
	
	return items, nil
}

// unquoteString removes quotes from string
func unquoteString(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// mapLgpLevel maps 1C level code to standard level name
func mapLgpLevel(code string) string {
	switch code {
	case "N":
		return "Note"
	case "I":
		return "Information"
	case "W":
		return "Warning"
	case "E":
		return "Error"
	case "C":
		return "Committed" // Transaction committed
	default:
		return code
	}
}

// mapLgpApplication maps 1C application code to standard name
func mapLgpApplication(code string) string {
	switch code {
	case "I":
		return "Internal"
	case "T":
		return "ThinClient"
	case "W":
		return "WebClient"
	case "H":
		return "ThickClient"
	default:
		return code
	}
}

// truncate truncates string to max length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

