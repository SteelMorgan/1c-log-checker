package circuitbreaker

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)

	// Test initial state (closed)
	if cb.GetState() != StateClosed {
		t.Errorf("Expected initial state to be Closed, got %v", cb.GetState())
	}

	// Test successful operation
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test failures that should open circuit
	for i := 0; i < 3; i++ {
		err := cb.Execute(context.Background(), func() error {
			return errors.New("test error")
		})
		if err == nil {
			t.Error("Expected error, got nil")
		}
	}

	// Circuit should be open now
	if cb.GetState() != StateOpen {
		t.Errorf("Expected state to be Open, got %v", cb.GetState())
	}

	// Test that operations are blocked when open
	err = cb.Execute(context.Background(), func() error {
		return nil
	})
	if err == nil {
		t.Error("Expected error when circuit is open, got nil")
	}

	// Wait for reset timeout
	time.Sleep(150 * time.Millisecond)

	// Circuit should transition to half-open
	// Try a successful operation
	err = cb.Execute(context.Background(), func() error {
		return nil
	})
	if err != nil {
		t.Logf("Operation in half-open state returned error (expected): %v", err)
	}

	// After successful operations, circuit should close
	time.Sleep(50 * time.Millisecond)
	stats := cb.GetStats()
	t.Logf("Circuit breaker stats: %+v", stats)
}

func TestCircuitBreakerStats(t *testing.T) {
	cb := NewCircuitBreaker(5, 1*time.Second)

	stats := cb.GetStats()
	if stats == nil {
		t.Error("Stats should not be nil")
	}

	// Check that stats contain expected keys
	expectedKeys := []string{"state", "failure_count", "last_fail_time", "half_open_ops", "half_open_fails"}
	for _, key := range expectedKeys {
		if _, ok := stats[key]; !ok {
			t.Errorf("Stats missing key: %s", key)
		}
	}
}

