package mcp

import (
	"context"
	"fmt"

	"github.com/1c-log-checker/internal/config"
	"github.com/rs/zerolog/log"
)

// Server implements MCP protocol server
type Server struct {
	cfg *config.Config
}

// NewServer creates a new MCP server
func NewServer(cfg *config.Config) (*Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	return &Server{
		cfg: cfg,
	}, nil
}

// Start starts the MCP server
func (s *Server) Start(ctx context.Context) error {
	log.Info().
		Int("port", s.cfg.MCPPort).
		Msg("MCP server starting...")
	
	// TODO: Implement MCP protocol
	// - HTTP server for tools
	// - stdio transport support
	// - Tool handlers
	
	// Wait for context cancellation
	<-ctx.Done()
	return ctx.Err()
}

// Stop stops the MCP server gracefully
func (s *Server) Stop() error {
	log.Info().Msg("MCP server stopping...")
	
	// TODO: Implement graceful shutdown
	
	return nil
}

