package writer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"github.com/1c-log-checker/internal/domain"
)

// calculateEventLogHash calculates SHA256 hash of event log record
// Hash is computed from all fields that uniquely identify a record
func calculateEventLogHash(record *domain.EventLogRecord) (string, error) {
	h := sha256.New()
	
	// Write all identifying fields to hash
	fmt.Fprintf(h, "%s|", record.EventTime.Format(time.RFC3339Nano))
	fmt.Fprintf(h, "%s|", record.ClusterGUID)
	fmt.Fprintf(h, "%s|", record.InfobaseGUID)
	fmt.Fprintf(h, "%s|", record.Level)
	fmt.Fprintf(h, "%s|", record.Event)
	fmt.Fprintf(h, "%s|", record.EventPresentation)
	fmt.Fprintf(h, "%s|", record.UserName)
	fmt.Fprintf(h, "%s|", record.Computer)
	fmt.Fprintf(h, "%s|", record.Application)
	fmt.Fprintf(h, "%d|", record.SessionID)
	fmt.Fprintf(h, "%d|", record.ConnectionID)
	fmt.Fprintf(h, "%s|", record.TransactionStatus)
	fmt.Fprintf(h, "%s|", record.TransactionID)
	fmt.Fprintf(h, "%s|", record.TransactionDateTime.Format(time.RFC3339Nano))
	fmt.Fprintf(h, "%d|", record.TransactionNumber)
	fmt.Fprintf(h, "%s|", record.DataSeparation)
	fmt.Fprintf(h, "%s|", record.MetadataName)
	fmt.Fprintf(h, "%s|", record.MetadataPresentation)
	fmt.Fprintf(h, "%s|", record.Comment)
	fmt.Fprintf(h, "%s|", record.Data)
	fmt.Fprintf(h, "%s|", record.DataPresentation)
	fmt.Fprintf(h, "%s|", record.Server)
	fmt.Fprintf(h, "%d|", record.PrimaryPort)
	fmt.Fprintf(h, "%d|", record.SecondaryPort)
	
	// Include properties (sorted for consistency)
	keys := make([]string, 0, len(record.Properties))
	for k := range record.Properties {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(h, "%s=%s|", k, record.Properties[k])
	}
	
	hashBytes := h.Sum(nil)
	return hex.EncodeToString(hashBytes), nil
}

// calculateTechLogHash calculates SHA256 hash of tech log record
// Hash is computed from all fields that uniquely identify a record
func calculateTechLogHash(record *domain.TechLogRecord) (string, error) {
	h := sha256.New()
	
	// Write all identifying fields to hash
	fmt.Fprintf(h, "%s|", record.Timestamp.Format(time.RFC3339Nano))
	fmt.Fprintf(h, "%d|", record.Duration)
	fmt.Fprintf(h, "%s|", record.Name)
	fmt.Fprintf(h, "%s|", record.Level)
	fmt.Fprintf(h, "%d|", record.Depth)
	fmt.Fprintf(h, "%s|", record.Process)
	fmt.Fprintf(h, "%d|", record.OSThread)
	fmt.Fprintf(h, "%d|", record.ClientID)
	fmt.Fprintf(h, "%s|", record.SessionID)
	fmt.Fprintf(h, "%s|", record.TransactionID)
	fmt.Fprintf(h, "%s|", record.User)
	fmt.Fprintf(h, "%s|", record.ApplicationID)
	fmt.Fprintf(h, "%d|", record.ConnectionID)
	fmt.Fprintf(h, "%s|", record.Interface)
	fmt.Fprintf(h, "%s|", record.Method)
	fmt.Fprintf(h, "%d|", record.CallID)
	fmt.Fprintf(h, "%s|", record.ClusterGUID)
	fmt.Fprintf(h, "%s|", record.InfobaseGUID)
	fmt.Fprintf(h, "%s|", record.RawLine)
	
	// Include properties (sorted for consistency)
	keys := make([]string, 0, len(record.Properties))
	for k := range record.Properties {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(h, "%s=%s|", k, record.Properties[k])
	}
	
	hashBytes := h.Sum(nil)
	return hex.EncodeToString(hashBytes), nil
}

