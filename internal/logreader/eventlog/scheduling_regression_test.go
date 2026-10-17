package eventlog

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SteelMorgan/1c-log-checker/internal/offset"
)

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for !condition() {
		select {
		case <-deadline:
			t.Fatal("condition timed out")
		case <-time.After(time.Millisecond):
		}
	}
}

func TestSchedulerDoesNotRequeueBusyGrowingFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "1Cv8.lgf"), []byte(streamHeader), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "events.lgp")
	if err := os.WriteFile(path, []byte(streamHeader+strings.Repeat(streamRecord+"\n", 20)), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := offset.NewBoltDBStore(filepath.Join(dir, "offset.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := NewReaderWithMetrics(dir, "", "", "", "", nil, 4, nil, s, nil)
	if err != nil {
		t.Fatal(err)
	}
	r.rescanInterval = 5 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.Open(ctx); err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	waitFor(t, func() bool { return r.filesProcessed.Load() == 1 })
	// Потребитель остановлен; первый worker должен сохранять владение файлом
	// на всех повторных сканированиях, несмотря на дописывание.
	for i := 0; i < 5; i++ {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.WriteString(streamRecord + "\n")
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(15 * time.Millisecond)
	}
	if n := r.filesProcessed.Load(); n != 1 {
		t.Fatalf("same file has %d workers", n)
	}
	for i := 0; i < 25; i++ {
		if _, err := r.Read(ctx); err != nil {
			t.Fatal(err)
		}
	}
	waitFor(t, func() bool {
		off, _ := s.GetEventLogOffset(ctx, path)
		st, _ := os.Stat(path)
		return off != nil && off.OffsetBytes == st.Size()
	})
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(streamRecord + "\n")
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Read(ctx); err != nil {
		t.Fatal("append after worker completion lost:", err)
	}
	// При ротации новый путь читается независимо от старого checkpoint.
	if err := os.WriteFile(filepath.Join(dir, "rotated.lgp"), []byte(streamHeader+streamRecord), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Read(ctx); err != nil {
		t.Fatal("rotation lost:", err)
	}
}

func TestSchedulerDiscoversFirstFileAndCancelsBlockedQueue(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "1Cv8.lgf"), []byte(streamHeader), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := NewReader(dir, "", "", "", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	r.rescanInterval = time.Millisecond
	if err := r.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b", "c", "d"} {
		if err := os.WriteFile(filepath.Join(dir, name+".lgp"), []byte(streamHeader+streamRecord), 0600); err != nil {
			t.Fatal(err)
		}
	}
	waitFor(t, func() bool { return r.filesProcessed.Load() > 0 })
	done := make(chan struct{})
	go func() { r.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown blocked")
	}
}
