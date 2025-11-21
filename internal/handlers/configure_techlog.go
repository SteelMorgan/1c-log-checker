package handlers

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/SteelMorgan/1c-log-checker/internal/techlog"
)

// ConfigureTechLogHandler handles configure_techlog MCP tool
type ConfigureTechLogHandler struct {
	configDir   string   // Directory for logcfg.xml files
	techLogDirs []string // Base directories for tech logs (from TECHLOG_DIRS)
}

// NewConfigureTechLogHandler creates a new handler
func NewConfigureTechLogHandler(configDir string, techLogDirs []string) *ConfigureTechLogHandler {
	return &ConfigureTechLogHandler{
		configDir:   configDir,
		techLogDirs: techLogDirs,
	}
}

// ConfigureTechLog generates logcfg.xml configuration and optionally saves it to file
func (h *ConfigureTechLogHandler) ConfigureTechLog(ctx context.Context, params ConfigureTechLogParams) (string, error) {
	// Validate path structure if cluster_guid and infobase_guid are provided
	if params.ClusterGUID != "" && params.InfobaseGUID != "" {
		// Normalize backslashes to forward slashes for cross-platform compatibility
		normalizedLocation := strings.ReplaceAll(params.Location, "\\", "/")
		if err := techlog.ValidateTechLogPath(normalizedLocation, params.ClusterGUID, params.InfobaseGUID, h.techLogDirs); err != nil {
			return "", fmt.Errorf("invalid techlog location path: %w", err)
		}
		// Use normalized path for config
		params.Location = normalizedLocation
	}
	
	config := LogConfig{
		XMLName: xml.Name{Local: "config", Space: "http://v8.1c.ru/v8/tech-log"},
		Dump:    DumpConfig{Create: false},
		Logs:    []LogElement{},
	}
	
	// Create main log element
	logElem := LogElement{
		Location: params.Location,
		History:  params.History,
		Format:   params.Format,
		Events:   []EventFilter{},
	}
	
	// Add event filters
	for _, eventName := range params.Events {
		logElem.Events = append(logElem.Events, EventFilter{
			Eq: PropertyCondition{
				Property: "name",
				Value:    eventName,
			},
		})
	}
	
	// Add property filters
	for _, propName := range params.Properties {
		logElem.Properties = append(logElem.Properties, PropertyElement{
			Name: propName,
		})
	}
	
	// If no specific properties, use "all"
	if len(logElem.Properties) == 0 {
		logElem.Properties = append(logElem.Properties, PropertyElement{
			Name: "all",
		})
	}
	
	config.Logs = append(config.Logs, logElem)
	
	// Marshal to XML
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	
	encoder := xml.NewEncoder(&buf)
	encoder.Indent("", "  ")
	if err := encoder.Encode(config); err != nil {
		return "", fmt.Errorf("failed to encode XML: %w", err)
	}
	
	xmlContent := buf.String()
	
	// Determine config file path
	configPath := params.ConfigPath
	if configPath == "" {
		// Use default path from config directory
		configPath = filepath.Join(h.configDir, "logcfg.xml")
	}
	
	// Create directory if it doesn't exist
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}
	
	// Write file
	if err := os.WriteFile(configPath, []byte(xmlContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write config file: %w", err)
	}
	
	return xmlContent, nil
}

// ConfigureTechLogParams defines parameters for configure_techlog tool
type ConfigureTechLogParams struct {
	Location      string   // Directory for tech log files (MUST include cluster_guid/infobase_guid if provided)
	ClusterGUID   string   // Cluster GUID (optional, but required for path validation)
	InfobaseGUID  string   // Infobase GUID (optional, but required for path validation)
	ConfigPath    string   // Path to save logcfg.xml file (optional, if empty only returns XML)
	History       int      // Hours to keep logs
	Format        string   // text or json
	Events        []string // Event names (EXCP, CONN, DBMSSQL, etc.)
	Properties    []string // Property names (all, sql, etc.)
}

// XML structure for logcfg.xml
type LogConfig struct {
	XMLName xml.Name      `xml:"config"`
	XMLNS   string        `xml:"xmlns,attr"`
	Dump    DumpConfig    `xml:"dump"`
	Logs    []LogElement  `xml:"log"`
}

type DumpConfig struct {
	Create bool `xml:"create,attr"`
}

type LogElement struct {
	Location   string            `xml:"location,attr"`
	History    int               `xml:"history,attr"`
	Format     string            `xml:"format,attr,omitempty"`
	Events     []EventFilter     `xml:"event"`
	Properties []PropertyElement `xml:"property"`
}

type EventFilter struct {
	Eq PropertyCondition `xml:"eq"`
}

type PropertyCondition struct {
	Property string `xml:"property,attr"`
	Value    string `xml:"value,attr"`
}

type PropertyElement struct {
	Name string `xml:"name,attr"`
}

