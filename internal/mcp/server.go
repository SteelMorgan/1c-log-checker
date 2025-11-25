package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/SteelMorgan/1c-log-checker/internal/clickhouse"
	"github.com/SteelMorgan/1c-log-checker/internal/config"
	"github.com/SteelMorgan/1c-log-checker/internal/handlers"
	"github.com/SteelMorgan/1c-log-checker/internal/mapping"
	"github.com/SteelMorgan/1c-log-checker/internal/ratelimit"
	"github.com/rs/zerolog/log"
)

// Server implements MCP protocol server
type Server struct {
	cfg         *config.Config
	httpServer  *http.Server
	chClient    *clickhouse.Client
	clusterMap  *mapping.ClusterMap
	rateLimiter *ratelimit.Limiter

	// Handlers
	eventLogHandler              *handlers.EventLogHandler
	techLogHandler               *handlers.TechLogHandler
	configureTechHandler         *handlers.ConfigureTechLogHandler
	saveTechHandler              *handlers.SaveTechLogHandler
	restoreTechHandler           *handlers.RestoreTechLogHandler
	disableTechHandler           *handlers.DisableTechLogHandler
	getTechCfgHandler            *handlers.GetTechLogConfigHandler
	getActualLogTimestampHandler *handlers.GetActualLogTimestampHandler
}

// NewServer creates a new MCP server
func NewServer(cfg *config.Config) (*Server, error) {
	log.Info().Msg("Initializing MCP server...")
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	log.Info().
		Str("clickhouse_host", cfg.ClickHouseHost).
		Int("clickhouse_port", cfg.ClickHousePort).
		Str("clickhouse_db", cfg.ClickHouseDB).
		Int("mcp_port", cfg.MCPPort).
		Msg("MCP server configuration")

	// Connect to ClickHouse with retry configuration
	log.Info().Msg("Connecting to ClickHouse...")
	chClient, err := clickhouse.NewClientFromConfig(
		cfg.ClickHouseHost,
		cfg.ClickHousePort,
		cfg.ClickHouseDB,
		cfg.RetryMaxAttempts,
		cfg.RetryInitialDelay,
		cfg.RetryMaxDelay,
		cfg.RetryMultiplier,
	)
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
	configureTechHandler := handlers.NewConfigureTechLogHandler(cfg.TechLogConfigDir, cfg.TechLogDirs)
	saveTechHandler := handlers.NewSaveTechLogHandler(cfg.TechLogConfigDir)
	restoreTechHandler := handlers.NewRestoreTechLogHandler(cfg.TechLogConfigDir)
	disableTechHandler := handlers.NewDisableTechLogHandler(cfg.TechLogConfigDir)
	getTechCfgHandler := handlers.NewGetTechLogConfigHandler()
	getActualLogTimestampHandler := handlers.NewGetActualLogTimestampHandler(chClient)

	// Initialize rate limiter (100 requests per second, burst of 20)
	rateLimiter := ratelimit.NewLimiter(100, 20)

	return &Server{
		cfg:                          cfg,
		chClient:                     chClient,
		clusterMap:                   clusterMap,
		rateLimiter:                  rateLimiter,
		eventLogHandler:              eventLogHandler,
		techLogHandler:               techLogHandler,
		configureTechHandler:         configureTechHandler,
		saveTechHandler:              saveTechHandler,
		restoreTechHandler:           restoreTechHandler,
		disableTechHandler:           disableTechHandler,
		getTechCfgHandler:            getTechCfgHandler,
		getActualLogTimestampHandler: getActualLogTimestampHandler,
	}, nil
}

// Start starts the MCP server (HTTP or stdio mode)
func (s *Server) Start(ctx context.Context) error {
	// Check if running in stdio mode (no port configured or MCP_MODE=stdio)
	mcpMode := os.Getenv("MCP_MODE")
	if mcpMode == "stdio" || s.cfg.MCPPort == 0 {
		return s.startStdio(ctx)
	}

	// Default: HTTP mode
	return s.startHTTP(ctx)
}

