package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// DeadLetterQueue stores failed records for later processing
type DeadLetterQueue struct {
	dir      string
	mu       sync.Mutex
	eventLog *os.File
	techLog  *os.File
}

// NewDeadLetterQueue creates a new dead letter queue
func NewDeadLetterQueue(dir string) (*DeadLetterQueue, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create DLQ directory: %w", err)
	}

	dlq := &DeadLetterQueue{dir: dir}

	// Open files for appending
	eventLogPath := filepath.Join(dir, "event_log_dlq.jsonl")
	techLogPath := filepath.Join(dir, "tech_log_dlq.jsonl")

	var err error
	dlq.eventLog, err = os.OpenFile(eventLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open event log DLQ file: %w", err)
	}

	dlq.techLog, err = os.OpenFile(techLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		dlq.eventLog.Close()
		return nil, fmt.Errorf("failed to open tech log DLQ file: %w", err)
	}

	log.Info().
		Str("dir", dir).
		Msg("Dead letter queue initialized")

	return dlq, nil
}

// Record represents a failed record with error context
type Record struct {
	Timestamp time.Time       `json:"timestamp"`
	Type      string          `json:"type"` // "event_log" or "tech_log"
	Record    json.RawMessage `json:"record"`
	Error     string          `json:"error"`
	Attempts  int             `json:"attempts"`
}

// AddEventLog adds a failed event log record to the queue
func (dlq *DeadLetterQueue) AddEventLog(ctx context.Context, record interface{}, err error, attempts int) error {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()

	recordJSON, marshalErr := json.Marshal(record)
	if marshalErr != nil {
		return fmt.Errorf("failed to marshal record: %w", marshalErr)
	}

	dlqRecord := Record{
		Timestamp: time.Now(),
		Type:      "event_log",
		Record:    recordJSON,
		Error:     err.Error(),
		Attempts:  attempts,
	}

	encoder := json.NewEncoder(dlq.eventLog)
	if err := encoder.Encode(dlqRecord); err != nil {
		return fmt.Errorf("failed to write to DLQ: %w", err)
	}

	log.Warn().
		Str("type", "event_log").
		Int("attempts", attempts).
		Err(err).
		Msg("Record added to dead letter queue")

	return nil
}

// AddTechLog adds a failed tech log record to the queue
func (dlq *DeadLetterQueue) AddTechLog(ctx context.Context, record interface{}, err error, attempts int) error {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()

	recordJSON, marshalErr := json.Marshal(record)
	if marshalErr != nil {
		return fmt.Errorf("failed to marshal record: %w", marshalErr)
	}

	dlqRecord := Record{
		Timestamp: time.Now(),
		Type:      "tech_log",
		Record:    recordJSON,
		Error:     err.Error(),
		Attempts:  attempts,
	}

	encoder := json.NewEncoder(dlq.techLog)
	if err := encoder.Encode(dlqRecord); err != nil {
		return fmt.Errorf("failed to write to DLQ: %w", err)
	}

	log.Warn().
		Str("type", "tech_log").
		Int("attempts", attempts).
		Err(err).
		Msg("Record added to dead letter queue")

	return nil
}

// Close closes the dead letter queue
func (dlq *DeadLetterQueue) Close() error {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()

	var errs []error
	if dlq.eventLog != nil {
		if err := dlq.eventLog.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close event log DLQ: %w", err))
		}
	}
	if dlq.techLog != nil {
		if err := dlq.techLog.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close tech log DLQ: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing DLQ: %v", errs)
	}

	return nil
}

