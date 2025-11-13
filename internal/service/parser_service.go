package service

import (
	"context"
	"fmt"

	"github.com/1c-log-checker/internal/config"
	"github.com/rs/zerolog/log"
)

// ParserService orchestrates log parsing workers
type ParserService struct {
	cfg *config.Config
}

// NewParserService creates a new parser service
func NewParserService(cfg *config.Config) (*ParserService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	return &ParserService{
		cfg: cfg,
	}, nil
}

// Start starts the parser service
func (s *ParserService) Start(ctx context.Context) error {
	log.Info().Msg("Parser service starting...")
	
	// TODO: Implement workers for event log and tech log parsing
	
	// Wait for context cancellation
	<-ctx.Done()
	return ctx.Err()
}

// Stop stops the parser service gracefully
func (s *ParserService) Stop() error {
	log.Info().Msg("Parser service stopping...")
	
	// TODO: Implement graceful shutdown
	// - Stop workers
	// - Flush pending batches
	// - Save offsets
	
	return nil
}

