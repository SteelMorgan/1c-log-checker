package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/1c-log-checker/internal/retry"
	"github.com/rs/zerolog/log"
)

// Client wraps ClickHouse connection
type Client struct {
	conn      clickhouse.Conn
	retryCfg  retry.Config
}

// NewClient creates a new ClickHouse client with default retry config
func NewClient(host string, port int, database string) (*Client, error) {
	return NewClientWithRetry(host, port, database, retry.DefaultConfig())
}

// NewClientFromConfig creates a new ClickHouse client with retry config from application config
func NewClientFromConfig(host string, port int, database string, maxAttempts int, initialDelayMs int, maxDelayMs int, multiplier float64) (*Client, error) {
	retryCfg := retry.Config{
		MaxAttempts:  maxAttempts,
		InitialDelay: time.Duration(initialDelayMs) * time.Millisecond,
		MaxDelay:     time.Duration(maxDelayMs) * time.Millisecond,
		Multiplier:   multiplier,
		RetryableErrors: retry.DefaultConfig().RetryableErrors, // Use default retryable errors
	}
	return NewClientWithRetry(host, port, database, retryCfg)
}

// NewClientWithRetry creates a new ClickHouse client with custom retry configuration
func NewClientWithRetry(host string, port int, database string, retryCfg retry.Config) (*Client, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", host, port)},
		Auth: clickhouse.Auth{
			Database: database,
			Username: "default",
			Password: "",
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to clickhouse: %w", err)
	}
	
	// Test connection with retry
	ctx := context.Background()
	if err := retry.Do(ctx, retryCfg, func() error {
		return conn.Ping(ctx)
	}); err != nil {
		return nil, fmt.Errorf("failed to ping clickhouse: %w", err)
	}
	
	log.Info().
		Str("host", host).
		Int("port", port).
		Str("database", database).
		Msg("Connected to ClickHouse")
	
	return &Client{
		conn:     conn,
		retryCfg: retryCfg,
	}, nil
}

// Conn returns the underlying ClickHouse connection
func (c *Client) Conn() clickhouse.Conn {
	return c.conn
}

// Close closes the connection
func (c *Client) Close() error {
	log.Info().Msg("Closing ClickHouse connection")
	return c.conn.Close()
}

// Query executes a SELECT query and returns rows with retry logic
func (c *Client) Query(ctx context.Context, query string, args ...interface{}) (driver.Rows, error) {
	return retry.DoWithResult(ctx, c.retryCfg, func() (driver.Rows, error) {
		return c.conn.Query(ctx, query, args...)
	})
}

// Exec executes a non-SELECT query with retry logic
func (c *Client) Exec(ctx context.Context, query string, args ...interface{}) error {
	return retry.Do(ctx, c.retryCfg, func() error {
		return c.conn.Exec(ctx, query, args...)
	})
}

