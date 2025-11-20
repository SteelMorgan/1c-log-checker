package eventlog

// LGP Parser for 1C Event Log files
//
// This parser is based on the logic from OneSTools.EventLog project:
// https://github.com/akpaevj/OneSTools.EventLog
//
// Key concepts borrowed:
// - Structure of event log record fields (17 fields)
// - Multi-line record parsing with brace depth tracking
// - Quote-aware brace counting (braces inside strings are ignored)
// - Transaction parsing from hex format
// - Value mappings (severity, application, event types)
// - Complex data structure parsing (R, U, S, B, P types)
//
// See: docs/changelog/lgp-parser-enhancement.md for details

import (
	"bufio"
	"context"
	"os"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/1c-log-checker/internal/domain"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// LgpParser parses .lgp files in 1C Event Log format
// Format: 1CV8LOG(ver 2.0)
//
//	<infobase_guid>
//	{timestamp,level,{transaction},session,user,computer,event,record,app,"comment",data_sep,{metadata},"metadata_pres",...,{props}}
type LgpParser struct {
	infobaseGUID string
	clusterGUID  string
	clusterName  string
	infobaseName string
	lgfReader    *LgfReader // For resolving user_id, computer_id, etc.
}

// NewLgpParser creates a new parser for .lgp files
func NewLgpParser(clusterGUID, infobaseGUID, clusterName, infobaseName string, lgfReader *LgfReader) *LgpParser {
	return &LgpParser{
		clusterGUID:  clusterGUID,
		clusterName:  clusterName,
		infobaseGUID: infobaseGUID,
		infobaseName: infobaseName,
		lgfReader:    lgfReader,
	}
}

// Parse reads and parses .lgp file
func (p *LgpParser) Parse(r io.Reader) ([]*domain.EventLogRecord, error) {
	// Use buffered reader with 4MB buffer for better performance
	reader := bufio.NewReaderSize(r, 4*1024*1024)

	// Pre-allocate records slice with estimated capacity
	records := make([]*domain.EventLogRecord, 0, 10000)
	lineNum := 0

	// Read header
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}
	if line == "" {
		return nil, fmt.Errorf("empty file")
	}
	header := strings.TrimSpace(line)
	// Remove BOM (Byte Order Mark) if present
	header = strings.TrimPrefix(header, "\ufeff")
	if !strings.HasPrefix(header, "1CV8LOG") {
		return nil, fmt.Errorf("invalid header: %s", header)
	}
	lineNum++

	// Read infobase GUID (if not already set)
	line, err = reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read infobase GUID: %w", err)
	}
	guid := strings.TrimSpace(line)
	if p.infobaseGUID == "" {
		p.infobaseGUID = guid
	}
	lineNum++

	// Read all records (records can span multiple lines)
	// Based on C# BracketsListReader logic: track brace depth to find complete records
	// Pre-allocate builder with estimated record size (2KB per record)
	currentRecord := strings.Builder{}
	currentRecord.Grow(2048)
	braceDepth := 0
	inQuotes := false
	escapeNext := false

	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("scanner error: %w", err)
		}
		if line == "" && err == io.EOF {
			break
		}
		lineNum++

		// Trim leading whitespace for processing, but preserve structure
		lineTrimmed := strings.TrimLeft(line, " \t")

		// Handle lines that start with comma
		// If we're building a record (braceDepth > 0), comma is part of continuation - add it
		// If we're not building a record (braceDepth == 0), comma is separator - skip it
		if strings.HasPrefix(lineTrimmed, ",") {
			if braceDepth == 0 {
				// Not building a record - comma is separator, check what's after it
				afterComma := strings.TrimSpace(lineTrimmed[1:])
				if strings.HasPrefix(afterComma, "{") {
					// New record starts after comma - process from after comma
					lineTrimmed = afterComma
				} else if afterComma == "" {
					// Just a comma, skip this line
					continue
				} else {
					// Something else after comma - might be continuation, but we're not building
					// This shouldn't happen, but handle it
					lineTrimmed = afterComma
				}
			}
			// If braceDepth > 0, we're building a record - comma is part of it, process normally
		}

		// Process each character to track braces and quotes
		for _, r := range lineTrimmed {
			// Handle escape sequences in quoted strings
			if escapeNext {
				currentRecord.WriteRune(r)
				escapeNext = false
				continue
			}

			if r == '\\' && inQuotes {
				escapeNext = true
				currentRecord.WriteRune(r)
				continue
			}

			// Track quotes (strings can contain braces)
			if r == '"' {
				inQuotes = !inQuotes
				currentRecord.WriteRune(r)
				continue
			}

			// Only count braces when not inside quotes
			if !inQuotes {
				if r == '{' {
					// If we're starting a new record and depth was 0, this is the start
					if braceDepth == 0 {
						currentRecord.Reset()
					}
					braceDepth++
					currentRecord.WriteRune(r)
				} else if r == '}' {
					braceDepth--
					currentRecord.WriteRune(r)

					// If brace depth is 0, we have a complete record
					if braceDepth == 0 {
						recordStr := strings.TrimSpace(currentRecord.String())

						// Skip empty records
						if recordStr == "" || recordStr == "{}" {
							currentRecord.Reset()
							continue
						}

						record, err := p.parseRecord(recordStr)
						if err != nil {
							log.Warn().Err(err).Int("line", lineNum).Str("line_preview", truncate(recordStr, 100)).Msg("Failed to parse record, skipping")
							currentRecord.Reset()
							continue
						}

						records = append(records, record)
						currentRecord.Reset()
					}
				} else {
					currentRecord.WriteRune(r)
				}
			} else {
				// Inside quotes - just copy
				currentRecord.WriteRune(r)
			}
		}

		// If we're building a record and haven't closed it, add newline
		// (but only if we actually added something to the record)
		if braceDepth > 0 && currentRecord.Len() > 0 {
			currentRecord.WriteRune('\n')
		}

		// Check if we reached EOF
		if err == io.EOF {
			break
		}
	}

	// Check if there's an incomplete record at the end
	if braceDepth > 0 && currentRecord.Len() > 0 {
		log.Warn().Int("line", lineNum).Str("record_preview", truncate(currentRecord.String(), 100)).Msg("Incomplete record at end of file, skipping")
	}

	log.Info().Int("records", len(records)).Msg("Parsed .lgp file")
	return records, nil
}

