package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all configuration for the application
type Config struct {
	// ClickHouse configuration
	ClickHouseHost string
	ClickHousePort int
	ClickHouseDB   string

	// Log directories
	LogDirs     []string // Paths to event log directories (журнал регистрации)
	TechLogDirs []string // Paths to tech log directories (технологический журнал)

	// Parser settings
	LogRetentionDays    int  // TTL in days
	ReadOnly            bool // Technical mode: read logs but don't write to ClickHouse
	OffsetMirror        bool // Mirror offset storage to ClickHouse
	EnableDeduplication bool // Enable deduplication check (slower but prevents duplicates)
	MaxWorkers          int  // Max parallel workers for file processing (default: 4)
	BatchSize           int  // Batch size for ClickHouse inserts (default: 5000)
	BatchFlushTimeout   int  // Batch flush timeout in milliseconds (default: 1000)

	// MCP settings
	MCPPort       int
	ClusterMapPath string // Path to cluster_map.yaml (used only by MCP server, not parser)
	TechLogConfigDir string // Directory for logcfg.xml file (mounted from host)

	// GUID Enrichment (GUID → Presentation)
	// MCP-server initiated: collects unknown GUIDs and requests 1C for presentations
	EnrichmentEnabled  bool   // Enable enrichment worker
	EnrichmentEndpoint string // 1C HTTP endpoint (e.g., "http://1c-server:8080")
	EnrichmentAPIKey   string // API key for authentication
	EnrichmentBatchSize int   // Max GUIDs per request (default: 500)
	EnrichmentInterval int    // Worker interval in minutes (default: 10)

	// Observability
	LogLevel       string
	TracingEnabled bool

	// Internal (computed)
	IsInDocker bool // Auto-detected based on CLICKHOUSE_HOST
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		ClickHouseHost: getEnv("CLICKHOUSE_HOST", "localhost"),
		ClickHousePort: getEnvInt("CLICKHOUSE_PORT", 9000),
		ClickHouseDB:   getEnv("CLICKHOUSE_DB", "logs"),

		LogDirs:     parsePathList(getEnv("LOG_DIRS", "")),
		TechLogDirs: parsePathList(getEnv("TECHLOG_DIRS", "")),

		LogRetentionDays:    getEnvInt("LOG_RETENTION_DAYS", 30),
		ReadOnly:            getEnvBool("READ_ONLY", false),
		OffsetMirror:        getEnvBool("OFFSET_MIRROR", false),
		EnableDeduplication: getEnvBool("ENABLE_DEDUPLICATION", false),
		MaxWorkers:          getEnvInt("MAX_WORKERS", 4),
		BatchSize:           getEnvInt("BATCH_SIZE", 5000),
		BatchFlushTimeout:   getEnvInt("BATCH_FLUSH_TIMEOUT", 1000),

		MCPPort:       getEnvInt("MCP_PORT", 8080),
		ClusterMapPath: getEnv("CLUSTER_MAP_PATH", "configs/cluster_map.yaml"),
		TechLogConfigDir: getEnv("TECHLOG_CONFIG_DIR", "/app/configs/techlog"), // Default inside container

		EnrichmentEnabled:  getEnvBool("ENRICHMENT_ENABLED", false),
		EnrichmentEndpoint: getEnv("ENRICHMENT_ENDPOINT", ""),
		EnrichmentAPIKey:   getEnv("ENRICHMENT_API_KEY", ""),
		EnrichmentBatchSize: getEnvInt("ENRICHMENT_BATCH_SIZE", 500),
		EnrichmentInterval: getEnvInt("ENRICHMENT_INTERVAL", 10), // minutes

		LogLevel:       getEnv("LOG_LEVEL", "info"),
		TracingEnabled: getEnvBool("TRACING_ENABLED", false),

		IsInDocker: getEnv("CLICKHOUSE_HOST", "localhost") != "localhost",
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.ClickHouseHost == "" {
		return fmt.Errorf("CLICKHOUSE_HOST is required")
	}
	if c.ClickHousePort <= 0 || c.ClickHousePort > 65535 {
		return fmt.Errorf("CLICKHOUSE_PORT must be between 1 and 65535")
	}
	if c.ClickHouseDB == "" {
		return fmt.Errorf("CLICKHOUSE_DB is required")
	}
	// Note: LOG_DIRS and TECHLOG_DIRS are optional
	// They are required only for parser service, not for MCP server
	if c.LogRetentionDays < 1 {
		return fmt.Errorf("LOG_RETENTION_DAYS must be at least 1")
	}
	if c.MCPPort <= 0 || c.MCPPort > 65535 {
		return fmt.Errorf("MCP_PORT must be between 1 and 65535")
	}

	return nil
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets an integer environment variable or returns a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvBool gets a boolean environment variable or returns a default value
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// parsePathList parses a semicolon-separated list of paths
func parsePathList(pathsStr string) []string {
	if pathsStr == "" {
		return nil
	}

	paths := strings.Split(pathsStr, ";")
	result := make([]string, 0, len(paths))

	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

