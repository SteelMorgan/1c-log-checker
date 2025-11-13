package domain

import "time"

// TechLogRecord represents a single record from 1C Tech Log (Технологический журнал)
type TechLogRecord struct {
	// Core fields (present in all events)
	Timestamp    time.Time
	Duration     uint64 // microseconds
	Name         string // Event name (PROC, CONN, DBMSSQL, etc.)
	Level        string // INFO, WARN, ERROR
	Depth        uint8  // Nesting level in call stack
	Process      string // Process name
	OSThread     uint32 // OS thread ID
	
	// Common fields (may be absent in some events)
	ClientID      uint64
	SessionID     string
	TransactionID string
	User          string
	ApplicationID string
	ConnectionID  uint64
	Interface     string
	Method        string
	CallID        uint64
	
	// Cluster/Infobase identification
	ClusterGUID   string
	InfobaseGUID  string
	
	// Raw line for forensics
	RawLine string
	
	// Dynamic properties (all other event-specific fields)
	// Stored as key-value map to handle arbitrary tech log properties
	Properties map[string]string
}