// ParseStream reads and parses .lgp file streamingly, sending records to channel
// This avoids loading all records into memory for large files
// file must be *os.File to support offset tracking
// offsetCallback is called periodically to save progress (every 1000 records)
// startOffset is the byte offset from which to start reading (0 = from beginning)
func (p *LgpParser) ParseStream(ctx context.Context, file *os.File, recordChan chan<- *domain.EventLogRecord, offsetCallback func(currentOffset int64, recordsCount int64, lastTimestamp time.Time) error, startOffset int64) error {
	// Use buffered reader with 4MB buffer for better performance
	reader := bufio.NewReaderSize(file, 4*1024*1024)
	
	lineNum := 0
	recordsCount := 0
	const offsetSaveInterval = 100000 // Save offset every 100000 records (reduced frequency for performance)

	// Only read header if we're at the beginning of the file
	// If we're resuming from offset, skip header reading
	if startOffset == 0 {
		// Read header
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read header: %w", err)
		}
		if line == "" {
			return fmt.Errorf("empty file")
		}
		header := strings.TrimSpace(line)
		// Remove BOM (Byte Order Mark) if present
		header = strings.TrimPrefix(header, "\ufeff")
		if !strings.HasPrefix(header, "1CV8LOG") {
			return fmt.Errorf("invalid header: %s", header)
		}
		lineNum++

		// Read infobase GUID (if not already set)
		line, err = reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read infobase GUID: %w", err)
		}
		guid := strings.TrimSpace(line)
		if p.infobaseGUID == "" {
			p.infobaseGUID = guid
		}
		lineNum++
	}

	// Read all records (records can span multiple lines)
	// Based on C# BracketsListReader logic: track brace depth to find complete records
	// Pre-allocate builder with estimated record size (2KB per record)
	currentRecord := strings.Builder{}
	currentRecord.Grow(2048)
	braceDepth := 0
	inQuotes := false
	escapeNext := false

	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return fmt.Errorf("scanner error: %w", err)
		}
		if line == "" && err == io.EOF {
			break
		}
		lineNum++

		// Trim leading whitespace for processing, but preserve structure
		lineTrimmed := strings.TrimLeft(line, " \t")

		// Handle lines that start with comma
		// If we're building a record (braceDepth > 0), comma is part of continuation - add it
		// If we're not building a record (braceDepth == 0), comma is separator - skip it
		if strings.HasPrefix(lineTrimmed, ",") {
			if braceDepth == 0 {
				// Not building a record - comma is separator, check what's after it
				afterComma := strings.TrimSpace(lineTrimmed[1:])
				if strings.HasPrefix(afterComma, "{") {
					// New record starts after comma - process from after comma
					lineTrimmed = afterComma
				} else if afterComma == "" {
					// Just a comma, skip this line
					continue
				} else {
					// Something else after comma - might be continuation, but we're not building
					// This shouldn't happen, but handle it
					lineTrimmed = afterComma
				}
			}
			// If braceDepth > 0, we're building a record - comma is part of it, process normally
		}

		// Process each character to track braces and quotes
		for _, r := range lineTrimmed {
			// Handle escape sequences in quoted strings
			if escapeNext {
				currentRecord.WriteRune(r)
				escapeNext = false
				continue
			}

			if r == '\\' && inQuotes {
				escapeNext = true
				currentRecord.WriteRune(r)
				continue
			}

			// Track quotes (strings can contain braces)
			if r == '"' {
				inQuotes = !inQuotes
				currentRecord.WriteRune(r)
				continue
			}

			// Only count braces when not inside quotes
			if !inQuotes {
				if r == '{' {
					// If we're starting a new record and depth was 0, this is the start
					if braceDepth == 0 {
						currentRecord.Reset()
					}
					braceDepth++
					currentRecord.WriteRune(r)
				} else if r == '}' {
					braceDepth--
					currentRecord.WriteRune(r)

					// If brace depth is 0, we have a complete record
					if braceDepth == 0 {
						recordStr := strings.TrimSpace(currentRecord.String())

						// Skip empty records
						if recordStr == "" || recordStr == "{}" {
							currentRecord.Reset()
							continue
						}

						record, err := p.parseRecord(recordStr)
						if err != nil {
							log.Warn().Err(err).Int("line", lineNum).Str("line_preview", truncate(recordStr, 100)).Msg("Failed to parse record, skipping")
							currentRecord.Reset()
							continue
						}

						// Send record to channel (non-blocking with context check)
						select {
						case <-ctx.Done():
							return ctx.Err()
						case recordChan <- record:
							recordsCount++
							
							// Save offset periodically (every offsetSaveInterval records)
							if offsetCallback != nil && recordsCount%offsetSaveInterval == 0 {
								// Get current file position
								currentPos, err := file.Seek(0, io.SeekCurrent)
								if err == nil {
									if err := offsetCallback(currentPos, int64(recordsCount), record.EventTime); err != nil {
										log.Warn().Err(err).Msg("Failed to save offset")
									}
								}
							}
						}
						currentRecord.Reset()
					}
				} else {
					currentRecord.WriteRune(r)
				}
			} else {
				// Inside quotes - just copy
				currentRecord.WriteRune(r)
			}
		}

		// If we're building a record and haven't closed it, add newline
		// (but only if we actually added something to the record)
		if braceDepth > 0 && currentRecord.Len() > 0 {
			currentRecord.WriteRune('\n')
		}

		// Check if we reached EOF
		if err == io.EOF {
			break
		}
	}

	// Check if there's an incomplete record at the end
	if braceDepth > 0 && currentRecord.Len() > 0 {
		log.Warn().Int("line", lineNum).Str("record_preview", truncate(currentRecord.String(), 100)).Msg("Incomplete record at end of file, skipping")
	}

	log.Info().Int("records", recordsCount).Msg("Parsed .lgp file (streaming mode)")
	return nil
}

