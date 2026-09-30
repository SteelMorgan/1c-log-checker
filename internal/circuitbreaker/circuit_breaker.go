package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// State represents the circuit breaker state
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

// CircuitBreaker implements circuit breaker pattern
type CircuitBreaker struct {
	maxFailures    int
	resetTimeout   time.Duration
	halfOpenMaxOps int

	mu            sync.RWMutex
	state         State
	failureCount  int
	lastFailTime  time.Time
	halfOpenOps   int
	halfOpenFails int
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:    maxFailures,
		resetTimeout:   resetTimeout,
		halfOpenMaxOps: 5, // Allow 5 operations in half-open state
		state:          StateClosed,
	}
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, operation func() error) error {
	// Check if we should allow the operation
	if !cb.allowOperation() {
		return errors.New("circuit breaker is open")
	}

	// Execute operation
	err := operation()

	// Record result
	cb.recordResult(err)

	return err
}

// allowOperation checks if operation should be allowed
func (cb *CircuitBreaker) allowOperation() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if reset timeout has passed
		if time.Since(cb.lastFailTime) >= cb.resetTimeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			// Double-check after acquiring write lock
			if cb.state == StateOpen && time.Since(cb.lastFailTime) >= cb.resetTimeout {
				cb.state = StateHalfOpen
				cb.halfOpenOps = 0
				cb.halfOpenFails = 0
				log.Info().Msg("Circuit breaker transitioning to half-open state")
			}
			cb.mu.Unlock()
			cb.mu.RLock()
			return cb.state == StateHalfOpen
		}
		return false
	case StateHalfOpen:
		// Allow limited operations in half-open state
		return cb.halfOpenOps < cb.halfOpenMaxOps
	default:
		return false
	}
}

// recordResult records the result of an operation
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failureCount++
		cb.lastFailTime = time.Now()

		if cb.state == StateHalfOpen {
			cb.halfOpenFails++
			// If too many failures in half-open, go back to open
			if cb.halfOpenFails >= 2 {
				cb.state = StateOpen
				log.Warn().
					Int("half_open_fails", cb.halfOpenFails).
					Msg("Circuit breaker transitioning back to open state")
			}
		} else if cb.state == StateClosed {
			// Check if we should open the circuit
			if cb.failureCount >= cb.maxFailures {
				cb.state = StateOpen
				log.Warn().
					Int("failures", cb.failureCount).
					Int("max_failures", cb.maxFailures).
					Msg("Circuit breaker opened due to too many failures")
			}
		}
	} else {
		// Success - reset failure count
		if cb.state == StateHalfOpen {
			cb.halfOpenOps++
			// If enough successes in half-open, close the circuit
			if cb.halfOpenOps >= 3 && cb.halfOpenFails == 0 {
				cb.state = StateClosed
				cb.failureCount = 0
				log.Info().Msg("Circuit breaker closed after successful operations")
			}
		} else if cb.state == StateClosed {
			cb.failureCount = 0
		}
	}
}

// GetState returns the current state
func (cb *CircuitBreaker) GetState() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns circuit breaker statistics
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	stateStr := "closed"
	switch cb.state {
	case StateOpen:
		stateStr = "open"
	case StateHalfOpen:
		stateStr = "half-open"
	}

	return map[string]interface{}{
		"state":         stateStr,
		"failure_count": cb.failureCount,
		"last_fail_time": cb.lastFailTime,
		"half_open_ops":  cb.halfOpenOps,
		"half_open_fails": cb.halfOpenFails,
	}
}

