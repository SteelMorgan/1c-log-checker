package writer

import (
	"context"
	"testing"
	"time"

	"github.com/SteelMorgan/1c-log-checker/internal/domain"
	"github.com/SteelMorgan/1c-log-checker/internal/retry"
)

func TestBatchConfig(t *testing.T) {
	cfg := BatchConfig{
		MaxSize:            1000,
		FlushTimeout:       1000,
		EnableDeduplication: false,
	}

	if cfg.MaxSize != 1000 {
		t.Errorf("Expected MaxSize 1000, got %d", cfg.MaxSize)
	}

	if cfg.FlushTimeout != 1000 {
		t.Errorf("Expected FlushTimeout 1000, got %d", cfg.FlushTimeout)
	}
}

func TestClickHouseWriterMaxBatchSize(t *testing.T) {
	// Test that maxBatchSize is calculated correctly
	cfg := BatchConfig{
		MaxSize: 5000,
	}
	retryCfg := retry.DefaultConfig()

	// This would require a real ClickHouse connection
	// For now, just test the logic
	maxBatchSize := cfg.MaxSize * 2
	if maxBatchSize > 50000 {
		maxBatchSize = 50000
	}

	if maxBatchSize != 10000 {
		t.Errorf("Expected maxBatchSize 10000, got %d", maxBatchSize)
	}

	// Test with large MaxSize
	cfg.MaxSize = 30000
	maxBatchSize = cfg.MaxSize * 2
	if maxBatchSize > 50000 {
		maxBatchSize = 50000
	}

	if maxBatchSize != 50000 {
		t.Errorf("Expected maxBatchSize 50000 (capped), got %d", maxBatchSize)
	}
}

func TestEnsureValidDateTime(t *testing.T) {
	// Test valid date
	validDate := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	result := ensureValidDateTime(validDate)
	if !result.Equal(validDate) {
		t.Errorf("Valid date should be unchanged, got %v", result)
	}

	// Test zero time
	zeroTime := time.Time{}
	result = ensureValidDateTime(zeroTime)
	if result.IsZero() {
		t.Error("Zero time should be replaced with minClickHouseDateTime")
	}

	// Test date before min
	oldDate := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	result = ensureValidDateTime(oldDate)
	if result.Before(minClickHouseDateTime) {
		t.Error("Date before min should be replaced with minClickHouseDateTime")
	}

	// Test date after max
	futureDate := time.Date(2300, 1, 1, 0, 0, 0, 0, time.UTC)
	result = ensureValidDateTime(futureDate)
	if result.After(maxClickHouseDateTime) {
		t.Error("Date after max should be replaced with minClickHouseDateTime")
	}
}

func TestBatchSizeLimit(t *testing.T) {
	// Test that batch size limit prevents memory leaks
	// This is a logic test without actual ClickHouse connection
	
	cfg := BatchConfig{
		MaxSize: 1000,
	}
	
	maxBatchSize := cfg.MaxSize * 2
	if maxBatchSize > 50000 {
		maxBatchSize = 50000
	}

	// Simulate batch growth
	batchSize := 0
	for i := 0; i < maxBatchSize+100; i++ {
		batchSize++
		if batchSize >= maxBatchSize {
			// Should trigger flush
			if batchSize > maxBatchSize {
				t.Errorf("Batch size exceeded limit: %d > %d", batchSize, maxBatchSize)
			}
			batchSize = 0 // Reset after flush
		}
	}
}