// parseRecord parses a single record from .lgp file
// Format based on OneSTools.EventLog C# implementation:
// {timestamp,transaction_status,{transaction_date_hex,transaction_number_hex},user_id,computer_id,application,connection,
// event_id,severity,comment,metadata,data,data_presentation,server,main_port,add_port,session}
func (p *LgpParser) parseRecord(line string) (*domain.EventLogRecord, error) {
	// Trim whitespace first
	line = strings.TrimSpace(line)

	// Skip empty lines or lines that don't start with {
	if line == "" || !strings.HasPrefix(line, "{") {
		return nil, fmt.Errorf("empty or invalid record line")
	}

	// Remove outer braces
	line = strings.TrimPrefix(line, "{")
	line = strings.TrimSuffix(line, "}")
	line = strings.TrimSpace(line)

	// Check if line is empty after removing braces
	if line == "" {
		return nil, fmt.Errorf("empty record after removing braces")
	}

	// Default date for invalid/zero dates (1980-01-01 instead of 0001-01-01 to avoid Grafana/ClickHouse errors)
	defaultTransactionDate := time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

	record := &domain.EventLogRecord{
		ClusterGUID:         p.clusterGUID,
		ClusterName:         p.clusterName,
		InfobaseGUID:        p.infobaseGUID,
		InfobaseName:        p.infobaseName,
		TransactionDateTime: defaultTransactionDate, // Set default to avoid zero time
		Properties:          make(map[string]string),
	}

	// Parse fields using tokenizer that handles nested structures
	tokens, err := tokenizeRecord(line)
	if err != nil {
		return nil, fmt.Errorf("tokenization failed: %w", err)
	}

	if len(tokens) < 9 {
		return nil, fmt.Errorf("insufficient tokens: %d, expected at least 9", len(tokens))
	}

	// Field 0: timestamp (YYYYMMDDHHMMSS)
	timestamp, err := parseLgpTimestamp(tokens[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %w", err)
	}
	record.EventTime = timestamp
	record.EventDate = time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), 0, 0, 0, 0, timestamp.Location())

	// Field 1: transaction status (U, C, R, N)
	record.TransactionStatus = getTransactionPresentation(tokens[1])

	// Field 2: transaction {transaction_date_hex, transaction_number_hex}
	if len(tokens) > 2 {
		// DEBUG: Log raw token before parsing
		log.Debug().
			Str("transaction_token_raw", tokens[2]).
			Int("token_index", 2).
			Msg("DEBUG: Raw transaction token from record")

		transactionDateTime, transactionNumber, connectionID, err := parseTransactionFromHex(tokens[2])
		if err == nil {
			// DEBUG: Log parsing result
			log.Debug().
				Time("parsed_datetime", transactionDateTime).
				Int64("parsed_number", transactionNumber).
				Uint64("parsed_connection_id", connectionID).
				Bool("is_zero", transactionDateTime.IsZero()).
				Msg("DEBUG: Transaction parsing result")

			// Use parsed transaction datetime as-is (no normalization/validation)
			// If transactionDateTime is zero (from {0,0}), keep default value (1980-01-01)
			if !transactionDateTime.IsZero() {
				record.TransactionDateTime = transactionDateTime
				log.Debug().
					Time("assigned_datetime", record.TransactionDateTime).
					Msg("DEBUG: Assigned transaction datetime to record")
			} else {
				log.Debug().
					Time("default_datetime", record.TransactionDateTime).
					Msg("DEBUG: Transaction datetime is zero, keeping default (1980-01-01)")
			}
			record.TransactionNumber = transactionNumber
			record.ConnectionID = connectionID
			// TransactionID is derived from transaction number
			record.TransactionID = fmt.Sprintf("%d", transactionNumber)
		} else {
			log.Debug().
				Err(err).
				Str("token", tokens[2]).
				Msg("DEBUG: Failed to parse transaction, keeping default datetime")
		}
	} else {
		log.Debug().
			Int("tokens_count", len(tokens)).
			Msg("DEBUG: Not enough tokens for transaction field, keeping default datetime")
	}

	// Field 3: user_id (number - resolved from LGF file)
	// Based on OneSTools.EventLog: GetReferencedObjectValue(ObjectType.Users, parsedData[3])
	if len(tokens) > 3 {
		userID := parseNumberString(tokens[3])
		if p.lgfReader != nil && userID > 0 {
			userName, userUUID := p.lgfReader.GetReferencedObjectValue(ObjectTypeUsers, userID, context.Background())
			if userName != "" {
				record.UserName = userName
			} else if userUUID != "" {
				// If name is empty but UUID exists, use UUID as fallback
				record.UserName = userUUID
			}
			if userUUID != "" {
				if uuid, err := uuid.Parse(userUUID); err == nil {
					record.UserID = uuid
				}
			}
		}
		// Store original ID in properties for reference
		record.Properties["user_id"] = tokens[3]
	}

	// Field 4: computer_id (number - resolved from LGF file)
	// Based on OneSTools.EventLog: GetObjectValue(ObjectType.Computers, parsedData[4])
	if len(tokens) > 4 {
		computerID := parseNumberString(tokens[4])
		if p.lgfReader != nil && computerID > 0 {
			computerName := p.lgfReader.GetObjectValue(ObjectTypeComputers, computerID, context.Background())
			if computerName != "" {
				record.Computer = computerName
			}
		}
		// Store original ID in properties for reference
		record.Properties["computer_id"] = tokens[4]
	}

	// Field 5: application (code - will be resolved from LGF file later)
	if len(tokens) > 5 {
		applicationID := parseNumberString(tokens[5])
		if p.lgfReader != nil && applicationID > 0 {
			// Get application code from LGF file (e.g., "1CV8C", "WebClient")
			applicationCode := p.lgfReader.GetObjectValue(ObjectTypeApplications, applicationID, context.Background())
			if applicationCode != "" {
				record.Application = applicationCode
				record.ApplicationPresentation = getApplicationPresentation(applicationCode)
			} else {
				// Fallback: use raw ID if not found in LGF
				record.Application = tokens[5]
				record.ApplicationPresentation = getApplicationPresentation(tokens[5])
			}
		} else {
			// If ID is 0 or LGF reader is nil, use raw value
			record.Application = tokens[5]
			record.ApplicationPresentation = getApplicationPresentation(tokens[5])
		}
	}

	// Field 6: connection (string)
	if len(tokens) > 6 {
		record.Connection = unquoteString(tokens[6])
	}

	// Field 7: event_id (number - MUST be resolved from LGF file)
	// The event ID is a number that maps to an event code (e.g., "_$Data$_.New")
	// which then needs to be converted to human-readable presentation
	if len(tokens) > 7 {
		eventID := parseNumberString(tokens[7])
		if p.lgfReader != nil && eventID > 0 {
			// Get event code from LGF file (e.g., "_$Data$_.New", "_$Access$_.Access")
			eventCode := p.lgfReader.GetObjectValue(ObjectTypeEvents, eventID, context.Background())
			if eventCode != "" {
				record.Event = eventCode
				record.EventPresentation = getEventPresentation(eventCode)
			} else {
				// Fallback: if event not found in LGF, use raw ID and try to map it
				record.Event = tokens[7]
				record.EventPresentation = getEventPresentation(tokens[7])
				log.Debug().
					Str("event_id", tokens[7]).
					Msg("Event ID not found in LGF file, using raw ID")
			}
		} else {
			// If ID is 0 or LGF reader is nil, use raw value
			record.Event = tokens[7]
			record.EventPresentation = getEventPresentation(tokens[7])
		}
	}

	// Field 8: severity (I, E, W, N) - this is the level!
	if len(tokens) > 8 {
		record.Level = getSeverityPresentation(tokens[8])
	}

	// Field 9: comment (quoted string)
	if len(tokens) > 9 {
		record.Comment = unquoteString(tokens[9])
	}

	// Field 10: metadata (array - resolved from LGF file)
	// Based on C#: GetReferencedObjectValue(ObjectType.Metadata, parsedData[10])
	// Returns (value, uuid) where value is the presentation (MetadataPresentation)
	if len(tokens) > 10 {
		metadataID := parseNumberString(tokens[10])
		if p.lgfReader != nil && metadataID > 0 {
			metadataPresentation, metadataUUID := p.lgfReader.GetReferencedObjectValue(ObjectTypeMetadata, metadataID, context.Background())
			if metadataPresentation != "" {
				record.MetadataPresentation = metadataPresentation
				// Extract metadata name from presentation if possible, or use UUID
				if metadataUUID != "" {
					record.MetadataName = metadataUUID
				} else {
					record.MetadataName = metadataPresentation
				}
			}
		}
		// Store original ID in properties for reference
		record.Properties["metadata_id"] = tokens[10]
	}

	// Field 11: data (complex structure)
	if len(tokens) > 11 {
		data, err := parseDataField(tokens[11])
		if err == nil {
			record.Data = data
		}
	}

	// Field 12: data_presentation (quoted string)
	if len(tokens) > 12 {
		record.DataPresentation = unquoteString(tokens[12])
	}

	// Field 13: server (code - will be resolved from LGF file later)
	if len(tokens) > 13 {
		record.Server = tokens[13]
	}

	// Field 14: main_port (code - will be resolved from LGF file later)
	if len(tokens) > 14 {
		if port, err := strconv.ParseUint(tokens[14], 10, 16); err == nil {
			record.PrimaryPort = uint16(port)
		}
	}

	// Field 15: add_port (code - will be resolved from LGF file later)
	if len(tokens) > 15 {
		if port, err := strconv.ParseUint(tokens[15], 10, 16); err == nil {
			record.SecondaryPort = uint16(port)
		}
	}

	// Field 16: session (string)
	if len(tokens) > 16 {
		if sessionID, err := strconv.ParseUint(tokens[16], 10, 64); err == nil {
			record.SessionID = sessionID
		} else {
			// If not a number, store as string
			record.Properties["session_string"] = tokens[16]
		}
	}

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
// 1C stores timestamps in local time (MSK for Russian installations)
// We parse as UTC to match ClickHouse storage (ClickHouse stores DateTime in UTC)
// The timestamp from 1C log is already in local time, so we treat it as UTC
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

	// Parse as UTC - 1C logs store time in local timezone (MSK), but we need UTC for ClickHouse
	// Note: This assumes 1C log timestamps are in MSK. If your 1C server uses different timezone,
	// you may need to adjust this or add timezone configuration.
	return time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC), nil
}

