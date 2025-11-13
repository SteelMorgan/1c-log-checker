package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/1c-log-checker/internal/config"
	"github.com/1c-log-checker/internal/mcp"
	"github.com/1c-log-checker/internal/observability"
	"github.com/rs/zerolog/log"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	observability.InitLogger(cfg.LogLevel)

	log.Info().
		Str("version", "0.1.0").
		Msg("Starting 1C Log MCP Server")

	// Create MCP server
	mcpServer, err := mcp.NewServer(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create MCP server")
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start MCP server
	errChan := make(chan error, 1)
	go func() {
		if err := mcpServer.Start(ctx); err != nil {
			errChan <- err
		}
	}()

	log.Info().
		Int("port", cfg.MCPPort).
		Msg("MCP server started successfully")

	// Wait for shutdown signal or error
	select {
	case <-sigChan:
		log.Info().Msg("Received shutdown signal")
	case err := <-errChan:
		log.Error().Err(err).Msg("MCP server error")
	}

	// Graceful shutdown
	log.Info().Msg("Shutting down gracefully...")
	cancel()

	if err := mcpServer.Stop(); err != nil {
		log.Error().Err(err).Msg("Error during shutdown")
	}

	log.Info().Msg("MCP server stopped")
}

