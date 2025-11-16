package handlers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// SaveTechLogHandler handles save_techlog MCP tool
type SaveTechLogHandler struct {
	configDir string // Directory for logcfg.xml files
}

// NewSaveTechLogHandler creates a new handler
func NewSaveTechLogHandler(configDir string) *SaveTechLogHandler {
	return &SaveTechLogHandler{
		configDir: configDir,
	}
}

// SaveTechLog saves current logcfg.xml as logcfg.xml.OLD
func (h *SaveTechLogHandler) SaveTechLog(ctx context.Context, configPath string) error {
	// Use default path if not provided
	if configPath == "" {
		configPath = filepath.Join(h.configDir, "logcfg.xml")
	}

	// Check if source file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s (nothing to save)", configPath)
	}

	// Build backup path
	backupPath := configPath + ".OLD"

	// Read source file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Write backup file (overwrite if exists)
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	return nil
}