// parseTransactionFromHex parses transaction field {transaction_date_hex, transaction_number_hex}
// Based on C#: Convert.ToInt64(transactionData[0], 16) / 10000 for date, Convert.ToInt64(transactionData[1], 16) for number
func parseTransactionFromHex(s string) (time.Time, int64, uint64, error) {
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")

	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return time.Time{}, 0, 0, fmt.Errorf("invalid transaction format: expected 2 parts, got %d", len(parts))
	}

	// Parse transaction date (hex to int64, then divide by 10000 to get seconds)
	// Based on C#: new DateTime().AddSeconds(Convert.ToInt64(transactionData[0], 16) / 10000)
	// IMPORTANT: In C#, new DateTime() creates 0001-01-01 00:00:00, NOT Unix epoch (1970-01-01)
	dateHex := strings.TrimSpace(parts[0])

	// Handle special case: {0,0} means no transaction
	if dateHex == "0" {
		// Return zero time - caller should handle this appropriately
		return time.Time{}, 0, 0, nil
	}

	dateValue, err := strconv.ParseInt(dateHex, 16, 64)
	if err != nil {
		return time.Time{}, 0, 0, fmt.Errorf("failed to parse transaction date hex: %w", err)
	}

	// DEBUG: Log raw values from file
	log.Debug().
		Str("date_hex_raw", dateHex).
		Int64("date_value_parsed", dateValue).
		Msg("DEBUG: Raw transaction date values from file")

	// Divide by 10000 to get seconds directly (as per C# code: new DateTime().AddSeconds(Convert.ToInt64(transactionData[0], 16) / 10000))
	// The C# code divides by 10000 to get seconds, not ticks
	seconds := dateValue / 10000

	// DEBUG: Log intermediate calculations
	log.Debug().
		Int64("seconds_after_div_10000", seconds).
		Msg("DEBUG: Intermediate calculation values")

	// IMPORTANT: According to 1C logic, after dividing by 10000, we need to subtract
	// seconds in 1970 years (62 451 156 554) to get Unix epoch seconds
	// Then use Unix epoch (1970-01-01) instead of 1C epoch (0001-01-01)
	const secondsIn1970Years = 62451156554
	unixSeconds := seconds - secondsIn1970Years

	// Create time from Unix epoch (1970-01-01) using the corrected seconds
	transactionDateTime := time.Unix(unixSeconds, 0).UTC()

	// DEBUG: Log final calculated date
	log.Debug().
		Str("date_hex", dateHex).
		Int64("date_value", dateValue).
		Int64("seconds_after_div_10000", seconds).
		Int64("unix_seconds_after_subtract", unixSeconds).
		Time("calculated_date", transactionDateTime).
		Msg("DEBUG: Final transaction datetime calculation")

	// Log warning if date seems unreasonable (for debugging)
	if unixSeconds > 0 {
		// Check if date is in reasonable range (1900-2100)
		if transactionDateTime.Year() < 1900 || transactionDateTime.Year() > 2100 {
			log.Warn().
				Str("date_hex", dateHex).
				Int64("date_value", dateValue).
				Int64("unix_seconds", unixSeconds).
				Time("calculated_date", transactionDateTime).
				Msg("Transaction date outside reasonable range (1900-2100), but using as-is")
		}
	}

	// Parse transaction number (hex to int64)
	numberHex := strings.TrimSpace(parts[1])
	transactionNumber, err := strconv.ParseInt(numberHex, 16, 64)
	if err != nil {
		return time.Time{}, 0, 0, fmt.Errorf("failed to parse transaction number hex: %w", err)
	}

	// ConnectionID is typically the same as transaction number in this context
	connectionID := uint64(transactionNumber)

	return transactionDateTime, transactionNumber, connectionID, nil
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

