package eventlog

import (
	"context"
	"fmt"
	"github.com/SteelMorgan/1c-log-checker/internal/logreader"
	"github.com/SteelMorgan/1c-log-checker/internal/offset"
	"os"
	"path/filepath"
	"strings"
)

// Инициализация выполняется до запуска любых workers, без чтения содержимого LGP.
func InitializeTailRoots(ctx context.Context, store *offset.BoltDBStore, roots []string, locations []logreader.LogLocation) error {
	for _, root := range roots {
		done, err := store.EventLogTailInitialized(root)
		if err != nil {
			return err
		}
		if done {
			continue
		}
		sizes := map[string]int64{}
		found := false
		for _, loc := range locations {
			if !strings.HasPrefix(loc.BasePath, root+string(filepath.Separator)) {
				continue
			}
			found = true
			if loc.ClusterGUID == "" || loc.ClusterGUID == "00000000-0000-0000-0000-000000000000" || strings.HasPrefix(loc.ClusterGUID, "reg_") {
				return fmt.Errorf("tail source has unresolved cluster GUID")
			}
			for _, path := range loc.LgpFiles {
				st, err := os.Stat(path)
				if err != nil {
					return fmt.Errorf("tail stat: %w", err)
				}
				if !st.Mode().IsRegular() {
					return fmt.Errorf("tail source is not a regular file")
				}
				sizes[path] = st.Size()
			}
		}
		if !found {
			return fmt.Errorf("tail root has no discovered infobase: %s", root)
		}
		if err := store.InitializeEventLogTail(ctx, root, sizes); err != nil {
			return err
		}
	}
	return nil
}
