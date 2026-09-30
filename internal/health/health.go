package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/rs/zerolog/log"
)

// Checker performs health checks
type Checker struct {
	chConn      clickhouse.Conn
	mu          sync.RWMutex
	lastCheck   time.Time
	status      Status
	details     map[string]interface{}
	checkInterval time.Duration
}

// Status represents health status
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

// NewChecker creates a new health checker
func NewChecker(chConn clickhouse.Conn) *Checker {
	return &Checker{
		chConn:        chConn,
		status:        StatusHealthy,
		details:       make(map[string]interface{}),
		checkInterval: 10 * time.Second,
	}
}

// Check performs a health check
func (c *Checker) Check(ctx context.Context) Status {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Don't check too frequently
	if time.Since(c.lastCheck) < c.checkInterval {
		return c.status
	}

	c.lastCheck = time.Now()
	c.details = make(map[string]interface{})

	// Check ClickHouse connection
	if c.chConn == nil {
		c.status = StatusUnhealthy
		c.details["clickhouse"] = "not connected"
		return c.status
	}

	// Test ClickHouse connection
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var result uint8
	err := c.chConn.QueryRow(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		c.status = StatusUnhealthy
		c.details["clickhouse"] = fmt.Sprintf("query failed: %v", err)
		log.Warn().Err(err).Msg("Health check: ClickHouse query failed")
		return c.status
	}

	if result != 1 {
		c.status = StatusDegraded
		c.details["clickhouse"] = "unexpected query result"
		return c.status
	}

	// All checks passed
	c.status = StatusHealthy
	c.details["clickhouse"] = "connected"
	c.details["timestamp"] = time.Now().Format(time.RFC3339)

	return c.status
}

// GetStatus returns current health status
func (c *Checker) GetStatus() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// GetDetails returns health check details
func (c *Checker) GetDetails() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	details := make(map[string]interface{})
	for k, v := range c.details {
		details[k] = v
	}
	return details
}

// HTTPHandler returns an HTTP handler for health checks
func (c *Checker) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		status := c.Check(ctx)
		details := c.GetDetails()

		var statusCode int
		switch status {
		case StatusHealthy:
			statusCode = http.StatusOK
		case StatusDegraded:
			statusCode = http.StatusOK // Still 200, but indicates degraded state
		case StatusUnhealthy:
			statusCode = http.StatusServiceUnavailable
		}

		response := map[string]interface{}{
			"status":  string(status),
			"details": details,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(response)
	}
}

