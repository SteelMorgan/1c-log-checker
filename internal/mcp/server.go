package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
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
	eventLogHandler           *handlers.EventLogHandler
	techLogHandler            *handlers.TechLogHandler
	newErrorsHandler          *handlers.NewErrorsHandler
	configureTechHandler      *handlers.ConfigureTechLogHandler
	saveTechHandler           *handlers.SaveTechLogHandler
	restoreTechHandler        *handlers.RestoreTechLogHandler
	disableTechHandler        *handlers.DisableTechLogHandler
	getTechCfgHandler        *handlers.GetTechLogConfigHandler
	getActualLogTimestampHandler *handlers.GetActualLogTimestampHandler
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
	configureTechHandler := handlers.NewConfigureTechLogHandler(cfg.TechLogConfigDir, cfg.TechLogDirs)
	saveTechHandler := handlers.NewSaveTechLogHandler(cfg.TechLogConfigDir)
	restoreTechHandler := handlers.NewRestoreTechLogHandler(cfg.TechLogConfigDir)
	disableTechHandler := handlers.NewDisableTechLogHandler(cfg.TechLogConfigDir)
	getTechCfgHandler := handlers.NewGetTechLogConfigHandler()
	getActualLogTimestampHandler := handlers.NewGetActualLogTimestampHandler(chClient)

	return &Server{
		cfg:                       cfg,
		chClient:                  chClient,
		clusterMap:                clusterMap,
		eventLogHandler:           eventLogHandler,
		techLogHandler:            techLogHandler,
		newErrorsHandler:          newErrorsHandler,
		configureTechHandler:      configureTechHandler,
		saveTechHandler:           saveTechHandler,
		restoreTechHandler:        restoreTechHandler,
		disableTechHandler:        disableTechHandler,
		getTechCfgHandler:         getTechCfgHandler,
		getActualLogTimestampHandler: getActualLogTimestampHandler,
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
	mux.HandleFunc("/tools/get_new_errors_aggregated", s.handleGetNewErrorsAggregated)
	mux.HandleFunc("/tools/configure_techlog", s.handleConfigureTechLog)
	mux.HandleFunc("/tools/save_techlog", s.handleSaveTechLog)
	mux.HandleFunc("/tools/restore_techlog", s.handleRestoreTechLog)
	mux.HandleFunc("/tools/disable_techlog", s.handleDisableTechLog)
	mux.HandleFunc("/tools/get_techlog_config", s.handleGetTechLogConfig)
	mux.HandleFunc("/tools/get_actual_log_timestamp", s.handleGetActualLogTimestamp)
	
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
		Name         string `json:"name,omitempty"`
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
	params := handlers.TechLogParams{
		ClusterGUID:  req.ClusterGUID,
		InfobaseGUID: req.InfobaseGUID,
		From:         fromTime,
		To:           toTime,
		Name:         req.Name,
		Mode:         req.Mode,
		Limit:        req.Limit,
	}

	// Call handler
	result, err := s.techLogHandler.GetTechLog(r.Context(), params)
	if err != nil {
		// Check if it's a validation error
		if valErr, ok := err.(*handlers.ValidationError); ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(valErr)
			return
		}

		log.Error().Err(err).Msg("Failed to get tech log")
		http.Error(w, fmt.Sprintf("Internal error: %v", err), http.StatusInternalServerError)
		return
	}

	// Return result
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(result))
}

