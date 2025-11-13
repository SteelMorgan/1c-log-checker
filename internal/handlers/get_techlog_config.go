package handlers

import (
	"context"
	"fmt"
	"os"
)

// GetTechLogConfigHandler handles get_techlog_config MCP tool
type GetTechLogConfigHandler struct{}

// NewGetTechLogConfigHandler creates a new handler
func NewGetTechLogConfigHandler() *GetTechLogConfigHandler {
	return &GetTechLogConfigHandler{}
}

// GetTechLogConfig reads current logcfg.xml configuration
func (h *GetTechLogConfigHandler) GetTechLogConfig(ctx context.Context, configPath string) (string, error) {
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return "", fmt.Errorf("config file not found: %s", configPath)
	}
	
	// Read file content
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config: %w", err)
	}
	
	return string(data), nil
}

