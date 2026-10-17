package offset

import (
	"context"
	"path/filepath"
	"testing"
)

func TestInitialTailAtomicAndPersistent(t *testing.T) {
	ctx := context.Background()
	db := filepath.Join(t.TempDir(), "offsets.db")
	s, err := NewBoltDBStore(db)
	if err != nil {
		t.Fatal(err)
	}
	root := "/mnt/prod/srvinfo"
	old := root + "/old.lgp"
	existing := root + "/existing.lgp"
	if err = s.SaveEventLogOffset(ctx, &EventLogOffset{FilePath: existing, OffsetBytes: 7}); err != nil {
		t.Fatal(err)
	}
	if err = s.InitializeEventLogTail(ctx, root, map[string]int64{old: 2 << 30, existing: 100}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = NewBoltDBStore(db)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.InitializeEventLogTail(ctx, root, map[string]int64{old: 3 << 30, root + "/new.lgp": 900}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetEventLogOffset(ctx, old)
	if err != nil || got.OffsetBytes != 2<<30 || !got.InitialTail {
		t.Fatalf("snapshot: %+v %v", got, err)
	}
	got, _ = s.GetEventLogOffset(ctx, existing)
	if got.OffsetBytes != 7 || got.InitialTail {
		t.Fatal("existing overwritten")
	}
	got, _ = s.GetEventLogOffset(ctx, root+"/new.lgp")
	if got != nil {
		t.Fatal("new file skipped on restart")
	}
	done, err := s.EventLogTailInitialized(root)
	if err != nil || !done {
		t.Fatal("marker not persisted")
	}
	badRoot := "/bad"
	if err = s.InitializeEventLogTail(ctx, badRoot, map[string]int64{"good": 10, "bad": -1}); err == nil {
		t.Fatal("negative size accepted")
	}
	done, _ = s.EventLogTailInitialized(badRoot)
	if done {
		t.Fatal("failed transaction committed marker")
	}
	got, _ = s.GetEventLogOffset(ctx, "good")
	if got != nil {
		t.Fatal("failed transaction committed offset")
	}
}

func TestInitialTailOffsetBackwardCompatibility(t *testing.T) {
	for _, pending := range []bool{false, true} {
		raw, err := serializeEventLogOffset(&EventLogOffset{FilePath: "file", OffsetBytes: 10, InitialTail: pending})
		if err != nil {
			t.Fatal(err)
		}
		got, err := deserializeEventLogOffset(raw)
		if err != nil || got.InitialTail != pending {
			t.Fatal("roundtrip")
		}
		legacy := raw[:28+len("file")]
		got, err = deserializeEventLogOffset(legacy)
		if err != nil || got.InitialTail {
			t.Fatal("legacy decode")
		}
	}
}
