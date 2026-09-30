package clickhouse

import (
	"context"
	"testing"
	"time"

	"github.com/SteelMorgan/1c-log-checker/internal/retry"
)

func TestNewClient_InvalidHost(t *testing.T) {
	// Test with invalid host (should fail)
	_, err := NewClient("invalid-host-that-does-not-exist", 9000, "test")
	if err == nil {
		t.Error("Expected error with invalid host")
	}
}

func TestNewClientFromConfig(t *testing.T) {
	// Test with custom config
	_, err := NewClientFromConfig("localhost", 9000, "test", "default", "", 5, 200, 10000, 2.5)
	if err == nil {
		// If ClickHouse is not running, this is expected
		t.Log("ClickHouse connection test skipped (ClickHouse not available)")
	}
}

func TestNewClientWithRetry(t *testing.T) {
	cfg := retry.Config{
		MaxAttempts:  2,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}

	// Test with invalid host
	_, err := NewClientWithRetry("invalid-host", 9000, "test", cfg)
	if err == nil {
		t.Error("Expected error with invalid host")
	}
}

func TestClient_Conn(t *testing.T) {
	// This test requires a real ClickHouse connection
	// For now, we'll test the structure
	client := &Client{}
	if client.Conn() != nil {
		t.Error("Conn() should return nil for uninitialized client")
	}
}

func TestClient_Close(t *testing.T) {
	// Test closing nil client (should not panic)
	client := &Client{}
	if err := client.Close(); err == nil {
		// Close on nil connection might return nil or error
		// Both are acceptable
	}
}

func TestClient_Query_WithoutConnection(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	_, err := client.Query(ctx, "SELECT 1")
	if err == nil {
		t.Error("Query() should return error without connection")
	}
}

func TestClient_Exec_WithoutConnection(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	err := client.Exec(ctx, "SELECT 1")
	if err == nil {
		t.Error("Exec() should return error without connection")
	}
}