// getTransactionPresentation maps transaction status code to Russian presentation
// Based on C# GetTransactionPresentation
func getTransactionPresentation(code string) string {
	switch code {
	case "U":
		return "Зафиксирована"
	case "C":
		return "Отменена"
	case "R":
		return "Не завершена"
	case "N":
		return "Нет транзакции"
	default:
		return code
	}
}

// getSeverityPresentation maps severity code to Russian presentation
// Based on C# GetSeverityPresentation
func getSeverityPresentation(code string) string {
	switch code {
	case "I":
		return "Информация"
	case "E":
		return "Ошибка"
	case "W":
		return "Предупреждение"
	case "N":
		return "Примечание"
	default:
		return code
	}
}

// getApplicationPresentation maps application code to Russian presentation
// Based on C# GetApplicationPresentation
func getApplicationPresentation(code string) string {
	switch code {
	case "1CV8":
		return "Толстый клиент"
	case "1CV8C":
		return "Тонкий клиент"
	case "WebClient":
		return "Веб-клиент"
	case "Designer":
		return "Конфигуратор"
	case "COMConnection":
		return "Внешнее соединение (COM, обычное)"
	case "WSConnection":
		return "Сессия web-сервиса"
	case "BackgroundJob":
		return "Фоновое задание"
	case "SystemBackgroundJob":
		return "Системное фоновое задание"
	case "SrvrConsole":
		return "Консоль кластера"
	case "COMConsole":
		return "Внешнее соединение (COM, административное)"
	case "JobScheduler":
		return "Планировщик заданий"
	case "Debugger":
		return "Отладчик"
	case "RAS":
		return "Сервер администрирования"
	default:
		return code
	}
}