// startHTTP starts the MCP server in HTTP mode
func (s *Server) startHTTP(ctx context.Context) error {
	log.Info().
		Int("port", s.cfg.MCPPort).
		Msg("MCP server starting in HTTP mode...")

	// Setup HTTP server with MCP tool endpoints
	mux := http.NewServeMux()

	// Register specific endpoints first (more specific paths before generic ones)
	// Health check (no rate limiting)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Register tool endpoints with rate limiting (legacy REST API)
	mux.Handle("/tools/logc_get_event_log", s.rateLimiter.HTTPMiddleware(http.HandlerFunc(s.handleGetEventLog)))
	mux.Handle("/tools/logc_get_tech_log", s.rateLimiter.HTTPMiddleware(http.HandlerFunc(s.handleGetTechLog)))
	mux.Handle("/tools/logc_configure_techlog", s.rateLimiter.HTTPMiddleware(http.HandlerFunc(s.handleConfigureTechLog)))
	mux.Handle("/tools/logc_save_techlog", s.rateLimiter.HTTPMiddleware(http.HandlerFunc(s.handleSaveTechLog)))
	mux.Handle("/tools/logc_restore_techlog", s.rateLimiter.HTTPMiddleware(http.HandlerFunc(s.handleRestoreTechLog)))
	mux.Handle("/tools/logc_disable_techlog", s.rateLimiter.HTTPMiddleware(http.HandlerFunc(s.handleDisableTechLog)))
	mux.Handle("/tools/logc_get_techlog_config", s.rateLimiter.HTTPMiddleware(http.HandlerFunc(s.handleGetTechLogConfig)))
	mux.Handle("/tools/logc_get_actual_log_timestamp", s.rateLimiter.HTTPMiddleware(http.HandlerFunc(s.handleGetActualLogTimestamp)))

	// MCP protocol endpoints (JSON-RPC over HTTP)
	// MCP HTTP transport may use different paths, so register multiple endpoints
	mux.HandleFunc("/mcp", s.handleMCPRequest)      // Explicit /mcp endpoint
	mux.HandleFunc("/sse", s.handleMCPRequest)      // SSE endpoint (some clients use this)
	mux.HandleFunc("/messages", s.handleMCPRequest) // Messages endpoint (some clients use this)
	mux.HandleFunc("/", s.handleMCPRequest)         // Root endpoint (fallback for all other paths)

	// Wrap mux with logging middleware to see all requests
	// This MUST be called for every request, so if we don't see logs, requests aren't reaching the server
	loggingMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Force immediate logging to stderr for debugging (this should ALWAYS appear)
		fmt.Fprintf(os.Stderr, "[LOGGING_MUX] %s %s from %s\n", r.Method, r.URL.Path, r.RemoteAddr)
		fmt.Fprintf(os.Stderr, "[LOGGING_MUX] URL: %s, Host: %s\n", r.URL.String(), r.Host)

		// Force log to both stdout and file
		log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Str("content_type", r.Header.Get("Content-Type")).
			Str("url", r.URL.String()).
			Str("host", r.Host).
			Str("step", "LOGGING_MUX").
			Msg("HTTP request received")

		// Call the actual mux
		mux.ServeHTTP(w, r)
	})

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.cfg.MCPPort),
		Handler: loggingMux,
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

// startStdio starts the MCP server in stdio mode
func (s *Server) startStdio(ctx context.Context) error {
	log.Info().Msg("MCP server starting in stdio mode...")

	protocol, err := NewMCPProtocol(s)
	if err != nil {
		return fmt.Errorf("failed to create MCP protocol: %w", err)
	}
	return protocol.Start(ctx)
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
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(valErr)
			return
		}

		log.Error().Err(err).Msg("Failed to get event log")
		http.Error(w, fmt.Sprintf("Internal error: %v", err), http.StatusInternalServerError)
		return
	}

	// Return result
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
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
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(valErr)
			return
		}

		log.Error().Err(err).Msg("Failed to get tech log")
		http.Error(w, fmt.Sprintf("Internal error: %v", err), http.StatusInternalServerError)
		return
	}

	// Return result
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
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
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
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
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to disable tech log",
			"message": err.Error(),
		})
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
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
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to save tech log",
			"message": err.Error(),
		})
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
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
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to restore tech log",
			"message": err.Error(),
		})
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
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
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
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
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(valErr)
			return
		}

		log.Error().Err(err).Msg("Failed to get actual log timestamp")
		http.Error(w, fmt.Sprintf("Internal error: %v", err), http.StatusInternalServerError)
		return
	}

	// Return result
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(result))
}

