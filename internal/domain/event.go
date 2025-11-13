package domain

import "time"

// EventLogRecord represents a single record from 1C Event Log (Журнал регистрации)
type EventLogRecord struct {
	EventTime         time.Time
	ClusterGUID       string
	ClusterName       string
	InfobaseGUID      string
	InfobaseName      string
	Level             string // Error, Warning, Information, Note
	Event             string
	User              string
	Computer          string
	Application       string
	ConnectionID      uint64
	SessionID         uint64
	TransactionID     string
	Metadata          string
	Comment           string
	Data              string
	DataPresentation  string
	Server            string
	Port              uint16
	Properties        map[string]string // Additional properties
}

