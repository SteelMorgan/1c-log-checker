package writer

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"github.com/SteelMorgan/1c-log-checker/internal/domain"
)

// calculateEventLogHash calculates SHA1 hash of event log record
// Hash is computed from all fields that uniquely identify a record
// Using SHA1 instead of SHA256 for better performance (20-30% faster)
// SHA1 is sufficient for deduplication purposes (collision resistance is what matters)
func calculateEventLogHash(record *domain.EventLogRecord) (string, error) {
	h := sha1.New()
	
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

// calculateTechLogHash calculates SHA1 hash of tech log record
// Hash is computed from all fields that uniquely identify a record
// Using SHA1 instead of SHA256 for better performance (20-30% faster)
// SHA1 is sufficient for deduplication purposes (collision resistance is what matters)
func calculateTechLogHash(record *domain.TechLogRecord) (string, error) {
	h := sha1.New()
	
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
	
	// SQL event properties
	fmt.Fprintf(h, "%s|", record.SQL)
	fmt.Fprintf(h, "%s|", record.PlanSQLText)
	fmt.Fprintf(h, "%d|", record.Rows)
	fmt.Fprintf(h, "%d|", record.RowsAffected)
	fmt.Fprintf(h, "%s|", record.DBMS)
	fmt.Fprintf(h, "%s|", record.Database)
	fmt.Fprintf(h, "%s|", record.Dbpid)
	fmt.Fprintf(h, "%s|", record.DBCopy)
	fmt.Fprintf(h, "%d|", record.NParams)
	fmt.Fprintf(h, "%s|", record.MDX)
	fmt.Fprintf(h, "%s|", record.DBConnID)
	fmt.Fprintf(h, "%s|", record.DBConnStr)
	fmt.Fprintf(h, "%s|", record.DBUsr)
	
	// SDBL query properties
	fmt.Fprintf(h, "%s|", record.Query)
	fmt.Fprintf(h, "%s|", record.Sdbl)
	fmt.Fprintf(h, "%s|", record.QueryFields)
	
	// Exception properties
	fmt.Fprintf(h, "%s|", record.Exception)
	fmt.Fprintf(h, "%s|", record.ExceptionDescr)
	fmt.Fprintf(h, "%s|", record.ExceptionContext)
	fmt.Fprintf(h, "%s|", record.Func)
	fmt.Fprintf(h, "%d|", record.Line)
	fmt.Fprintf(h, "%s|", record.File)
	fmt.Fprintf(h, "%s|", record.Module)
	fmt.Fprintf(h, "%s|", record.OSException)
	
	// Lock properties
	fmt.Fprintf(h, "%s|", record.Locks)
	fmt.Fprintf(h, "%s|", record.Regions)
	fmt.Fprintf(h, "%s|", record.WaitConnections)
	fmt.Fprintf(h, "%s|", record.Lka)
	fmt.Fprintf(h, "%s|", record.Lkp)
	fmt.Fprintf(h, "%s|", record.Lkpid)
	fmt.Fprintf(h, "%s|", record.Lkaid)
	fmt.Fprintf(h, "%s|", record.Lksrc)
	fmt.Fprintf(h, "%d|", record.Lkpto)
	fmt.Fprintf(h, "%d|", record.Lkato)
	fmt.Fprintf(h, "%s|", record.DeadlockConnectionIntersections)
	
	// Connection properties
	fmt.Fprintf(h, "%s|", record.Server)
	fmt.Fprintf(h, "%d|", record.Port)
	fmt.Fprintf(h, "%d|", record.SyncPort)
	fmt.Fprintf(h, "%d|", record.Connection)
	
	// Session properties
	fmt.Fprintf(h, "%d|", record.SessionNmb)
	fmt.Fprintf(h, "%s|", record.SeanceID)
	
	// Process properties
	fmt.Fprintf(h, "%s|", record.ProcID)
	fmt.Fprintf(h, "%d|", record.PID)
	fmt.Fprintf(h, "%s|", record.ProcessName)
	fmt.Fprintf(h, "%s|", record.Finish)
	fmt.Fprintf(h, "%d|", record.ExitCode)
	
	// Call properties
	fmt.Fprintf(h, "%s|", record.MName)
	fmt.Fprintf(h, "%s|", record.IName)
	fmt.Fprintf(h, "%d|", record.DstClientID)
	fmt.Fprintf(h, "%s|", record.RetExcp)
	fmt.Fprintf(h, "%d|", record.Memory)
	fmt.Fprintf(h, "%d|", record.MemoryPeak)
	
	// Additional identifying fields
	fmt.Fprintf(h, "%d|", record.DurationUs)
	fmt.Fprintf(h, "%s|", record.Host)
	fmt.Fprintf(h, "%s|", record.Val)
	fmt.Fprintf(h, "%d|", record.Err)
	fmt.Fprintf(h, "%d|", record.Calls)
	fmt.Fprintf(h, "%d|", record.InBytes)
	fmt.Fprintf(h, "%d|", record.OutBytes)
	
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

