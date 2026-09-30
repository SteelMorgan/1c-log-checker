package techlog

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SteelMorgan/1c-log-checker/internal/domain"
	"github.com/SteelMorgan/1c-log-checker/internal/offset"
)

func TestTailerProcessNewRecordsDoesNotWriteProgressWhileIdle(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	clusterGUID := "0de031da-e8d9-43de-bb39-7c8bd4d9855c"
	infobaseGUID := "320f6387-89b5-43fc-b344-67b11f957472"
	logDir := filepath.Join(tmpDir, clusterGUID, infobaseGUID, "rphost_1")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("failed to create log dir: %v", err)
	}

	logPath := filepath.Join(logDir, "26062410.log")
	line := "00:00.000001-1,SCALL,0,level=INFO,process=1cv8,OSThread=1,ClientID=1\n"
	if err := os.WriteFile(logPath, []byte(line), 0o644); err != nil {
		t.Fatalf("failed to write tech log file: %v", err)
	}

	store, err := offset.NewBoltDBStore(filepath.Join(tmpDir, "offsets.db"))
	if err != nil {
		t.Fatalf("failed to create offset store: %v", err)
	}
	defer store.Close()

	progressCalls := 0
	tailer := NewTailer(
		tmpDir,
		false,
		store,
		1,
		func(progress *domain.FileReadingProgress) error {
			progressCalls++
			return nil
		},
		nil,
		clusterGUID,
		infobaseGUID,
		"cluster",
		"infobase",
		nil,
		nil,
	)

	var handledRecords int
	handler := func(record *domain.TechLogRecord) error {
		handledRecords++
		return nil
	}

	if err := tailer.processNewRecords(ctx, handler); err != nil {
		t.Fatalf("first processNewRecords failed: %v", err)
	}
	if handledRecords != 1 {
		t.Fatalf("expected 1 handled record after first pass, got %d", handledRecords)
	}
	if progressCalls != 1 {
		t.Fatalf("expected one progress write after first pass, got %d", progressCalls)
	}

	if err := tailer.processNewRecords(ctx, handler); err != nil {
		t.Fatalf("idle processNewRecords failed: %v", err)
	}
	if handledRecords != 1 {
		t.Fatalf("idle pass should not parse more records, got %d handled records", handledRecords)
	}
	if progressCalls != 1 {
		t.Fatalf("idle pass should not write progress again, got %d calls", progressCalls)
	}

	stored, err := store.GetTechLogOffset(ctx, logPath)
	if err != nil {
		t.Fatalf("failed to read stored offset: %v", err)
	}
	if stored == nil {
		t.Fatal("expected stored techlog offset")
	}
	if stored.OffsetBytes == 0 || stored.LastLine != 1 {
		t.Fatalf("unexpected stored offset: offset=%d line=%d", stored.OffsetBytes, stored.LastLine)
	}
	if time.Since(stored.LastTimestamp) > 24*time.Hour {
		t.Fatalf("unexpected stale timestamp: %s", stored.LastTimestamp)
	}
}
