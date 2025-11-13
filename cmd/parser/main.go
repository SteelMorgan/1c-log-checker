package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/1c-log-checker/internal/config"
	"github.com/1c-log-checker/internal/observability"
	"github.com/1c-log-checker/internal/service"
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
		Msg("Starting 1C Log Parser")

	// Initialize tracer (if enabled)
	if cfg.TracingEnabled {
		shutdown, err := observability.InitTracer("1c-log-parser")
		if err != nil {
			log.Error().Err(err).Msg("Failed to initialize tracer")
		} else {
			defer shutdown(context.Background())
		}
	}

	// Create parser service
	parserSvc, err := service.NewParserService(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create parser service")
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start parser service
	errChan := make(chan error, 1)
	go func() {
		if err := parserSvc.Start(ctx); err != nil {
			errChan <- err
		}
	}()

	log.Info().Msg("Parser service started successfully")

	// Wait for shutdown signal or error
	select {
	case <-sigChan:
		log.Info().Msg("Received shutdown signal")
	case err := <-errChan:
		log.Error().Err(err).Msg("Parser service error")
	}

	// Graceful shutdown
	log.Info().Msg("Shutting down gracefully...")
	cancel()

	if err := parserSvc.Stop(); err != nil {
		log.Error().Err(err).Msg("Error during shutdown")
	}

	log.Info().Msg("Parser service stopped")
}

