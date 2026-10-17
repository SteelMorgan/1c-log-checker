package eventlog

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SteelMorgan/1c-log-checker/internal/domain"
	"github.com/SteelMorgan/1c-log-checker/internal/offset"
)

const streamHeader = "1CV8LOG(ver 2.0)\nbase\n"
const streamRecord = "{20261017000000,N,{0,0},0,0,0,0,0,I,\"test\",0,0,\"\",0,0,0,0}"

func streamFixture(t *testing.T, text string) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "events.lgp")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestStreamCheckpointBeforeIncompleteRecord(t *testing.T) {
	complete := streamHeader + streamRecord + ",\n"
	f := streamFixture(t, complete+streamRecord[:35])
	p := NewLgpParser("cluster", "base", "", "", nil)
	out := make(chan *domain.EventLogRecord, 10)
	var checkpoint int64
	callback := func(next, count int64, ts time.Time) error { checkpoint = next; return nil }
	if err := p.ParseStream(context.Background(), f, out, callback, 0); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || checkpoint != int64(len(complete)) {
		t.Fatalf("records=%d checkpoint=%d want=%d", len(out), checkpoint, len(complete))
	}
	if err := os.WriteFile(f.Name(), []byte(complete+streamRecord+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(checkpoint, 0); err != nil {
		t.Fatal(err)
	}
	if err := p.ParseStream(context.Background(), f, out, callback, checkpoint); err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("completed append lost: records=%d", len(out))
	}
}

func TestStreamRecordBound(t *testing.T) {
	for _, payload := range []string{strings.Repeat("x", 17<<20), strings.Repeat("x\n", 9<<20)} {
		f := streamFixture(t, streamHeader+"{20261017000000,N,{0,0},0,0,0,0,0,I,\""+payload)
		p := NewLgpParser("cluster", "base", "", "", nil)
		if err := p.ParseStream(context.Background(), f, make(chan *domain.EventLogRecord, 1), nil, 0); err == nil {
			t.Fatal("oversized unfinished record accepted")
		}
	}
}

func TestStreamPreservesMultilineAndUTF8(t *testing.T) {
	record := strings.Replace(streamRecord, "test", "строка 1\n  строка 2 { } , \"\"цитата\"\"", 1)
	f := streamFixture(t, streamHeader+record+",\n"+streamRecord)
	out := make(chan *domain.EventLogRecord, 2)
	if err := NewLgpParser("", "", "", "", nil).ParseStream(context.Background(), f, out, nil, 0); err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("records=%d", len(out))
	}
	if got := (<-out).Comment; !strings.Contains(got, "\n  строка 2") {
		t.Fatalf("multiline changed: %q", got)
	}
}

func TestStreamBackslashDoesNotEscapeLGPQuote(t *testing.T) {
	// В строковом формате 1С кавычки удваиваются, обратный слеш — обычный символ.
	record := strings.Replace(streamRecord, "test", `C:\logs\`, 1)
	f := streamFixture(t, streamHeader+record+",\n"+streamRecord)
	out := make(chan *domain.EventLogRecord, 2)
	if err := NewLgpParser("", "", "", "", nil).ParseStream(context.Background(), f, out, nil, 0); err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("quote swallowed after backslash: records=%d", len(out))
	}
	if got := (<-out).Comment; got != `C:\logs\` {
		t.Fatalf("comment=%q", got)
	}
}

func TestStreamLargeValidRecordCrossesChunks(t *testing.T) {
	text := strings.Repeat("ю", 100000)
	record := strings.Replace(streamRecord, "test", text, 1)
	f := streamFixture(t, streamHeader+record)
	out := make(chan *domain.EventLogRecord, 1)
	if err := NewLgpParser("", "", "", "", nil).ParseStream(context.Background(), f, out, nil, 0); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatal("large valid record skipped")
	}
	if got := (<-out).Comment; got != text {
		t.Fatal("chunk split changed UTF-8")
	}
}

func TestReaderKeepsIncompleteCheckpointAndResetsOnTruncation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "1Cv8.lgf"), []byte(streamHeader), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "events.lgp")
	full := streamHeader + streamRecord + "\n"
	if err := os.WriteFile(path, []byte(full+streamRecord[:35]), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := offset.NewBoltDBStore(filepath.Join(dir, "offsets.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := NewReaderWithMetrics(dir, "", "", "", "", nil, 1, nil, s, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Для прямого вызова processFile тест сам исполняет роль потребителя.
	r.recordChan = make(chan *domain.EventLogRecord, 10)
	r.processFile(context.Background(), path, 0)
	off, _ := s.GetEventLogOffset(context.Background(), path)
	if off == nil || off.OffsetBytes != int64(len(full)) {
		t.Fatalf("unsafe offset: %+v", off)
	}
	if err := os.WriteFile(path, []byte(full+streamRecord+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r.processFile(context.Background(), path, 0)
	if len(r.recordChan) != 2 {
		t.Fatal("append lost")
	}
	if err := os.WriteFile(path, []byte(full), 0600); err != nil {
		t.Fatal(err)
	}
	r.processFile(context.Background(), path, 0)
	off, _ = s.GetEventLogOffset(context.Background(), path)
	if off.OffsetBytes != int64(len(full)) || len(r.recordChan) != 3 {
		t.Fatalf("truncation: %+v records=%d", off, len(r.recordChan))
	}
}

func TestStreamRejectsExcessiveNesting(t *testing.T) {
	f := streamFixture(t, streamHeader+strings.Repeat("{", maxLGPDepth+1)+strings.Repeat("}", maxLGPDepth+1))
	if err := NewLgpParser("", "", "", "", nil).ParseStream(context.Background(), f, make(chan *domain.EventLogRecord, 1), nil, 0); err == nil {
		t.Fatal("unbounded nesting accepted")
	}
}

func TestNestedDataDecodeHasCumulativeBudget(t *testing.T) {
	text := `{S,"` + strings.Repeat("x", 1<<20) + `"}`
	for i := 0; i < 40; i++ {
		text = "{P,{0," + text + "}}"
	}
	if _, err := parseDataField(text); err == nil {
		t.Fatal("cumulative decode budget not enforced")
	}
	if got, err := parseDataField(`{P,{0,{S,"value"}}}`); err != nil || !strings.Contains(got, "value") {
		t.Fatalf("normal nested data changed: %q %v", got, err)
	}
}