// getEventPresentation maps event code to Russian presentation
// Based on C# GetEventPresentation - full mapping
func getEventPresentation(code string) string {
	// Remove quotes if present
	code = unquoteString(code)

	eventMap := map[string]string{
		"_$Access$_.Access":                                 "Доступ.Доступ",
		"_$Access$_.AccessDenied":                           "Доступ.Отказ в доступе",
		"_$Data$_.Delete":                                   "Данные.Удаление",
		"_$Data$_.DeletePredefinedData":                     "Данные.Удаление предопределенных данных",
		"_$Data$_.DeleteVersions":                           "Данные.Удаление версий",
		"_$Data$_.New":                                      "Данные.Добавление",
		"_$Data$_.NewPredefinedData":                        "Данные.Добавление предопределенных данных",
		"_$Data$_.NewVersion":                               "Данные.Добавление версии",
		"_$Data$_.Pos":                                      "Данные.Проведение",
		"_$Data$_.PredefinedDataInitialization":             "Данные.Инициализация предопределенных данных",
		"_$Data$_.PredefinedDataInitializationDataNotFound": "Данные.Инициализация предопределенных данных.Данные не найдены",
		"_$Data$_.SetPredefinedDataInitialization":          "Данные.Установка инициализации предопределенных данных",
		"_$Data$_.SetStandardODataInterfaceContent":         "Данные.Изменение состава стандартного интерфейса OData",
		"_$Data$_.TotalsMaxPeriodUpdate":                    "Данные.Изменение максимального периода рассчитанных итогов",
		"_$Data$_.TotalsMinPeriodUpdate":                    "Данные.Изменение минимального периода рассчитанных итогов",
		"_$Data$_.Post":                                     "Данные.Проведение",
		"_$Data$_.Unpost":                                   "Данные.Отмена проведения",
		"_$Data$_.Update":                                   "Данные.Изменение",
		"_$Data$_.UpdatePredefinedData":                     "Данные.Изменение предопределенных данных",
		"_$Data$_.VersionCommentUpdate":                     "Данные.Изменение комментария версии",
		"_$InfoBase$_.ConfigExtensionUpdate":                "Информационная база.Изменение расширения конфигурации",
		"_$InfoBase$_.ConfigUpdate":                         "Информационная база.Изменение конфигурации",
		"_$InfoBase$_.DBConfigBackgroundUpdateCancel":       "Информационная база.Отмена фонового обновления",
		"_$InfoBase$_.DBConfigBackgroundUpdateFinish":       "Информационная база.Завершение фонового обновления",
		"_$InfoBase$_.DBConfigBackgroundUpdateResume":       "Информационная база.Продолжение (после приостановки) процесса фонового обновления",
		"_$InfoBase$_.DBConfigBackgroundUpdateStart":        "Информационная база.Запуск фонового обновления",
		"_$InfoBase$_.DBConfigBackgroundUpdateSuspend":      "Информационная база.Приостановка (пауза) процесса фонового обновления",
		"_$InfoBase$_.DBConfigExtensionUpdate":              "Информационная база.Изменение расширения конфигурации",
		"_$InfoBase$_.DBConfigExtensionUpdateError":         "Информационная база.Ошибка изменения расширения конфигурации",
		"_$InfoBase$_.DBConfigUpdate":                       "Информационная база.Изменение конфигурации базы данных",
		"_$InfoBase$_.DBConfigUpdateStart":                  "Информационная база.Запуск обновления конфигурации базы данных",
		"_$InfoBase$_.DumpError":                            "Информационная база.Ошибка выгрузки в файл",
		"_$InfoBase$_.DumpFinish":                           "Информационная база.Окончание выгрузки в файл",
		"_$InfoBase$_.DumpStart":                            "Информационная база.Начало выгрузки в файл",
		"_$InfoBase$_.EraseData":                            "Информационная база.Удаление данных информационной баз",
		"_$InfoBase$_.EventLogReduce":                       "Информационная база.Сокращение журнала регистрации",
		"_$InfoBase$_.EventLogReduceError":                  "Информационная база.Ошибка сокращения журнала регистрации",
		"_$InfoBase$_.EventLogSettingsUpdate":               "Информационная база.Изменение параметров журнала регистрации",
		"_$InfoBase$_.EventLogSettingsUpdateError":          "Информационная база.Ошибка при изменение настроек журнала регистрации",
		"_$InfoBase$_.InfoBaseAdmParamsUpdate":              "Информационная база.Изменение параметров информационной базы",
		"_$InfoBase$_.InfoBaseAdmParamsUpdateError":         "Информационная база.Ошибка изменения параметров информационной базы",
		"_$InfoBase$_.IntegrationServiceActiveUpdate":       "Информационная база.Изменение активности сервиса интеграции",
		"_$InfoBase$_.IntegrationServiceSettingsUpdate":     "Информационная база.Изменение настроек сервиса интеграции",
		"_$InfoBase$_.MasterNodeUpdate":                     "Информационная база.Изменение главного узла",
		"_$InfoBase$_.PredefinedDataUpdate":                 "Информационная база.Обновление предопределенных данных",
		"_$InfoBase$_.RegionalSettingsUpdate":               "Информационная база.Изменение региональных установок",
		"_$InfoBase$_.RestoreError":                         "Информационная база.Ошибка загрузки из файла",
		"_$InfoBase$_.RestoreFinish":                        "Информационная база.Окончание загрузки из файла",
		"_$InfoBase$_.RestoreStart":                         "Информационная база.Начало загрузки из файла",
		"_$InfoBase$_.SessionLockChange":                    "Информационная база.Изменение блокировки сеанса",
		"_$InfoBase$_.SecondFactorAuthTemplateDelete":       "Информационная база.Удаление шаблона вторго фактора аутентификации",
		"_$InfoBase$_.SecondFactorAuthTemplateNew":          "Информационная база.Добавление шаблона вторго фактора аутентификации",
		"_$InfoBase$_.SecondFactorAuthTemplateUpdate":       "Информационная база.Изменение шаблона вторго фактора аутентификации",
		"_$InfoBase$_.SetPredefinedDataUpdate":              "Информационная база.Установить обновление предопределенных данных",
		"_$InfoBase$_.TARImportant":                         "Тестирование и исправление.Ошибка",
		"_$InfoBase$_.TARInfo":                              "Тестирование и исправление.Сообщение",
		"_$InfoBase$_.TARMess":                              "Тестирование и исправление.Предупреждение",
		"_$Job$_.Cancel":                                    "Фоновое задание.Отмена",
		"_$Job$_.Fail":                                      "Фоновое задание.Ошибка выполнения",
		"_$Job$_.Start":                                     "Фоновое задание.Запуск",
		"_$Job$_.Succeed":                                   "Фоновое задание.Успешное завершение",
		"_$Job$_.Terminate":                                 "Фоновое задание.Принудительное завершение",
		"_$OpenIDProvider$_.NegativeAssertion":              "Провайдер OpenID.Отклонено",
		"_$OpenIDProvider$_.PositiveAssertion":              "Провайдер OpenID.Подтверждено",
		"_$PerformError$_":                                  "Ошибка выполнения",
		"_$Session$_.Authentication":                        "Сеанс.Аутентификация",
		"_$Session$_.AuthenticationError":                   "Сеанс.Ошибка аутентификации",
		"_$Session$_.AuthenticationFirstFactor":             "Сеанс.Аутентификация первый фактор",
		"_$Session$_.ConfigExtensionApplyError":             "Сеанс.Ошибка применения расширения конфигурации",
		"_$Session$_.DataZoneChange":                        "Сеанс.Изменение зоны данных",
		"_$Session$_.Finish":                                "Сеанс.Завершение",
		"_$Session$_.Start":                                 "Сеанс.Начало",
		"_$Transaction$_.Begin":                             "Транзакция.Начало",
		"_$Transaction$_.Commit":                            "Транзакция.Фиксация",
		"_$Transaction$_.Rollback":                          "Транзакция.Отмена",
		"_$User$_.AuthenticationLock":                       "Пользователи.Блокировка аутентификации",
		"_$User$_.AuthenticationUnlock":                     "Пользователи.Разблокировка аутентификации",
		"_$User$_.AuthenticationUnlockError ":               "Пользователи.Ошибка разблокировки аутентификации",
		"_$User$_.Delete":                                   "Пользователи.Удаление",
		"_$User$_.DeleteError":                              "Пользователи.Ошибка удаления",
		"_$User$_.New":                                      "Пользователи.Добавление",
		"_$User$_.NewError":                                 "Пользователи.Ошибка добавления",
		"_$User$_.Update":                                   "Пользователи.Изменение",
		"_$User$_.UpdateError":                              "Пользователи. Ошибка изменения",
	}

	if presentation, ok := eventMap[code]; ok {
		return presentation
	}
	return code
}