// handleMCPRequest handles MCP protocol requests over HTTP (JSON-RPC)
// This allows MCP clients to connect via HTTP instead of stdio
func (s *Server) handleMCPRequest(w http.ResponseWriter, r *http.Request) {
	// Force immediate logging to ensure it's written
	log.Info().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Str("url", r.URL.String()).
		Str("host", r.Host).
		Msg("MCP HTTP request received")

	// Also log to stderr for immediate visibility
	fmt.Fprintf(os.Stderr, "[MCP] Request: %s %s from %s\n", r.Method, r.URL.Path, r.RemoteAddr)
	os.Stderr.Sync()
	log.Info().
		Str("step", "MCP").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Msg("MCP request received (stderr)")

	// Only handle POST requests for JSON-RPC
	if r.Method != http.MethodPost {
		log.Warn().Str("method", r.Method).Str("path", r.URL.Path).Msg("Method not allowed for MCP request")
		fmt.Fprintf(os.Stderr, "[MCP] ERROR: Method not allowed: %s\n", r.Method)
		os.Stderr.Sync()
		log.Error().
			Str("step", "MCP").
			Str("method", r.Method).
			Msg("Method not allowed (stderr)")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse JSON-RPC request
	var req struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      interface{}     `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
	}

	log.Info().Msg("[BEFORE STEP 1] About to parse JSON-RPC request body")
	fmt.Fprintf(os.Stderr, "[STEP 1] Starting JSON decode from request body\n")
	os.Stderr.Sync()
	log.Info().Str("step", "STEP 1").Msg("Starting JSON decode from request body")
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fmt.Fprintf(os.Stderr, "[STEP 1 ERROR] Failed to decode JSON: %v\n", err)
		os.Stderr.Sync()
		log.Error().Str("step", "STEP 1").Err(err).Msg("Failed to decode JSON")
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	fmt.Fprintf(os.Stderr, "[STEP 1 OK] JSON decoded: method=%s, id=%v, params_len=%d\n", req.Method, req.ID, len(req.Params))
	os.Stderr.Sync()
	log.Info().
		Str("step", "STEP 1").
		Str("method", req.Method).
		Interface("id", req.ID).
		Int("params_len", len(req.Params)).
		Msg("JSON decoded successfully")

	// Create MCP protocol handler with skipNotifications=true for HTTP mode
	fmt.Fprintf(os.Stderr, "[STEP 2] Creating MCP protocol handler\n")
	os.Stderr.Sync()
	log.Info().Str("step", "STEP 2").Msg("Creating MCP protocol handler")
	log.Info().Msg("Creating MCP protocol handler with skipNotifications=true for HTTP mode")
	protocol, err := NewMCPProtocolWithOptions(s, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[STEP 2 ERROR] Failed to create protocol: %v\n", err)
		os.Stderr.Sync()
		log.Error().Str("step", "STEP 2").Err(err).Msg("Failed to create protocol")
		log.Error().Err(err).Msg("Failed to create MCP protocol handler")
		http.Error(w, fmt.Sprintf("Internal error: %v", err), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(os.Stderr, "[STEP 2 OK] Protocol handler created\n")
	os.Stderr.Sync()
	log.Info().Str("step", "STEP 2").Bool("skip_notifications", protocol.skipNotifications).Msg("Protocol handler created")
	log.Info().Bool("skip_notifications", protocol.skipNotifications).Msg("MCP protocol handler created")

	// Convert to MCPRequest format
	fmt.Fprintf(os.Stderr, "[STEP 3] Converting to MCPRequest format\n")
	os.Stderr.Sync()
	log.Info().Str("step", "STEP 3").Msg("Converting to MCPRequest format")
	mcpReq := &MCPRequest{
		JSONRPC: req.JSONRPC,
		ID:      req.ID,
		Method:  req.Method,
		Params:  req.Params,
	}
	fmt.Fprintf(os.Stderr, "[STEP 3 OK] MCPRequest created: method=%s, id=%v\n", mcpReq.Method, mcpReq.ID)
	os.Stderr.Sync()
	log.Info().
		Str("step", "STEP 3").
		Str("method", mcpReq.Method).
		Interface("id", mcpReq.ID).
		Msg("MCPRequest created")

	// For HTTP mode, capture response from protocol handler
	// Create response capture that only captures the first JSON object
	var responseData []byte
	var responseCaptured bool
	responseWriter := &httpResponseWriter{
		data:     &responseData,
		captured: &responseCaptured,
	}

	// Temporarily replace stdout to capture response
	fmt.Fprintf(os.Stderr, "[STEP 4] Setting up response capture\n")
	os.Stderr.Sync()
	log.Info().Str("step", "STEP 4").Msg("Setting up response capture")
	oldStdout := protocol.stdout
	protocol.stdout = responseWriter
	defer func() { protocol.stdout = oldStdout }()
	fmt.Fprintf(os.Stderr, "[STEP 4 OK] Response capture setup complete\n")
	os.Stderr.Sync()
	log.Info().Str("step", "STEP 4").Msg("Response capture setup complete")

	log.Info().Str("method", req.Method).Interface("id", req.ID).Msg("Handling MCP request in HTTP mode")

	// Log request details before handling - FORCE OUTPUT
	paramsPreview := ""
	if len(req.Params) > 0 {
		previewLen := min(len(req.Params), 200)
		paramsPreview = string(req.Params[:previewLen])
	}
	fmt.Fprintf(os.Stderr, "[STEP 5] About to call protocol.handleRequest: method=%s, id=%v, params_len=%d, preview=%.100s\n", req.Method, req.ID, len(req.Params), paramsPreview)
	os.Stderr.Sync()
	log.Info().
		Str("step", "STEP 5").
		Str("method", req.Method).
		Interface("id", req.ID).
		Int("params_len", len(req.Params)).
		Str("params_preview", paramsPreview).
		Msg("About to call protocol.handleRequest")
	log.Info().
		Str("method", req.Method).
		Interface("id", req.ID).
		Str("params_preview", paramsPreview).
		Msg("About to call protocol.handleRequest")

	// Handle the request (protocol will skip notifications in HTTP mode)
	fmt.Fprintf(os.Stderr, "[STEP 5] Calling protocol.handleRequest...\n")
	os.Stderr.Sync()
	log.Info().Str("step", "STEP 5").Msg("Calling protocol.handleRequest")
	if err := protocol.handleRequest(r.Context(), mcpReq); err != nil {
		fmt.Fprintf(os.Stderr, "[STEP 5 ERROR] handleRequest returned error: %v\n", err)
		os.Stderr.Sync()
		log.Error().Str("step", "STEP 5").Err(err).Msg("handleRequest returned error")
		log.Error().Err(err).Str("method", req.Method).Msg("Failed to handle MCP request")
		http.Error(w, fmt.Sprintf("Internal error: %v", err), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(os.Stderr, "[STEP 5 OK] protocol.handleRequest completed successfully\n")
	os.Stderr.Sync()
	log.Info().Str("step", "STEP 5").Msg("protocol.handleRequest completed successfully")

	log.Info().Int("response_bytes", len(responseData)).Bool("captured", responseCaptured).Msg("Request handled, checking response")

	// Parse captured response (only first JSON object, ignore notifications)
	if !responseCaptured || len(responseData) == 0 {
		fmt.Fprintf(os.Stderr, "[handleMCPRequest ERROR] No response data captured: bytes=%d, captured=%v\n", len(responseData), responseCaptured)
		log.Error().Int("response_bytes", len(responseData)).Bool("captured", responseCaptured).Msg("No response data captured or not marked as captured")
		// Log what was in responseData for debugging
		if len(responseData) > 0 {
			fmt.Fprintf(os.Stderr, "[handleMCPRequest DEBUG] Response data preview: %.200s\n", string(responseData))
			log.Debug().Str("response_preview", string(responseData[:min(len(responseData), 200)])).Msg("Response data preview")
		}
		http.Error(w, "Internal error: no response", http.StatusInternalServerError)
		return
	}

	// Remove trailing newline if present
	jsonData := responseData
	if len(jsonData) > 0 && jsonData[len(jsonData)-1] == '\n' {
		jsonData = jsonData[:len(jsonData)-1]
	}

	log.Debug().Int("json_bytes", len(jsonData)).Str("preview", string(jsonData[:min(len(jsonData), 200)])).Msg("Attempting to parse JSON response")

	var mcpResponse map[string]interface{}
	if err := json.Unmarshal(jsonData, &mcpResponse); err != nil {
		log.Error().Err(err).Int("data_len", len(jsonData)).Str("data", string(jsonData[:min(len(jsonData), 500)])).Msg("Failed to parse MCP response")
		http.Error(w, fmt.Sprintf("Internal error: %v", err), http.StatusInternalServerError)
		return
	}

	log.Info().Str("method", req.Method).Msg("Sending HTTP response")

	// Send response
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(mcpResponse); err != nil {
		log.Error().Err(err).Msg("Failed to encode response")
		return
	}

	log.Info().Str("method", req.Method).Msg("HTTP response sent successfully")
}

// httpResponseWriter captures written data (only first JSON object for HTTP responses)
type httpResponseWriter struct {
	data     *[]byte
	captured *bool
}

func (w *httpResponseWriter) Write(data []byte) (int, error) {
	// Only capture first JSON object (response), ignore subsequent notifications
	if !*w.captured {
		log.Debug().Int("bytes_received", len(data)).Str("preview", string(data[:min(len(data), 50)])).Msg("httpResponseWriter: Received data")
		*w.data = append(*w.data, data...)

		// Check if we have a complete JSON object
		// In MCP protocol, responses are single-line JSON objects ending with \n
		jsonData := *w.data

		// Find the end of the first JSON object (either \n or end of data)
		// JSON objects in MCP are always on a single line
		endIdx := len(jsonData)
		for i := 0; i < len(jsonData); i++ {
			if jsonData[i] == '\n' {
				endIdx = i
				break
			}
		}

		log.Debug().Int("total_bytes", len(jsonData)).Int("end_idx", endIdx).Msg("httpResponseWriter: Checking for complete JSON")

		// Extract the JSON object (without trailing newline)
		if endIdx > 0 {
			jsonObj := jsonData[:endIdx]

			// Try to unmarshal to verify it's valid JSON
			var test map[string]interface{}
			if err := json.Unmarshal(jsonObj, &test); err == nil {
				// Valid JSON found, mark as captured
				log.Info().Int("bytes_captured", len(jsonObj)).Str("preview", string(jsonObj[:min(len(jsonObj), 100)])).Msg("httpResponseWriter: Captured first JSON object")
				*w.captured = true
				*w.data = jsonObj // Store only the JSON object without newline
			} else {
				// JSON might not be complete yet, wait for more data
				log.Debug().Int("bytes_total", len(jsonData)).Err(err).Str("preview", string(jsonObj[:min(len(jsonObj), 200)])).Msg("httpResponseWriter: JSON not complete yet, waiting for more data")
			}
		}
	} else {
		log.Debug().Int("bytes_ignored", len(data)).Msg("httpResponseWriter: Ignoring subsequent data (notification)")
	}
	return len(data), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
