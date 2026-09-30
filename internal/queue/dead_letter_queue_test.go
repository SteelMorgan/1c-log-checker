package queue

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDeadLetterQueue(t *testing.T) {
	// Create temporary directory
	tmpDir := filepath.Join(os.TempDir(), "dlq_test")
	defer os.RemoveAll(tmpDir)

	dlq, err := NewDeadLetterQueue(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create DLQ: %v", err)
	}
	defer dlq.Close()

	ctx := context.Background()

	// Test AddEventLog
	testRecord := map[string]interface{}{
		"event_time": time.Now(),
		"level":      "Error",
		"message":    "Test error",
	}

	err = dlq.AddEventLog(ctx, testRecord, fmt.Errorf("test error"), 3)
	if err != nil {
		t.Errorf("Failed to add event log: %v", err)
	}

	// Test AddTechLog
	err = dlq.AddTechLog(ctx, testRecord, fmt.Errorf("test error"), 3)
	if err != nil {
		t.Errorf("Failed to add tech log: %v", err)
	}

	// Verify files were created
	eventLogPath := filepath.Join(tmpDir, "event_log_dlq.jsonl")
	techLogPath := filepath.Join(tmpDir, "tech_log_dlq.jsonl")

	if _, err := os.Stat(eventLogPath); os.IsNotExist(err) {
		t.Error("Event log DLQ file was not created")
	}

	if _, err := os.Stat(techLogPath); os.IsNotExist(err) {
		t.Error("Tech log DLQ file was not created")
	}
}

func TestDeadLetterQueueClose(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "dlq_test_close")
	defer os.RemoveAll(tmpDir)

	dlq, err := NewDeadLetterQueue(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create DLQ: %v", err)
	}

	// Close should not error
	if err := dlq.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Double close should not panic
	if err := dlq.Close(); err != nil {
		t.Errorf("Second close failed: %v", err)
	}
}

