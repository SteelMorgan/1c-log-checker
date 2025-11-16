package techlog

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LogCfgConfig represents the parsed logcfg.xml configuration
type LogCfgConfig struct {
	Format string // "text" or "json"
	Location string // Directory path for log files
}

// ReadLogCfgXML reads and parses logcfg.xml file
// Returns the format ("text" or "json") and location from the first <log> element
// If file doesn't exist or can't be parsed, returns error
func ReadLogCfgXML(configPath string) (*LogCfgConfig, error) {
	// Try to find logcfg.xml in common locations if path is empty
	if configPath == "" {
		// Try standard locations
		possiblePaths := []string{
			"C:\\Program Files\\1cv8\\conf\\logcfg.xml",
			"C:\\Program Files (x86)\\1cv8\\conf\\logcfg.xml",
			os.ExpandEnv("$HOME/.1cv8/conf/logcfg.xml"),
		}
		
		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				configPath = path
				break
			}
		}
		
		if configPath == "" {
			return nil, fmt.Errorf("logcfg.xml not found in standard locations")
		}
	}
	
	// Read file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read logcfg.xml: %w", err)
	}
	
	// Parse XML
	var config LogCfgXML
	if err := xml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse logcfg.xml: %w", err)
	}
	
	// Extract format and location from first <log> element
	if len(config.Logs) == 0 {
		return nil, fmt.Errorf("no <log> elements found in logcfg.xml")
	}
	
	firstLog := config.Logs[0]
	format := strings.ToLower(firstLog.Format)
	if format == "" {
		format = "text" // Default format
	}
	
	return &LogCfgConfig{
		Format:   format,
		Location: firstLog.Location,
	}, nil
}

// DetectFormatFromFile detects format by reading the first line of a log file
// Returns "json" if first line is valid JSON, "text" otherwise
func DetectFormatFromFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	
	// Read first line
	var firstLine string
	buf := make([]byte, 1024)
	n, err := file.Read(buf)
	if err != nil && err.Error() != "EOF" {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	
	if n > 0 {
		// Find first newline
		lines := strings.SplitN(string(buf[:n]), "\n", 2)
		firstLine = strings.TrimSpace(lines[0])
	}
	
	if firstLine == "" {
		return "text", nil // Default to text if empty
	}

	// Remove BOM (Byte Order Mark) if present
	// UTF-8 BOM is "\ufeff" (EF BB BF in hex)
	firstLine = strings.TrimPrefix(firstLine, "\ufeff")
	firstLine = strings.TrimPrefix(firstLine, "\uFEFF")

	// Check if it's JSON (starts with {)
	if strings.HasPrefix(strings.TrimSpace(firstLine), "{") {
		return "json", nil
	}

	// Otherwise it's text format
	return "text", nil
}

// DetectFormatFromDirectory detects format by checking files in directory (recursively)
// Tries to find logcfg.xml first, then falls back to checking first log file
// configDir is the directory where logcfg.xml should be located (e.g., from TECHLOG_CONFIG_DIR)
func DetectFormatFromDirectory(dirPath string, configDir string) (string, error) {
	// Try to read from logcfg.xml if config directory is provided
	if configDir != "" {
		// Construct path to logcfg.xml
		configPath := filepath.Join(configDir, "logcfg.xml")
		if cfg, err := ReadLogCfgXML(configPath); err == nil {
			return cfg.Format, nil
		}
		// If file doesn't exist in configDir, try standard locations
		if cfg, err := ReadLogCfgXML(""); err == nil {
			return cfg.Format, nil
		}
	}

	// Fallback: detect from first log file in directory (search recursively)
	var firstLogFile string
	err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Continue on error
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Only process .log files
		if !strings.HasSuffix(path, ".log") {
			return nil
		}

		// Skip .zip files
		if strings.HasSuffix(path, ".zip") {
			return nil
		}

		// Found first log file
		firstLogFile = path
		return filepath.SkipAll // Stop walking after finding first file
	})

	if err != nil && err != filepath.SkipAll {
		return "text", fmt.Errorf("failed to walk directory: %w", err)
	}

	if firstLogFile == "" {
		// No log files yet, default to text
		return "text", nil
	}

	// Check first log file
	format, err := DetectFormatFromFile(firstLogFile)
	if err != nil {
		return "text", fmt.Errorf("failed to detect format from file: %w", err)
	}

	return format, nil
}

// LogCfgXML represents the XML structure of logcfg.xml
type LogCfgXML struct {
	XMLName xml.Name `xml:"config"`
	Logs    []LogElement `xml:"log"`
}

// LogElement represents a <log> element in logcfg.xml
type LogElement struct {
	Location string `xml:"location,attr"`
	Format   string `xml:"format,attr"`
	History  string `xml:"history,attr"`
}

