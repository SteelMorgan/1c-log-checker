package retry

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Config holds retry configuration
type Config struct {
	MaxAttempts      int           // Maximum number of retry attempts (default: 3)
	InitialDelay     time.Duration // Initial delay before first retry (default: 100ms)
	MaxDelay         time.Duration // Maximum delay between retries (default: 5s)
	Multiplier       float64       // Exponential backoff multiplier (default: 2.0)
	RetryableErrors  []string      // List of error substrings that are retryable
}

// DefaultConfig returns default retry configuration
func DefaultConfig() Config {
	return Config{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		RetryableErrors: []string{
			"connection refused",
			"connection reset",
			"connection lost",
			"timeout",
			"network is unreachable",
			"no such host",
			"temporary failure",
			"code: 999", // ClickHouse: Connection lost
			"code: 241", // ClickHouse: Memory limit exceeded (can be temporary)
			"code: 159", // ClickHouse: Timeout exceeded
			"code: 160", // ClickHouse: Unknown packet from server
		},
	}
}

// IsRetryableError checks if an error is retryable
func IsRetryableError(err error, cfg Config) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Check for network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() || netErr.Temporary() {
			return true
		}
	}

	// Check for connection errors
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	// Check for ClickHouse error codes in error message
	// ClickHouse errors are typically formatted as "code: XXX" or "Code: XXX"
	errStrLower := strings.ToLower(errStr)
	if strings.Contains(errStrLower, "code: 999") || strings.Contains(errStrLower, "connection lost") {
		return true // Connection lost
	}
	if strings.Contains(errStrLower, "code: 241") {
		return true // Memory limit exceeded (can be temporary)
	}
	if strings.Contains(errStrLower, "code: 159") || strings.Contains(errStrLower, "timeout exceeded") {
		return true // Timeout exceeded
	}
	if strings.Contains(errStrLower, "code: 160") {
		return true // Unknown packet from server
	}
	if strings.Contains(errStrLower, "code: 210") {
		return true // Connection pool timeout
	}
	// Don't retry on syntax errors (code: 62), validation errors, etc.
	if strings.Contains(errStrLower, "code: 62") || strings.Contains(errStrLower, "syntax error") {
		return false
	}

	// Check error message for retryable patterns
	for _, pattern := range cfg.RetryableErrors {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// Do executes a function with retry logic
func Do(ctx context.Context, cfg Config, operation func() error) error {
	var lastErr error
	delay := cfg.InitialDelay

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		// Check context cancellation
		if ctx.Err() != nil {
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		}

		// Execute operation
		err := operation()
		if err == nil {
			if attempt > 1 {
				log.Info().
					Int("attempt", attempt).
					Msg("Operation succeeded after retry")
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryableError(err, cfg) {
			log.Debug().
				Err(err).
				Int("attempt", attempt).
				Msg("Error is not retryable, aborting")
			return err
		}

		// Don't retry on last attempt
		if attempt >= cfg.MaxAttempts {
			log.Warn().
				Err(err).
				Int("attempt", attempt).
				Int("max_attempts", cfg.MaxAttempts).
				Msg("Max retry attempts reached")
			return fmt.Errorf("operation failed after %d attempts: %w", cfg.MaxAttempts, err)
		}

		// Log retry attempt
		log.Warn().
			Err(err).
			Int("attempt", attempt).
			Int("max_attempts", cfg.MaxAttempts).
			Dur("retry_delay", delay).
			Msg("Operation failed, retrying")

		// Wait before retry
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next attempt
		}

		// Calculate next delay with exponential backoff
		delay = time.Duration(float64(delay) * cfg.Multiplier)
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", cfg.MaxAttempts, lastErr)
}

// DoWithResult executes a function that returns a result with retry logic
func DoWithResult[T any](ctx context.Context, cfg Config, operation func() (T, error)) (T, error) {
	var zero T
	var lastErr error
	delay := cfg.InitialDelay

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		// Check context cancellation
		if ctx.Err() != nil {
			return zero, fmt.Errorf("context cancelled: %w", ctx.Err())
		}

		// Execute operation
		result, err := operation()
		if err == nil {
			if attempt > 1 {
				log.Info().
					Int("attempt", attempt).
					Msg("Operation succeeded after retry")
			}
			return result, nil
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryableError(err, cfg) {
			log.Debug().
				Err(err).
				Int("attempt", attempt).
				Msg("Error is not retryable, aborting")
			return zero, err
		}

		// Don't retry on last attempt
		if attempt >= cfg.MaxAttempts {
			log.Warn().
				Err(err).
				Int("attempt", attempt).
				Int("max_attempts", cfg.MaxAttempts).
				Msg("Max retry attempts reached")
			return zero, fmt.Errorf("operation failed after %d attempts: %w", cfg.MaxAttempts, err)
		}

		// Log retry attempt
		log.Warn().
			Err(err).
			Int("attempt", attempt).
			Int("max_attempts", cfg.MaxAttempts).
			Dur("retry_delay", delay).
			Msg("Operation failed, retrying")

		// Wait before retry
		select {
		case <-ctx.Done():
			return zero, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next attempt
		}

		// Calculate next delay with exponential backoff
		delay = time.Duration(float64(delay) * cfg.Multiplier)
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
	}

	return zero, fmt.Errorf("operation failed after %d attempts: %w", cfg.MaxAttempts, lastErr)
}

