package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/1c-log-checker/internal/clickhouse"
	"github.com/1c-log-checker/internal/config"
	"github.com/1c-log-checker/internal/handlers"
	"github.com/1c-log-checker/internal/mapping"
	"github.com/rs/zerolog/log"
)

// Server implements MCP protocol server
type Server struct {
	cfg         *config.Config
	httpServer  *http.Server
	chClient    *clickhouse.Client
	clusterMap  *mapping.ClusterMap
	
	// Handlers
	eventLogHandler      *handlers.EventLogHandler
	techLogHandler       *handlers.TechLogHandler
	newErrorsHandler     *handlers.NewErrorsHandler
	configureTechHandler *handlers.ConfigureTechLogHandler
	disableTechHandler   *handlers.DisableTechLogHandler
	getTechCfgHandler    *handlers.GetTechLogConfigHandler
}

// NewServer creates a new MCP server
func NewServer(cfg *config.Config) (*Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	// Connect to ClickHouse
	chClient, err := clickhouse.NewClient(cfg.ClickHouseHost, cfg.ClickHousePort, cfg.ClickHouseDB)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to clickhouse: %w", err)
	}

	// Load cluster map
	clusterMap, err := mapping.LoadClusterMap(cfg.ClusterMapPath)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load cluster map, using GUIDs")
		clusterMap = &mapping.ClusterMap{
			Clusters:  make(map[string]mapping.ClusterInfo),
			Infobases: make(map[string]mapping.InfobaseInfo),
		}
	}

	// Initialize handlers
	eventLogHandler := handlers.NewEventLogHandler(chClient, clusterMap)
	techLogHandler := handlers.NewTechLogHandler(chClient, clusterMap)
	newErrorsHandler := handlers.NewNewErrorsHandler(chClient, clusterMap)
	configureTechHandler := handlers.NewConfigureTechLogHandler()
	disableTechHandler := handlers.NewDisableTechLogHandler()
	getTechCfgHandler := handlers.NewGetTechLogConfigHandler()

	return &Server{
		cfg:                  cfg,
		chClient:             chClient,
		clusterMap:           clusterMap,
		eventLogHandler:      eventLogHandler,
		techLogHandler:       techLogHandler,
		newErrorsHandler:     newErrorsHandler,
		configureTechHandler: configureTechHandler,
		disableTechHandler:   disableTechHandler,
		getTechCfgHandler:    getTechCfgHandler,
	}, nil
}

// Start starts the MCP server
func (s *Server) Start(ctx context.Context) error {
	log.Info().
		Int("port", s.cfg.MCPPort).
		Msg("MCP server starting...")
	
	// Setup HTTP server with MCP tool endpoints
	mux := http.NewServeMux()
	
	// Register tool endpoints
	mux.HandleFunc("/tools/get_event_log", s.handleGetEventLog)
	mux.HandleFunc("/tools/get_tech_log", s.handleGetTechLog)
	mux.HandleFunc("/tools/get_new_errors", s.handleGetNewErrors)
	mux.HandleFunc("/tools/configure_techlog", s.handleConfigureTechLog)
	mux.HandleFunc("/tools/disable_techlog", s.handleDisableTechLog)
	mux.HandleFunc("/tools/get_techlog_config", s.handleGetTechLogConfig)
	
	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.cfg.MCPPort),
		Handler: mux,
	}
	
	// Start HTTP server in goroutine
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("HTTP server error")
		}
	}()
	
	log.Info().Int("port", s.cfg.MCPPort).Msg("MCP server started")
	
	// Wait for context cancellation
	<-ctx.Done()
	return nil
}

// Stop stops the MCP server gracefully
func (s *Server) Stop() error {
	log.Info().Msg("MCP server stopping...")
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("Error shutting down HTTP server")
		}
	}
	
	if s.chClient != nil {
		if err := s.chClient.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing ClickHouse client")
		}
	}
	
	log.Info().Msg("MCP server stopped")
	return nil
}

// HTTP handlers (simplified REST API for MCP tools)
func (s *Server) handleGetEventLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse JSON request body
	var req struct {
		ClusterGUID  string `json:"cluster_guid"`
		InfobaseGUID string `json:"infobase_guid"`
		From         string `json:"from"`
		To           string `json:"to"`
		Level        string `json:"level,omitempty"`
		Mode         string `json:"mode,omitempty"`
		Limit        int    `json:"limit,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Parse time strings
	fromTime, err := time.Parse(time.RFC3339, req.From)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid 'from' time format: %v", err), http.StatusBadRequest)
		return
	}

	toTime, err := time.Parse(time.RFC3339, req.To)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid 'to' time format: %v", err), http.StatusBadRequest)
		return
	}

	// Build parameters
	params := handlers.EventLogParams{
		ClusterGUID:  req.ClusterGUID,
		InfobaseGUID: req.InfobaseGUID,
		From:         fromTime,
		To:           toTime,
		Level:        req.Level,
		Mode:         req.Mode,
		Limit:        req.Limit,
	}

	// Call handler
	result, err := s.eventLogHandler.GetEventLog(r.Context(), params)
	if err != nil {
		// Check if it's a validation error
		if valErr, ok := err.(*handlers.ValidationError); ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(valErr)
			return
		}

		log.Error().Err(err).Msg("Failed to get event log")
		http.Error(w, fmt.Sprintf("Internal error: %v", err), http.StatusInternalServerError)
		return
	}

	// Return result
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(result))
}

func (s *Server) handleGetTechLog(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{"error": "not implemented yet"})
}

func (s *Server) handleGetNewErrors(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{"error": "not implemented yet"})
}

func (s *Server) handleConfigureTechLog(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{"error": "not implemented yet"})
}

func (s *Server) handleDisableTechLog(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{"error": "not implemented yet"})
}

func (s *Server) handleGetTechLogConfig(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{"error": "not implemented yet"})
}

