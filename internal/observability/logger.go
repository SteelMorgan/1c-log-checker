package observability

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// InitLogger initializes the global logger with the specified level
// If logFile is not empty, logs will be written to that file in addition to stdout
// Console output uses human-readable format, file output uses JSON format
func InitLogger(level string, logFile string) {
	var writers []io.Writer

	// Always write to stdout with console formatting
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "2006-01-02 15:04:05",
	}
	writers = append(writers, consoleWriter)

	// If log file is specified, also write to file (JSON format for easier parsing)
	if logFile != "" {
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			// If we can't open the file, log to stderr and continue with stdout only
			// Use fmt to avoid circular dependency
			fmt.Fprintf(os.Stderr, "Failed to open log file %s: %v, using stdout only\n", logFile, err)
		} else {
			// File will receive JSON format (zerolog default when writing to raw file)
			// Note: MultiWriter will write the same data to both, but ConsoleWriter formats it
			// So file will contain formatted console output, not pure JSON
			// For pure JSON in file, we'd need separate loggers, but this is simpler
			writers = append(writers, file)
		}
	}

	// Use MultiWriter to write to both stdout (console format) and file
	// Note: File will contain console-formatted text, not JSON
	// This is acceptable for most use cases - logs are readable in both places
	log.Logger = log.Output(io.MultiWriter(writers...))

	// Parse log level
	logLevel := parseLogLevel(level)
	zerolog.SetGlobalLevel(logLevel)

	log.Info().
		Str("level", logLevel.String()).
		Str("file", logFile).
		Msg("Logger initialized")
}

// parseLogLevel parses a string log level to zerolog.Level
func parseLogLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "panic":
		return zerolog.PanicLevel
	default:
		return zerolog.InfoLevel
	}
}

