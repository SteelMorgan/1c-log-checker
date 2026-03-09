package clickhouse

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/SteelMorgan/1c-log-checker/internal/retry"
	"github.com/rs/zerolog/log"
)

// Pool manages a pool of ClickHouse connections
type Pool struct {
	host     string
	port     int
	database string
	user     string
	password string
	retryCfg retry.Config
	maxConns int
	minConns int

	mu        sync.RWMutex
	conns     []clickhouse.Conn
	available []bool
	active    int
	created   int
}

// NewPool creates a new connection pool
func NewPool(host string, port int, database string, retryCfg retry.Config, maxConns, minConns int) (*Pool, error) {
	return NewPoolWithAuth(host, port, database, "default", "", retryCfg, maxConns, minConns)
}

// NewPoolWithAuth creates a new connection pool with explicit credentials.
func NewPoolWithAuth(host string, port int, database string, user string, password string, retryCfg retry.Config, maxConns, minConns int) (*Pool, error) {
	if maxConns < 1 {
		maxConns = 10 // Default
	}
	if minConns < 1 {
		minConns = 2 // Default
	}
	if minConns > maxConns {
		minConns = maxConns
	}

	pool := &Pool{
		host:      host,
		port:      port,
		database:  database,
		user:      user,
		password:  password,
		retryCfg:  retryCfg,
		maxConns:  maxConns,
		minConns:  minConns,
		conns:     make([]clickhouse.Conn, 0, maxConns),
		available: make([]bool, 0, maxConns),
	}

	// Create initial connections
	for i := 0; i < minConns; i++ {
		conn, err := pool.createConnection()
		if err != nil {
			// Close already created connections
			pool.Close()
			return nil, fmt.Errorf("failed to create initial connection %d: %w", i, err)
		}
		pool.conns = append(pool.conns, conn)
		pool.available = append(pool.available, true)
		pool.created++
	}

	log.Info().
		Int("min_connections", minConns).
		Int("max_connections", maxConns).
		Int("initial_connections", minConns).
		Msg("ClickHouse connection pool created")

	return pool, nil
}

// createConnection creates a new ClickHouse connection
func (p *Pool) createConnection() (clickhouse.Conn, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", p.host, p.port)},
		Auth: clickhouse.Auth{
			Database: p.database,
			Username: p.user,
			Password: p.password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Ping(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping: %w", err)
	}

	return conn, nil
}

// Get gets a connection from the pool
func (p *Pool) Get(ctx context.Context) (clickhouse.Conn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Try to find an available connection
	for i, conn := range p.conns {
		if p.available[i] {
			p.available[i] = false
			p.active++
			return conn, nil
		}
	}

	// No available connection - create new one if under limit
	if p.created < p.maxConns {
		conn, err := p.createConnection()
		if err != nil {
			// Connection creation failed - return error
			return nil, fmt.Errorf("failed to create new connection: %w", err)
		}
		// Successfully created - add to pool
		p.conns = append(p.conns, conn)
		p.available = append(p.available, false)
		p.created++
		p.active++
		log.Debug().
			Int("total_connections", p.created).
			Int("active_connections", p.active).
			Msg("Created new connection in pool")
		return conn, nil
	}

	// Pool exhausted - wait for a connection to become available
	// In a production system, you might want to implement a wait queue
	// For now, return error
	return nil, fmt.Errorf("connection pool exhausted (max: %d, active: %d)", p.maxConns, p.active)
}

// Put returns a connection to the pool
func (p *Pool) Put(conn clickhouse.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Find the connection in the pool
	for i, c := range p.conns {
		if c == conn {
			if !p.available[i] {
				p.available[i] = true
				p.active--
			}
			return
		}
	}

	// Connection not found in pool - might be a new connection that wasn't tracked
	// This shouldn't happen, but log it
	log.Warn().Msg("Returned connection not found in pool")
}

// Close closes all connections in the pool
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var errs []error
	for i, conn := range p.conns {
		if conn != nil {
			if err := conn.Close(); err != nil {
				errs = append(errs, fmt.Errorf("failed to close connection %d: %w", i, err))
			}
		}
	}

	p.conns = nil
	p.available = nil
	p.active = 0
	p.created = 0

	if len(errs) > 0 {
		return fmt.Errorf("errors closing pool: %v", errs)
	}

	return nil
}

// Stats returns pool statistics
func (p *Pool) Stats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"total_connections":     p.created,
		"active_connections":    p.active,
		"available_connections": p.created - p.active,
		"max_connections":       p.maxConns,
		"min_connections":       p.minConns,
	}
}

// Execute executes a function with a connection from the pool
func (p *Pool) Execute(ctx context.Context, fn func(conn clickhouse.Conn) error) error {
	conn, err := p.Get(ctx)
	if err != nil {
		return err
	}
	defer p.Put(conn)

	return fn(conn)
}

// Query executes a query using a connection from the pool
func (p *Pool) Query(ctx context.Context, query string, args ...interface{}) (driver.Rows, error) {
	var rows driver.Rows
	err := p.Execute(ctx, func(conn clickhouse.Conn) error {
		var err error
		rows, err = conn.Query(ctx, query, args...)
		return err
	})
	return rows, err
}

// Exec executes a non-SELECT query using a connection from the pool
func (p *Pool) Exec(ctx context.Context, query string, args ...interface{}) error {
	return p.Execute(ctx, func(conn clickhouse.Conn) error {
		return conn.Exec(ctx, query, args...)
	})
}
