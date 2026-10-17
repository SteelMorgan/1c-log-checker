package eventlog

import (
	"context"
	"github.com/SteelMorgan/1c-log-checker/internal/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testRecord = "{20260922000000,N,{0,0},0,0,0,0,0,I,\"test\",0,0,\"\",0,0,0,0}\n"

func incrementalFixture(t *testing.T, text string, start int64, pending bool) (int64, bool, int) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sample.lgp")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := make(chan *domain.EventLogRecord, 100)
	p := NewLgpParser("cluster", "base", "cluster-name", "base-name", nil)
	next, still, err := p.ParseIncremental(context.Background(), f, out, start, pending)
	if err != nil {
		t.Fatal(err)
	}
	return next, still, len(out)
}

func TestInitialTailSkipsHistoryAndPartialPrefix(t *testing.T) {
	history := "1CV8LOG(ver 2.0)\nbase\n" + testRecord
	next, pending, count := incrementalFixture(t, history, int64(len(history)), true)
	if next != int64(len(history)) || !pending || count != 0 {
		t.Fatal("EOF changed")
	}
	next, pending, count = incrementalFixture(t, history+testRecord, int64(len(history)), true)
	if next != int64(len(history+testRecord)) || pending || count != 1 {
		t.Fatal("new record not read")
	}
	text := history + "old unfinished comment }\"}\n" + testRecord
	_, pending, count = incrementalFixture(t, text, int64(len(history)), true)
	if pending || count != 1 {
		t.Fatal("partial prefix not discarded")
	}
}

func TestIncrementalIncompleteRecordCheckpoint(t *testing.T) {
	header := "1CV8LOG(ver 2.0)\nbase\n"
	full := header + testRecord
	next, pending, count := incrementalFixture(t, full+testRecord[:30], 0, false)
	if next != int64(len(full)) || pending || count != 1 {
		t.Fatalf("checkpoint at prefetch: %d", next)
	}
	_, _, count = incrementalFixture(t, full+testRecord, next, false)
	if count != 1 {
		t.Fatal("append lost")
	}
	multiline := strings.Replace(testRecord, "test", "line1\nline2 { }", 1)
	_, _, count = incrementalFixture(t, header+multiline, 0, false)
	if count != 1 {
		t.Fatal("multiline record lost")
	}
}

func TestNewFileStartsAtZero(t *testing.T) {
	header := "1CV8LOG(ver 2.0)\nbase\n"
	_, pending, count := incrementalFixture(t, header+testRecord, 0, false)
	if pending || count != 1 {
		t.Fatal("new file skipped")
	}
}
