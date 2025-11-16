package handlers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// RestoreTechLogHandler handles restore_techlog MCP tool
type RestoreTechLogHandler struct {
	configDir string // Directory for logcfg.xml files
}

// NewRestoreTechLogHandler creates a new handler
func NewRestoreTechLogHandler(configDir string) *RestoreTechLogHandler {
	return &RestoreTechLogHandler{
		configDir: configDir,
	}
}

// RestoreTechLog restores logcfg.xml from logcfg.xml.OLD
func (h *RestoreTechLogHandler) RestoreTechLog(ctx context.Context, configPath string) error {
	// Use default path if not provided
	if configPath == "" {
		configPath = filepath.Join(h.configDir, "logcfg.xml")
	}

	// Build backup path
	backupPath := configPath + ".OLD"

	// Check if backup file exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s (nothing to restore)", backupPath)
	}

	// Remove current config file if exists
	if _, err := os.Stat(configPath); err == nil {
		if err := os.Remove(configPath); err != nil {
			return fmt.Errorf("failed to remove current config file: %w", err)
		}
	}

	// Rename backup file to config file
	if err := os.Rename(backupPath, configPath); err != nil {
		return fmt.Errorf("failed to rename backup file: %w", err)
	}

	return nil
}

