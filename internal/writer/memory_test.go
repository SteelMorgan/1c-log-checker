package writer

import (
	"context"
	"fmt"
	"github.com/SteelMorgan/1c-log-checker/internal/retry"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/SteelMorgan/1c-log-checker/internal/domain"
)

type memoryTestConn struct {
	driver.Conn
	sends  atomic.Int32
	fail   atomic.Bool
	active atomic.Int32
	peak   atomic.Int32
}
type memoryTestBatch struct {
	driver.Batch
	conn *memoryTestConn
}

func (c *memoryTestConn) PrepareBatch(context.Context, string, ...driver.PrepareBatchOption) (driver.Batch, error) {
	return &memoryTestBatch{conn: c}, nil
}
func (b *memoryTestBatch) Append(...any) error { return nil }
func (b *memoryTestBatch) Send() error {
	n := b.conn.active.Add(1)
	defer b.conn.active.Add(-1)
	for old := b.conn.peak.Load(); n > old; old = b.conn.peak.Load() {
		if b.conn.peak.CompareAndSwap(old, n) {
			break
		}
	}
	time.Sleep(time.Millisecond)
	if b.conn.fail.Load() {
		return fmt.Errorf("simulated Send failure")
	}
	b.conn.sends.Add(1)
	return nil
}

func TestEventLogBatchFlushesByBytesAndReleasesPointers(t *testing.T) {
	c := &memoryTestConn{}
	w := NewClickHouseWriter(c, BatchConfig{MaxSize: 5000, FlushTimeout: 3600000})
	record := &domain.EventLogRecord{EventTime: time.Now(), Comment: strings.Repeat("x", 5<<20)}
	ctx := context.Background()
	if err := w.WriteEventLog(ctx, record); err != nil {
		t.Fatal(err)
	}
	if c.sends.Load() != 0 {
		t.Fatal("first record should fit")
	}
	if err := w.WriteEventLog(ctx, record); err != nil {
		t.Fatal(err)
	}
	if c.sends.Load() != 1 || len(w.eventLogBatch) != 1 {
		t.Fatal("byte budget did not flush before second record")
	}
	if err := w.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	if w.eventLogBytes != 0 || len(w.eventLogBatch) != 0 {
		t.Fatal("batch budget retained")
	}
	for _, record := range w.eventLogBatch[:cap(w.eventLogBatch)] {
		if record != nil {
			t.Fatal("backing slice retained record")
		}
	}
}

func TestWriterAllowsOnlyOneFlushInFlight(t *testing.T) {
	c := &memoryTestConn{}
	w := NewClickHouseWriter(c, BatchConfig{MaxSize: 1, FlushTimeout: 3600000})
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := w.WriteEventLog(context.Background(), &domain.EventLogRecord{EventTime: time.Now()}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if err := w.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if c.peak.Load() != 1 || c.sends.Load() != 16 {
		t.Fatalf("concurrent flushes=%d sends=%d", c.peak.Load(), c.sends.Load())
	}
}

func TestFailedByteFlushRetainsOldBatchAndDoesNotAcceptNewRecord(t *testing.T) {
	c := &memoryTestConn{}
	w := NewClickHouseWriterWithRetry(c, BatchConfig{MaxSize: 5000, FlushTimeout: 3600000}, retry.Config{MaxAttempts: 1})
	ctx := context.Background()
	first := &domain.EventLogRecord{EventTime: time.Now(), Comment: strings.Repeat("x", 5<<20)}
	second := &domain.EventLogRecord{EventTime: time.Now(), Comment: strings.Repeat("y", 5<<20)}
	if err := w.WriteEventLog(ctx, first); err != nil {
		t.Fatal(err)
	}
	before := w.eventLogBytes
	c.fail.Store(true)
	if err := w.WriteEventLog(ctx, second); err == nil {
		t.Fatal("Send error hidden")
	}
	if len(w.eventLogBatch) != 1 || w.eventLogBatch[0].Comment != first.Comment || w.eventLogBytes != before {
		t.Fatal("failed batch lost or current record accepted")
	}
	if err := w.Flush(ctx); err == nil {
		t.Fatal("Flush error hidden")
	}
	if len(w.eventLogBatch) != 1 {
		t.Fatal("Flush lost failed batch")
	}
	c.fail.Store(false)
	if err := w.WriteEventLog(ctx, second); err != nil {
		t.Fatal(err)
	}
	if len(w.eventLogBatch) != 1 || w.eventLogBatch[0].Comment != second.Comment || c.sends.Load() != 1 {
		t.Fatal("retry duplicated or lost records")
	}
	if err := w.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	if c.sends.Load() != 2 {
		t.Fatal("not all accepted batches delivered")
	}
}

func TestBatchBudgetCountsRetainedSource(t *testing.T) {
	record := &domain.EventLogRecord{Comment: "small", SourceBytes: 16 << 20}
	if got := recordMemoryBytes(record); got < 32<<20 {
		t.Fatalf("retained source underestimated: %d", got)
	}
}