// parseDataField parses complex data structure
// Based on C# GetData method
func parseDataField(s string) (string, error) {
	// Remove outer braces if present
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")

	if s == "" {
		return "", nil
	}

	// Parse data type and value
	// Format: {type,value} or {type,{subdata}}
	tokens, err := tokenizeRecord(s)
	if err != nil {
		return "", fmt.Errorf("failed to tokenize data field: %w", err)
	}

	if len(tokens) < 2 {
		return "", nil
	}

	dataType := tokens[0]
	value := tokens[1]

	switch dataType {
	case "R": // Reference
		return unquoteString(value), nil
	case "U": // Undefined
		return "", nil
	case "S": // String
		return unquoteString(value), nil
	case "B": // Boolean
		if value == "0" {
			return "false", nil
		}
		return "true", nil
	case "P": // Complex data
		// Parse sub-data recursively
		var result strings.Builder
		subData := value
		subData = strings.TrimPrefix(subData, "{")
		subData = strings.TrimSuffix(subData, "}")

		subTokens, err := tokenizeRecord(subData)
		if err != nil {
			return "", fmt.Errorf("failed to parse complex data: %w", err)
		}

		// Skip first token (sub-data type)
		for i := 1; i < len(subTokens); i++ {
			subValue, err := parseDataField("{" + subTokens[i] + "}")
			if err == nil && subValue != "" {
				result.WriteString(fmt.Sprintf("Item %d: %s\n", i, subValue))
			}
		}

		return strings.TrimSpace(result.String()), nil
	default:
		return "", nil
	}
}

// truncate truncates string to max length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
