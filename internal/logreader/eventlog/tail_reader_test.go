package eventlog

import (
	"context"
	"github.com/SteelMorgan/1c-log-checker/internal/domain"
	"github.com/SteelMorgan/1c-log-checker/internal/offset"
	"os"
	"path/filepath"
	"testing"
)

func TestReaderTailAppendAndNewFile(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "1Cv8.lgf"), []byte("1CV8LOG(ver 2.0)\nbase\n"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "20260922000000.lgp")
	history := "1CV8LOG(ver 2.0)\nbase\n" + testRecord
	if err := os.WriteFile(path, []byte(history), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := offset.NewBoltDBStore(filepath.Join(dir, "offsets.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.InitializeEventLogTail(ctx, dir, map[string]int64{path: int64(len(history))}); err != nil {
		t.Fatal(err)
	}
	r, err := NewReaderWithMetrics(dir, "cluster", "base", "cluster-name", "base-name", nil, 1, nil, s, nil)
	if err != nil {
		t.Fatal(err)
	}
	r.UseSafeCheckpoints = true
	r.recordChan = make(chan *domain.EventLogRecord, 10)
	r.processFile(ctx, path, 0)
	if len(r.recordChan) != 0 {
		t.Fatal("history emitted")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.WriteString(testRecord[:30]); err != nil {
		t.Fatal(err)
	}
	f.Close()
	r.processFile(ctx, path, 0)
	if len(r.recordChan) != 0 {
		t.Fatal("partial record emitted")
	}
	f, err = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.WriteString(testRecord[30:]); err != nil {
		t.Fatal(err)
	}
	f.Close()
	r.processFile(ctx, path, 0)
	if len(r.recordChan) != 1 {
		t.Fatal("append lost")
	}
	<-r.recordChan
	r.processFile(ctx, path, 0)
	if len(r.recordChan) != 0 {
		t.Fatal("record replayed")
	}
	newPath := filepath.Join(dir, "20260923000000.lgp")
	if err = os.WriteFile(newPath, []byte(history), 0600); err != nil {
		t.Fatal(err)
	}
	r.processFile(ctx, newPath, 0)
	if len(r.recordChan) != 1 {
		t.Fatal("new file not read from beginning")
	}
}
