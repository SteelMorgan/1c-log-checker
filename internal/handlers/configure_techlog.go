package handlers

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
)

// ConfigureTechLogHandler handles configure_techlog MCP tool
type ConfigureTechLogHandler struct{}

// NewConfigureTechLogHandler creates a new handler
func NewConfigureTechLogHandler() *ConfigureTechLogHandler {
	return &ConfigureTechLogHandler{}
}

// ConfigureTechLog generates logcfg.xml configuration
func (h *ConfigureTechLogHandler) ConfigureTechLog(ctx context.Context, params ConfigureTechLogParams) (string, error) {
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
	
	return buf.String(), nil
}

// ConfigureTechLogParams defines parameters for configure_techlog tool
type ConfigureTechLogParams struct {
	Location   string   // Directory for tech log files
	History    int      // Hours to keep logs
	Format     string   // text or json
	Events     []string // Event names (EXCP, CONN, DBMSSQL, etc.)
	Properties []string // Property names (all, sql, etc.)
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

