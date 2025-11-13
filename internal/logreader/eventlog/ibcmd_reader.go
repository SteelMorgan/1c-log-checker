package eventlog

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/1c-log-checker/internal/domain"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// IbcmdReader reads 1C Event Log using ibcmd utility (official 1C method)
// Based on official 1C script: https://its.1c.ru/db/metod8dev#content:6019:hdoc:tld_script
type IbcmdReader struct {
	ibcmdPath    string
	eventLogDir  string
	infobaseGUID string
	clusterGUID  string
	format       string // "json" or "xml"
	followTime   int    // milliseconds
	
	checkpointFile string
	lastCheckpoint string
	
	cmd     *exec.Cmd
	scanner *bufio.Scanner
}

// NewIbcmdReader creates a new ibcmd-based reader
// ibcmdPath should be path to version folder (e.g., C:\Program Files\1cv8\8.3.27.1719)
// The function will append \bin\ibcmd.exe automatically
func NewIbcmdReader(ibcmdPath, eventLogDir, clusterGUID, infobaseGUID string) (*IbcmdReader, error) {
	// Append \bin\ibcmd.exe to the path if it's a version folder
	if !strings.HasSuffix(strings.ToLower(ibcmdPath), "ibcmd.exe") {
		ibcmdPath = filepath.Join(ibcmdPath, "bin", "ibcmd.exe")
	}
	// Extract infobase GUID from path if not provided
	if infobaseGUID == "" {
		normalized := filepath.Clean(eventLogDir)
		parts := strings.Split(normalized, string(filepath.Separator))
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid event log directory path: %s", eventLogDir)
		}
		
		// Path format: .../reg_<port>/<infobase_guid>/1Cv8Log
		if strings.ToLower(parts[len(parts)-1]) != "1cv8log" {
			return nil, fmt.Errorf("event log directory must end with 1Cv8Log: %s", eventLogDir)
		}
		
		infobaseGUID = parts[len(parts)-2]
	}
	
	checkpointFile := fmt.Sprintf("%s.dat", infobaseGUID)
	
	return &IbcmdReader{
		ibcmdPath:     ibcmdPath,
		eventLogDir:   eventLogDir,
		infobaseGUID:  infobaseGUID,
		clusterGUID:   clusterGUID,
		format:        "json", // Default format
		followTime:    1000,   // Default 1 second
		checkpointFile: checkpointFile,
	}, nil
}

// Open opens the reader and starts ibcmd process
func (r *IbcmdReader) Open(ctx context.Context) error {
	// Read checkpoint
	checkpoint, err := r.readCheckpoint()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read checkpoint, starting from beginning")
		checkpoint = ""
	}
	r.lastCheckpoint = checkpoint
	
	// Build ibcmd command
	// ibcmd eventlog export --format json --follow 1000 --skip-root [--from <checkpoint>] <event_log_dir>
	args := []string{
		"eventlog",
		"export",
		"--format",
		r.format,
		"--follow",
		fmt.Sprintf("%d", r.followTime),
		"--skip-root",
	}
	
	if checkpoint != "" {
		args = append(args, "--from", checkpoint)
	}
	
	args = append(args, r.eventLogDir)
	
	log.Info().
		Str("ibcmd", r.ibcmdPath).
		Strs("args", args).
		Str("checkpoint", checkpoint).
		Msg("Starting ibcmd process")
	
	// Start ibcmd process
	r.cmd = exec.CommandContext(ctx, r.ibcmdPath, args...)
	
	stdout, err := r.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	
	stderr, err := r.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	
	if err := r.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ibcmd: %w", err)
	}
	
	// Create scanner for stdout
	r.scanner = bufio.NewScanner(stdout)
	
	// Monitor stderr in background
	go r.monitorStderr(stderr)
	
	log.Info().Msg("Ibcmd reader opened successfully")
	return nil
}

