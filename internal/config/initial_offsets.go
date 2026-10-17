package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (c *Config) validateInitialOffsets() error {
	switch c.InitialOffsetMode {
	case "", "beginning":
		if len(c.InitialOffsetRoots) > 0 {
			return fmt.Errorf("INITIAL_OFFSET_ROOTS requires INITIAL_OFFSET_MODE=tail")
		}
	case "tail":
		if len(c.InitialOffsetRoots) == 0 {
			return fmt.Errorf("tail requires explicit INITIAL_OFFSET_ROOTS")
		}
		for _, root := range c.InitialOffsetRoots {
			if !filepath.IsAbs(root) || filepath.Clean(root) != root || root == "/" {
				return fmt.Errorf("invalid INITIAL_OFFSET_ROOTS path")
			}
			found := false
			for _, configured := range c.LogDirs {
				if root == configured {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("INITIAL_OFFSET_ROOTS must match a LOG_DIRS root")
			}
		}
	default:
		return fmt.Errorf("INITIAL_OFFSET_MODE must be beginning or tail")
	}
	return nil
}

func (c *Config) TailForPath(path string) bool {
	if c.InitialOffsetMode != "tail" {
		return false
	}
	for _, root := range c.InitialOffsetRoots {
		if path == root || strings.HasPrefix(path, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
