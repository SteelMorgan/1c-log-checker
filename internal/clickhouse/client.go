package clickhouse

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/rs/zerolog/log"
)

// Client wraps ClickHouse connection
type Client struct {
	conn clickhouse.Conn
}

// NewClient creates a new ClickHouse client
func NewClient(host string, port int, database string) (*Client, error) {
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
	
	// Test connection
	if err := conn.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping clickhouse: %w", err)
	}
	
	log.Info().
		Str("host", host).
		Int("port", port).
		Str("database", database).
		Msg("Connected to ClickHouse")
	
	return &Client{conn: conn}, nil
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

// Query executes a SELECT query and returns rows
func (c *Client) Query(ctx context.Context, query string, args ...interface{}) (clickhouse.Rows, error) {
	return c.conn.Query(ctx, query, args...)
}

// Exec executes a non-SELECT query
func (c *Client) Exec(ctx context.Context, query string, args ...interface{}) error {
	return c.conn.Exec(ctx, query, args...)
}