// Read reads the next event log record
func (r *IbcmdReader) Read(ctx context.Context) (*domain.EventLogRecord, error) {
	if r.scanner == nil {
		return nil, fmt.Errorf("reader not opened")
	}
	
	for r.scanner.Scan() {
		line := strings.TrimSpace(r.scanner.Text())
		if line == "" {
			continue
		}
		
		// Parse JSON line
		record, err := r.parseJSONEvent(line)
		if err != nil {
			log.Warn().Err(err).Str("line", truncate(line, 100)).Msg("Failed to parse event, skipping")
			continue
		}
		
		// Update checkpoint
		if err := r.updateCheckpoint(record.EventTime); err != nil {
			log.Warn().Err(err).Msg("Failed to update checkpoint")
		}
		
		return record, nil
	}
	
	// Check for scanner error
	if err := r.scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}
	
	// Check if process exited
	if r.cmd != nil {
		if err := r.cmd.Wait(); err != nil {
			return nil, fmt.Errorf("ibcmd process exited with error: %w", err)
		}
	}
	
	return nil, fmt.Errorf("end of stream")
}

// Seek seeks to a specific position (not supported for ibcmd, uses checkpoint instead)
func (r *IbcmdReader) Seek(ctx context.Context, offset int64) error {
	// ibcmd uses checkpoint-based positioning, not file offsets
	// To seek, update the checkpoint file and restart the reader
	return fmt.Errorf("Seek not supported for ibcmd reader (use checkpoint instead)")
}

// Close closes the reader and stops ibcmd process
func (r *IbcmdReader) Close() error {
	if r.cmd != nil && r.cmd.Process != nil {
		if err := r.cmd.Process.Kill(); err != nil {
			log.Warn().Err(err).Msg("Failed to kill ibcmd process")
		}
	}
	return nil
}

