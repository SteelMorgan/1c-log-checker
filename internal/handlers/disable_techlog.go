package handlers

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"os"
)

// DisableTechLogHandler handles disable_techlog MCP tool
type DisableTechLogHandler struct{}

// NewDisableTechLogHandler creates a new handler
func NewDisableTechLogHandler() *DisableTechLogHandler {
	return &DisableTechLogHandler{}
}

// DisableTechLog disables tech log by removing logcfg.xml or emptying it
func (h *DisableTechLogHandler) DisableTechLog(ctx context.Context, configPath string) error {
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil // Already disabled
	}
	
	// Create minimal config that disables logging
	config := LogConfig{
		XMLName: xml.Name{Local: "config", Space: "http://v8.1c.ru/v8/tech-log"},
		Dump:    DumpConfig{Create: false},
		Logs:    []LogElement{}, // No log elements = disabled
	}
	
	// Marshal to XML
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	
	encoder := xml.NewEncoder(&buf)
	encoder.Indent("", "  ")
	if err := encoder.Encode(config); err != nil {
		return fmt.Errorf("failed to encode XML: %w", err)
	}
	
	// Write to file
	if err := os.WriteFile(configPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	
	return nil
}

