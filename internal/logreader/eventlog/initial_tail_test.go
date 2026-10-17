package eventlog

import (
	"context"
	"github.com/SteelMorgan/1c-log-checker/internal/logreader"
	"github.com/SteelMorgan/1c-log-checker/internal/offset"
	"os"
	"path/filepath"
	"testing"
)

func TestInitializeTailRootsOnlySelectedSource(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	root := filepath.Join(dir, "prod")
	base := filepath.Join(root, "reg_1541", "base", "1Cv8Log")
	if err := os.MkdirAll(base, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(base, "old.lgp")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(2 << 30); err != nil {
		t.Fatal(err)
	}
	f.Close()
	s, err := offset.NewBoltDBStore(filepath.Join(dir, "offsets.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	locations := []logreader.LogLocation{{BasePath: base, ClusterGUID: "valid-guid", LgpFiles: []string{path}}, {BasePath: filepath.Join(dir, "dev"), LgpFiles: []string{"/missing-dev-file"}}}
	if err = InitializeTailRoots(ctx, s, []string{root}, locations); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetEventLogOffset(ctx, path)
	if err != nil || got.OffsetBytes != 2<<30 {
		t.Fatal("wrong snapshot")
	}
	if err = InitializeTailRoots(ctx, s, []string{root}, nil); err != nil {
		t.Fatal("initialized root should not resnapshot")
	}
	if err = InitializeTailRoots(ctx, s, []string{filepath.Join(dir, "missing")}, nil); err == nil {
		t.Fatal("missing root silently skipped")
	}
}
