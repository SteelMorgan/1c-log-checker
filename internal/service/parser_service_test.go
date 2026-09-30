package service

import (
	"context"
	"testing"
	"time"

	"github.com/SteelMorgan/1c-log-checker/internal/config"
)

func TestNewParserService(t *testing.T) {
	// Test with nil config
	_, err := NewParserService(nil)
	if err == nil {
		t.Error("Expected error with nil config")
	}

	// Test with valid config
	cfg := &config.Config{
		ClickHouseHost: "localhost",
		ClickHousePort: 9000,
		ClickHouseDB:   "test",
		ReadOnly:       true, // Don't try to connect
		LogDirs:        []string{},
		TechLogDirs:    []string{},
	}

	service, err := NewParserService(cfg)
	if err != nil {
		t.Fatalf("Failed to create parser service: %v", err)
	}

	if service == nil {
		t.Fatal("Service should not be nil")
	}

	// Test Stop
	if err := service.Stop(); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}

func TestParserServiceGetClickHouseConn(t *testing.T) {
	cfg := &config.Config{
		ClickHouseHost: "localhost",
		ClickHousePort: 9000,
		ClickHouseDB:   "test",
		ReadOnly:       true,
		LogDirs:        []string{},
		TechLogDirs:    []string{},
	}

	service, err := NewParserService(cfg)
	if err != nil {
		t.Fatalf("Failed to create parser service: %v", err)
	}

	// In ReadOnly mode, connection should be nil
	conn := service.GetClickHouseConn()
	if conn != nil {
		t.Error("Expected nil connection in ReadOnly mode")
	}
}

func TestParserServiceContextCancellation(t *testing.T) {
	cfg := &config.Config{
		ClickHouseHost: "localhost",
		ClickHousePort: 9000,
		ClickHouseDB:   "test",
		ReadOnly:       true,
		LogDirs:        []string{},
		TechLogDirs:    []string{},
	}

	service, err := NewParserService(cfg)
	if err != nil {
		t.Fatalf("Failed to create parser service: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Start service in goroutine
	done := make(chan error, 1)
	go func() {
		done <- service.Start(ctx)
	}()

	// Cancel context after short delay
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Wait for service to stop
	select {
	case err := <-done:
		if err != nil && err != context.Canceled {
			t.Errorf("Unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Service did not stop within timeout")
	}

	// Stop service
	if err := service.Stop(); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}
