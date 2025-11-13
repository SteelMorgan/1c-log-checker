package writer

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/1c-log-checker/internal/domain"
	"github.com/rs/zerolog/log"
)

// ClickHouseWriter writes records to ClickHouse in batches
type ClickHouseWriter struct {
	conn clickhouse.Conn
	cfg  BatchConfig
	
	eventLogBatch []*domain.EventLogRecord
	techLogBatch  []*domain.TechLogRecord
	
	lastFlush time.Time
	stopCh    chan struct{}
}

// NewClickHouseWriter creates a new ClickHouse batch writer
func NewClickHouseWriter(conn clickhouse.Conn, cfg BatchConfig) *ClickHouseWriter {
	return &ClickHouseWriter{
		conn:          conn,
		cfg:           cfg,
		eventLogBatch: make([]*domain.EventLogRecord, 0, cfg.MaxSize),
		techLogBatch:  make([]*domain.TechLogRecord, 0, cfg.MaxSize),
		lastFlush:     time.Now(),
		stopCh:        make(chan struct{}),
	}
}

// WriteEventLog adds an event log record to the batch
func (w *ClickHouseWriter) WriteEventLog(ctx context.Context, record *domain.EventLogRecord) error {
	w.eventLogBatch = append(w.eventLogBatch, record)
	
	// Check if batch is full or timeout reached
	if len(w.eventLogBatch) >= w.cfg.MaxSize || time.Since(w.lastFlush).Milliseconds() >= w.cfg.FlushTimeout {
		return w.flushEventLog(ctx)
	}
	
	return nil
}

// WriteTechLog adds a tech log record to the batch
func (w *ClickHouseWriter) WriteTechLog(ctx context.Context, record *domain.TechLogRecord) error {
	w.techLogBatch = append(w.techLogBatch, record)
	
	// Check if batch is full or timeout reached
	if len(w.techLogBatch) >= w.cfg.MaxSize || time.Since(w.lastFlush).Milliseconds() >= w.cfg.FlushTimeout {
		return w.flushTechLog(ctx)
	}
	
	return nil
}

// Flush forces writing all pending records
func (w *ClickHouseWriter) Flush(ctx context.Context) error {
	if err := w.flushEventLog(ctx); err != nil {
		return err
	}
	return w.flushTechLog(ctx)
}

// Close flushes and closes the writer
func (w *ClickHouseWriter) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	return w.Flush(ctx)
}

// flushEventLog writes event log batch to ClickHouse
func (w *ClickHouseWriter) flushEventLog(ctx context.Context) error {
	if len(w.eventLogBatch) == 0 {
		return nil
	}
	
	batch, err := w.conn.PrepareBatch(ctx, "INSERT INTO logs.event_log")
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}
	
	for _, record := range w.eventLogBatch {
		err := batch.Append(
			record.EventTime,
			record.ClusterGUID,
			record.ClusterName,
			record.InfobaseGUID,
			record.InfobaseName,
			record.Level,
			record.Event,
			record.EventPresentation,
			record.UserName,
			record.UserID,
			record.Computer,
			record.Application,
			record.ApplicationPresentation,
			record.SessionID,
			record.ConnectionID,
			record.TransactionStatus,
			record.TransactionID,
			record.DataSeparation,
			record.MetadataName,
			record.MetadataPresentation,
			record.Comment,
			record.Data,
			record.DataPresentation,
			record.Server,
			record.PrimaryPort,
			record.SecondaryPort,
			mapToArrays(record.Properties),
		)
		if err != nil {
			return fmt.Errorf("failed to append to batch: %w", err)
		}
	}
	
	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}
	
	log.Info().
		Int("count", len(w.eventLogBatch)).
		Msg("Flushed event log batch to ClickHouse")
	
	w.eventLogBatch = w.eventLogBatch[:0]
	w.lastFlush = time.Now()
	
	return nil
}

// flushTechLog writes tech log batch to ClickHouse
func (w *ClickHouseWriter) flushTechLog(ctx context.Context) error {
	if len(w.techLogBatch) == 0 {
		return nil
	}
	
	batch, err := w.conn.PrepareBatch(ctx, "INSERT INTO logs.tech_log")
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}
	
	for _, record := range w.techLogBatch {
		propKeys, propVals := mapToArrays(record.Properties)
		
		err := batch.Append(
			record.Timestamp,
			record.Duration,
			record.Name,
			record.Level,
			record.Depth,
			record.Process,
			record.OSThread,
			record.ClientID,
			record.SessionID,
			record.TransactionID,
			record.User,
			record.ApplicationID,
			record.ConnectionID,
			record.Interface,
			record.Method,
			record.CallID,
			record.ClusterGUID,
			record.InfobaseGUID,
			record.RawLine,
			propKeys,
			propVals,
		)
		if err != nil {
			return fmt.Errorf("failed to append to batch: %w", err)
		}
	}
	
	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}
	
	log.Info().
		Int("count", len(w.techLogBatch)).
		Msg("Flushed tech log batch to ClickHouse")
	
	w.techLogBatch = w.techLogBatch[:0]
	w.lastFlush = time.Now()
	
	return nil
}

// mapToArrays converts a map to two arrays (keys, values) for ClickHouse Nested type
func mapToArrays(m map[string]string) ([]string, []string) {
	if len(m) == 0 {
		return []string{}, []string{}
	}
	
	keys := make([]string, 0, len(m))
	values := make([]string, 0, len(m))
	
	for k, v := range m {
		keys = append(keys, k)
		values = append(values, v)
	}
	
	return keys, values
}