func (s *Server) handleGetNewErrorsAggregated(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse JSON request body
	var req struct {
		ClusterGUID  string `json:"cluster_guid"`
		InfobaseGUID string `json:"infobase_guid"`
		Hours        int    `json:"hours,omitempty"`
		Limit        int    `json:"limit,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Build parameters
	params := handlers.NewErrorsParams{
		ClusterGUID:  req.ClusterGUID,
		InfobaseGUID: req.InfobaseGUID,
		Hours:        req.Hours,
		Limit:        req.Limit,
	}

	// Call handler
	result, err := s.newErrorsHandler.GetNewErrorsAggregated(r.Context(), params)
	if err != nil {
		// Check if it's a validation error
		if valErr, ok := err.(*handlers.ValidationError); ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(valErr)
			return
		}

		log.Error().Err(err).Msg("Failed to get new errors aggregated")
		http.Error(w, fmt.Sprintf("Internal error: %v", err), http.StatusInternalServerError)
		return
	}

	// Return result
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(result))
}

func (s *Server) handleConfigureTechLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse JSON request body
	var req struct {
		ClusterGUID  string   `json:"cluster_guid"`
		InfobaseGUID string   `json:"infobase_guid"`
		Location     string   `json:"location"`
		ConfigPath   string   `json:"config_path,omitempty"`
		History      int      `json:"history"`
		Format       string   `json:"format,omitempty"`
		Events       []string `json:"events"`
		Properties   []string `json:"properties,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Build parameters
	params := handlers.ConfigureTechLogParams{
		ClusterGUID:  req.ClusterGUID,
		InfobaseGUID: req.InfobaseGUID,
		Location:     req.Location,
		ConfigPath:   req.ConfigPath,
		History:      req.History,
		Format:       req.Format,
		Events:       req.Events,
		Properties:   req.Properties,
	}

	// Set default format if not provided
	if params.Format == "" {
		params.Format = "text"
	}

	// Call handler
	result, err := s.configureTechHandler.ConfigureTechLog(r.Context(), params)
	if err != nil {
		// Return validation error with details
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Configuration validation failed",
			"message": err.Error(),
		})
		return
	}

	// Return XML configuration
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(result))
}

func (s *Server) handleDisableTechLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse JSON request body
	var req struct {
		ConfigPath string `json:"config_path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// configPath is optional - handler will use default from config
	configPath := req.ConfigPath

	// Call handler
	if err := s.disableTechHandler.DisableTechLog(r.Context(), configPath); err != nil {
		log.Error().Err(err).Msg("Failed to disable tech log")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to disable tech log",
			"message": err.Error(),
		})
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Tech log disabled",
		"path":    configPath,
	})
}

func (s *Server) handleSaveTechLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse JSON request body
	var req struct {
		ConfigPath string `json:"config_path,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// configPath is optional - handler will use default from config
	configPath := req.ConfigPath

	// Call handler
	if err := s.saveTechHandler.SaveTechLog(r.Context(), configPath); err != nil {
		log.Error().Err(err).Msg("Failed to save tech log")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to save tech log",
			"message": err.Error(),
		})
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Tech log config saved as .OLD",
		"path":    configPath,
	})
}

func (s *Server) handleRestoreTechLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse JSON request body
	var req struct {
		ConfigPath string `json:"config_path,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// configPath is optional - handler will use default from config
	configPath := req.ConfigPath

	// Call handler
	if err := s.restoreTechHandler.RestoreTechLog(r.Context(), configPath); err != nil {
		log.Error().Err(err).Msg("Failed to restore tech log")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to restore tech log",
			"message": err.Error(),
		})
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Tech log config restored from .OLD",
		"path":    configPath,
	})
}

func (s *Server) handleGetTechLogConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse JSON request body
	var req struct {
		ConfigPath string `json:"config_path,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Use default path if not provided
	configPath := req.ConfigPath
	if configPath == "" {
		configPath = filepath.Join(s.cfg.TechLogConfigDir, "logcfg.xml")
	}

	// Call handler
	result, err := s.getTechCfgHandler.GetTechLogConfig(r.Context(), configPath)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get tech log config")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to read tech log config",
			"message": err.Error(),
		})
		return
	}

	// Return XML content
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(result))
}

func (s *Server) handleGetActualLogTimestamp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse JSON request body
	var req struct {
		BaseID string `json:"base_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Validate base_id
	if req.BaseID == "" {
		http.Error(w, "base_id is required", http.StatusBadRequest)
		return
	}

	// Call handler
	result, err := s.getActualLogTimestampHandler.GetActualLogTimestamp(r.Context(), req.BaseID)
	if err != nil {
		// Check if it's a validation error
		if valErr, ok := err.(*handlers.ValidationError); ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(valErr)
			return
		}

		log.Error().Err(err).Msg("Failed to get actual log timestamp")
		http.Error(w, fmt.Sprintf("Internal error: %v", err), http.StatusInternalServerError)
		return
	}

	// Return result
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(result))
}

