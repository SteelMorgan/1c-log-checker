package techlog

import (
	"testing"
	"time"
)

func TestParseJSONLine_EmptyLine(t *testing.T) {
	_, err := ParseJSONLine("")
	if err == nil {
		t.Error("Expected error for empty line")
	}
}

func TestParseJSONLine_InvalidJSON(t *testing.T) {
	_, err := ParseJSONLine("not json")
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestParseJSONLine_ValidJSON(t *testing.T) {
	line := `{"ts":"2025-01-15T10:30:45.123456","duration":"1000","name":"SCALL","level":"INFO","depth":"1"}`
	
	record, err := ParseJSONLine(line)
	if err != nil {
		t.Fatalf("ParseJSONLine() failed: %v", err)
	}
	
	if record == nil {
		t.Fatal("Record should not be nil")
	}
	
	if record.Name != "SCALL" {
		t.Errorf("Expected Name 'SCALL', got '%s'", record.Name)
	}
	
	if record.Level != "INFO" {
		t.Errorf("Expected Level 'INFO', got '%s'", record.Level)
	}
	
	if record.Duration != 1000 {
		t.Errorf("Expected Duration 1000, got %d", record.Duration)
	}
}

func TestParseJSONLine_Timestamp(t *testing.T) {
	line := `{"ts":"2025-01-15T10:30:45.123456","name":"TEST"}`
	
	record, err := ParseJSONLine(line)
	if err != nil {
		t.Fatalf("ParseJSONLine() failed: %v", err)
	}
	
	expectedTime := time.Date(2025, 1, 15, 10, 30, 45, 123456000, time.UTC)
	if !record.Timestamp.Equal(expectedTime) {
		t.Errorf("Expected timestamp %v, got %v", expectedTime, record.Timestamp)
	}
}

func TestParseJSONLine_AllFields(t *testing.T) {
	line := `{
		"ts":"2025-01-15T10:30:45.123456",
		"duration":"5000",
		"name":"SCALL",
		"level":"ERROR",
		"depth":"2",
		"process":"rphost",
		"OSThread":"1234",
		"ClientID":"5678",
		"SessionID":"session-123",
		"Trans":"trans-456",
		"Usr":"user1",
		"AppID":"app-789",
		"ConnID":"9999",
		"Interface":"COM",
		"Method":"TestMethod",
		"CallID":"1111"
	}`
	
	record, err := ParseJSONLine(line)
	if err != nil {
		t.Fatalf("ParseJSONLine() failed: %v", err)
	}
	
	if record.Process != "rphost" {
		t.Errorf("Expected Process 'rphost', got '%s'", record.Process)
	}
	
	if record.SessionID != "session-123" {
		t.Errorf("Expected SessionID 'session-123', got '%s'", record.SessionID)
	}
	
	if record.User != "user1" {
		t.Errorf("Expected User 'user1', got '%s'", record.User)
	}
}

func TestParseJSONLine_BOM(t *testing.T) {
	// Test with BOM prefix
	line := "\ufeff{\"ts\":\"2025-01-15T10:30:45.123456\",\"name\":\"TEST\"}"
	
	record, err := ParseJSONLine(line)
	if err != nil {
		t.Fatalf("ParseJSONLine() failed with BOM: %v", err)
	}
	
	if record.Name != "TEST" {
		t.Errorf("Expected Name 'TEST', got '%s'", record.Name)
	}
}

func TestParseJSONLine_InvalidTimestamp(t *testing.T) {
	line := `{"ts":"invalid-timestamp","name":"TEST"}`
	
	_, err := ParseJSONLine(line)
	if err == nil {
		t.Error("Expected error for invalid timestamp")
	}
}

func TestParseJSONLine_InvalidDuration(t *testing.T) {
	line := `{"ts":"2025-01-15T10:30:45.123456","duration":"invalid","name":"TEST"}`
	
	_, err := ParseJSONLine(line)
	if err == nil {
		t.Error("Expected error for invalid duration")
	}
}

func TestParseJSONLine_TransactionID(t *testing.T) {
	// Test with "Trans" field
	line1 := `{"ts":"2025-01-15T10:30:45.123456","Trans":"trans-123","name":"TEST"}`
	record1, err := ParseJSONLine(line1)
	if err != nil {
		t.Fatalf("ParseJSONLine() failed: %v", err)
	}
	if record1.TransactionID != "trans-123" {
		t.Errorf("Expected TransactionID 'trans-123', got '%s'", record1.TransactionID)
	}
	
	// Test with "TransactionID" field
	line2 := `{"ts":"2025-01-15T10:30:45.123456","TransactionID":"trans-456","name":"TEST"}`
	record2, err := ParseJSONLine(line2)
	if err != nil {
		t.Fatalf("ParseJSONLine() failed: %v", err)
	}
	if record2.TransactionID != "trans-456" {
		t.Errorf("Expected TransactionID 'trans-456', got '%s'", record2.TransactionID)
	}
}