// parseJSONEvent parses a JSON event line from ibcmd output
func (r *IbcmdReader) parseJSONEvent(line string) (*domain.EventLogRecord, error) {
	var eventData map[string]interface{}
	if err := json.Unmarshal([]byte(line), &eventData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	
	record := &domain.EventLogRecord{
		ClusterGUID:  r.clusterGUID,
		InfobaseGUID: r.infobaseGUID,
		Properties:   make(map[string]string),
	}
	
	// Parse Date
	if dateStr, ok := eventData["Date"].(string); ok {
		eventTime, err := time.Parse("2006-01-02T15:04:05", dateStr)
		if err != nil {
			// Try with microseconds
			eventTime, err = time.Parse("2006-01-02T15:04:05.000000", dateStr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse date: %w", err)
			}
		}
		record.EventTime = eventTime
		record.EventDate = time.Date(eventTime.Year(), eventTime.Month(), eventTime.Day(), 0, 0, 0, 0, eventTime.Location())
	}
	
	// Parse Event
	if event, ok := eventData["Event"].(string); ok {
		record.Event = event
	}
	if eventPres, ok := eventData["EventPresentation"].(string); ok {
		record.EventPresentation = eventPres
	}
	
	// Parse Level
	if level, ok := eventData["Level"].(string); ok {
		record.Level = level
	}
	
	// Parse Session
	if session, ok := eventData["Session"].(float64); ok {
		record.SessionID = uint64(session)
	}
	
	// Parse User
	if user, ok := eventData["User"].(string); ok {
		record.UserName = user
	}
	if userName, ok := eventData["UserName"].(string); ok {
		if record.UserName == "" {
			record.UserName = userName
		}
	}
	
	// Parse UserID (UUID)
	if userIDStr, ok := eventData["UserID"].(string); ok {
		if userID, err := uuid.Parse(userIDStr); err == nil {
			record.UserID = userID
		}
	}
	
	// Parse Computer
	if computer, ok := eventData["Computer"].(string); ok {
		record.Computer = computer
	}
	
	// Parse Application
	if app, ok := eventData["Application"].(string); ok {
		record.Application = app
	}
	if appPres, ok := eventData["ApplicationPresentation"].(string); ok {
		record.ApplicationPresentation = appPres
	}
	
	// Parse Connection
	if conn, ok := eventData["Connection"].(float64); ok {
		record.ConnectionID = uint64(conn)
	}
	
	// Parse Transaction
	if transStatus, ok := eventData["TransactionStatus"].(string); ok {
		record.TransactionStatus = transStatus
	}
	if transID, ok := eventData["TransactionID"].(string); ok {
		record.TransactionID = transID
	}
	
	// Parse Metadata
	if metadata, ok := eventData["MetadataName"].(string); ok {
		record.MetadataName = metadata
	}
	if metadataPres, ok := eventData["MetadataPresentation"].(string); ok {
		record.MetadataPresentation = metadataPres
	}
	
	// Parse Comment
	if comment, ok := eventData["Comment"].(string); ok {
		record.Comment = comment
	}
	
	// Parse Data
	if data, ok := eventData["Data"].(string); ok {
		record.Data = data
	}
	if dataPres, ok := eventData["DataPresentation"].(string); ok {
		record.DataPresentation = dataPres
	}
	
	// Parse Server
	if server, ok := eventData["ServerName"].(string); ok {
		record.Server = server
	}
	if port, ok := eventData["Port"].(float64); ok {
		record.PrimaryPort = uint16(port)
	}
	if syncPort, ok := eventData["SyncPort"].(float64); ok {
		record.SecondaryPort = uint16(syncPort)
	}
	
	// Parse DataSeparation
	if dataSep, ok := eventData["SessionDataSeparation"].(string); ok {
		record.DataSeparation = dataSep
	}
	
	// Store all other fields in Properties
	for key, value := range eventData {
		if !isKnownField(key) {
			// Convert value to string
			if strValue, ok := value.(string); ok {
				record.Properties[key] = strValue
			} else {
				// Serialize complex types to JSON
				if jsonBytes, err := json.Marshal(value); err == nil {
					record.Properties[key] = string(jsonBytes)
				}
			}
		}
	}
	
	return record, nil
}

// isKnownField checks if field is a known domain field
func isKnownField(key string) bool {
	knownFields := map[string]bool{
		"Date": true, "Event": true, "EventPresentation": true,
		"Level": true, "Session": true, "User": true, "UserName": true,
		"UserID": true, "Computer": true, "Application": true,
		"ApplicationPresentation": true, "Connection": true,
		"TransactionStatus": true, "TransactionID": true,
		"MetadataName": true, "MetadataPresentation": true,
		"Comment": true, "Data": true, "DataPresentation": true,
		"ServerName": true, "Port": true, "SyncPort": true,
		"SessionDataSeparation": true,
	}
	return knownFields[key]
}

// readCheckpoint reads checkpoint from YAML file
func (r *IbcmdReader) readCheckpoint() (string, error) {
	// Checkpoint file: <infobase_guid>.dat
	data, err := os.ReadFile(r.checkpointFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Create empty checkpoint file
			checkpoint := map[string]interface{}{
				"checkpoint": "",
			}
			if yamlData, err := yaml.Marshal(checkpoint); err == nil {
				os.WriteFile(r.checkpointFile, yamlData, 0644)
			}
			return "", nil
		}
		return "", err
	}
	
	var checkpoint map[string]interface{}
	if err := yaml.Unmarshal(data, &checkpoint); err != nil {
		return "", err
	}
	
	if cp, ok := checkpoint["checkpoint"].(string); ok {
		return cp, nil
	}
	
	return "", nil
}

// updateCheckpoint updates checkpoint file with last event date
func (r *IbcmdReader) updateCheckpoint(eventTime time.Time) error {
	checkpoint := eventTime.Format("2006-01-02T15:04:05")
	
	// Only update if changed
	if checkpoint == r.lastCheckpoint {
		return nil
	}
	
	data := map[string]interface{}{
		"checkpoint": checkpoint,
	}
	
	yamlData, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	
	if err := os.WriteFile(r.checkpointFile, yamlData, 0644); err != nil {
		return err
	}
	
	r.lastCheckpoint = checkpoint
	return nil
}

// monitorStderr monitors stderr output from ibcmd
func (r *IbcmdReader) monitorStderr(stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		log.Warn().Str("ibcmd_stderr", line).Msg("ibcmd stderr output")
	}
}

