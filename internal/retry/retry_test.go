package retry

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	
	if cfg.MaxAttempts != 3 {
		t.Errorf("Expected MaxAttempts 3, got %d", cfg.MaxAttempts)
	}
	
	if cfg.InitialDelay != 100*time.Millisecond {
		t.Errorf("Expected InitialDelay 100ms, got %v", cfg.InitialDelay)
	}
	
	if cfg.MaxDelay != 5*time.Second {
		t.Errorf("Expected MaxDelay 5s, got %v", cfg.MaxDelay)
	}
	
	if cfg.Multiplier != 2.0 {
		t.Errorf("Expected Multiplier 2.0, got %f", cfg.Multiplier)
	}
	
	if len(cfg.RetryableErrors) == 0 {
		t.Error("RetryableErrors should not be empty")
	}
}

func TestIsRetryableError(t *testing.T) {
	cfg := DefaultConfig()
	
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "network timeout error",
			err:      &net.OpError{Op: "read", Err: &net.DNSError{IsTimeout: true}},
			expected: true,
		},
		{
			name:     "connection refused",
			err:      errors.New("connection refused"),
			expected: true,
		},
		{
			name:     "ClickHouse code 999",
			err:      errors.New("code: 999 connection lost"),
			expected: true,
		},
		{
			name:     "ClickHouse code 241",
			err:      errors.New("code: 241 memory limit exceeded"),
			expected: true,
		},
		{
			name:     "ClickHouse code 159",
			err:      errors.New("code: 159 timeout exceeded"),
			expected: true,
		},
		{
			name:     "ClickHouse code 62 (syntax error - not retryable)",
			err:      errors.New("code: 62 syntax error"),
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("some other error"),
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.err, cfg)
			if result != tt.expected {
				t.Errorf("IsRetryableError() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDo_Success(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxAttempts = 3
	
	ctx := context.Background()
	attempts := 0
	
	err := Do(ctx, cfg, func() error {
		attempts++
		return nil
	})
	
	if err != nil {
		t.Errorf("Do() returned error: %v", err)
	}
	
	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

func TestDo_RetryOnError(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxAttempts = 3
	cfg.InitialDelay = 10 * time.Millisecond
	
	ctx := context.Background()
	attempts := 0
	
	err := Do(ctx, cfg, func() error {
		attempts++
		if attempts < 2 {
			return errors.New("connection refused")
		}
		return nil
	})
	
	if err != nil {
		t.Errorf("Do() returned error: %v", err)
	}
	
	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestDo_MaxAttemptsExceeded(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxAttempts = 3
	cfg.InitialDelay = 10 * time.Millisecond
	
	ctx := context.Background()
	attempts := 0
	
	err := Do(ctx, cfg, func() error {
		attempts++
		return errors.New("connection refused")
	})
	
	if err == nil {
		t.Error("Do() should return error after max attempts")
	}
	
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestDo_ContextCancellation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxAttempts = 10
	cfg.InitialDelay = 100 * time.Millisecond
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	attempts := 0
	
	// Cancel context after first attempt
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	
	err := Do(ctx, cfg, func() error {
		attempts++
		return errors.New("connection refused")
	})
	
	if err == nil {
		t.Error("Do() should return error on context cancellation")
	}
	
	if attempts < 1 {
		t.Errorf("Expected at least 1 attempt, got %d", attempts)
	}
}

func TestDoWithResult_Success(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxAttempts = 3
	
	ctx := context.Background()
	
	result, err := DoWithResult(ctx, cfg, func() (int, error) {
		return 42, nil
	})
	
	if err != nil {
		t.Errorf("DoWithResult() returned error: %v", err)
	}
	
	if result != 42 {
		t.Errorf("Expected result 42, got %d", result)
	}
}

func TestDoWithResult_RetryOnError(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxAttempts = 3
	cfg.InitialDelay = 10 * time.Millisecond
	
	ctx := context.Background()
	attempts := 0
	
	result, err := DoWithResult(ctx, cfg, func() (int, error) {
		attempts++
		if attempts < 2 {
			return 0, errors.New("connection refused")
		}
		return 100, nil
	})
	
	if err != nil {
		t.Errorf("DoWithResult() returned error: %v", err)
	}
	
	if result != 100 {
		t.Errorf("Expected result 100, got %d", result)
	}
	
	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

