package offset

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
)

func TestEventLogConcurrentOffsetsNeverRegress(t *testing.T) {
	s, err := NewBoltDBStore(filepath.Join(t.TempDir(), "offsets.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.SaveEventLogOffset(ctx, &EventLogOffset{FilePath: "events", OffsetBytes: 1000, RecordsParsed: 100}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := s.SaveEventLogOffset(ctx, &EventLogOffset{FilePath: "events", OffsetBytes: int64(i), RecordsParsed: int64(i)}); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	off, err := s.GetEventLogOffset(ctx, "events")
	if err != nil || off.OffsetBytes != 1000 || off.RecordsParsed != 100 {
		t.Fatalf("regression: %+v %v", off, err)
	}
}

func TestEventLogRotationAllowsResetButRejectsStaleGeneration(t *testing.T) {
	s, err := NewBoltDBStore(filepath.Join(t.TempDir(), "offsets.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	for _, off := range []*EventLogOffset{
		{FilePath: "events", OffsetBytes: 1000, RecordsParsed: 100},
		{FilePath: "events", OffsetBytes: 50, RecordsParsed: 1, Generation: 1, CheckpointHash: "hash", FileSize: 50},
		{FilePath: "events", OffsetBytes: 2000, RecordsParsed: 200},
	} {
		if err := s.SaveEventLogOffset(ctx, off); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.GetEventLogOffset(ctx, "events")
	if err != nil || got.Generation != 1 || got.OffsetBytes != 50 || got.CheckpointHash != "hash" {
		t.Fatalf("stale generation accepted: %+v %v", got, err)
	}
}
