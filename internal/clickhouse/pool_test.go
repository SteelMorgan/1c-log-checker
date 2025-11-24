package clickhouse

import (
	"context"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

func TestNewPool_InvalidConfig(t *testing.T) {
	// Test with invalid config
	_, err := NewPool("invalid-host", 9000, "test", 0, 0)
	if err == nil {
		t.Error("Expected error with invalid config")
	}
}

func TestPool_Get_WithoutConnections(t *testing.T) {
	// Create pool with maxConns = 0 (invalid)
	pool := &Pool{
		maxConns: 0,
		active:   0,
	}
	
	ctx := context.Background()
	_, err := pool.Get(ctx)
	if err == nil {
		t.Error("Get() should return error when pool is exhausted")
	}
}

func TestPool_Put_NilConnection(t *testing.T) {
	pool := &Pool{
		conns:     []clickhouse.Conn{},
		available: []bool{},
	}
	
	// Put nil connection (should not panic)
	pool.Put(nil)
}

func TestPool_Close_EmptyPool(t *testing.T) {
	pool := &Pool{
		conns: []clickhouse.Conn{},
	}
	
	err := pool.Close()
	if err != nil {
		t.Errorf("Close() should not return error for empty pool: %v", err)
	}
}

func TestPool_Stats(t *testing.T) {
	pool := &Pool{
		maxConns: 10,
		created:  5,
		active:   3,
	}
	
	stats := pool.Stats()
	if stats == nil {
		t.Error("Stats() should not return nil")
	}
	
	if stats["max_connections"] != 10 {
		t.Errorf("Expected max_connections 10, got %v", stats["max_connections"])
	}
	
	if stats["created"] != 5 {
		t.Errorf("Expected created 5, got %v", stats["created"])
	}
	
	if stats["active"] != 3 {
		t.Errorf("Expected active 3, got %v", stats["active"])
	}
}

func TestPool_Exhausted(t *testing.T) {
	pool := &Pool{
		maxConns: 2,
		created:  2,
		active:   2,
		conns:    make([]clickhouse.Conn, 2),
		available: []bool{false, false},
	}
	
	ctx := context.Background()
	_, err := pool.Get(ctx)
	if err == nil {
		t.Error("Get() should return error when pool is exhausted")
	}
}

