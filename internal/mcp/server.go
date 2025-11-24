package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	eventLogHandler           *handlers.EventLogHandler
	techLogHandler            *handlers.TechLogHandler
	configureTechHandler      *handlers.ConfigureTechLogHandler
	saveTechHandler           *handlers.SaveTechLogHandler
	restoreTechHandler        *handlers.RestoreTechLogHandler
	disableTechHandler        *handlers.DisableTechLogHandler
	getTechCfgHandler        *handlers.GetTechLogConfigHandler
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
		cfg:                       cfg,
		chClient:                  chClient,
		clusterMap:                clusterMap,
		rateLimiter:               rateLimiter,
		eventLogHandler:           eventLogHandler,
		techLogHandler:            techLogHandler,
		configureTechHandler:      configureTechHandler,
		saveTechHandler:           saveTechHandler,
		restoreTechHandler:        restoreTechHandler,
		disableTechHandler:        disableTechHandler,
		getTechCfgHandler:         getTechCfgHandler,
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

	// MCP protocol endpoints (JSON-RPC over HTTP) - register last as fallback
	// Standard MCP HTTP transport uses root path "/" for JSON-RPC requests
	mux.HandleFunc("/mcp", s.handleMCPRequest) // Also support /mcp for compatibility
	mux.HandleFunc("/", s.handleMCPRequest) // Main MCP endpoint for JSON-RPC requests (fallback for all other paths)

	// Wrap mux with logging middleware to see all requests
	loggingMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Str("content_type", r.Header.Get("Content-Type")).
			Msg("HTTP request received")
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
	log.Info().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Msg("MCP HTTP request received")
	
	// Only handle POST requests for JSON-RPC
	if r.Method != http.MethodPost {
		log.Warn().Str("method", r.Method).Str("path", r.URL.Path).Msg("Method not allowed for MCP request")
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

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Create MCP protocol handler
	protocol, err := NewMCPProtocol(s)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create MCP protocol handler")
		http.Error(w, fmt.Sprintf("Internal error: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert to MCPRequest format
	mcpReq := &MCPRequest{
		JSONRPC: req.JSONRPC,
		ID:      req.ID,
		Method:  req.Method,
		Params:  req.Params,
	}

	// Handle the request directly without capturing stdout
	// For HTTP mode, we need to handle responses differently than stdio
	// because notifications (like "initialized") should not be sent in HTTP responses
	
	// Create a custom handler that captures only the response, not notifications
	var mcpResponse map[string]interface{}
	
	// For initialize method, handle specially to avoid sending initialized notification
	if req.Method == "initialize" {
		// Handle initialize without sending initialized notification
		var initReq struct {
			ProtocolVersion string                 `json:"protocolVersion"`
			Capabilities    map[string]interface{} `json:"capabilities"`
			ClientInfo      struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"clientInfo"`
		}
		if err := json.Unmarshal(req.Params, &initReq); err != nil {
			log.Error().Err(err).Msg("Failed to parse initialize params")
			http.Error(w, fmt.Sprintf("Invalid params: %v", err), http.StatusBadRequest)
			return
		}
		
		log.Info().
			Str("client_name", initReq.ClientInfo.Name).
			Str("client_version", initReq.ClientInfo.Version).
			Str("protocol_version", initReq.ProtocolVersion).
			Msg("Initialize request received")
		
		// Return initialize response without initialized notification
		mcpResponse = map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				"serverInfo": map[string]interface{}{
					"name":    "1c-log-checker",
					"version": "0.1.0",
				},
			},
		}
	} else {
		// For other methods (tools/list, tools/call), use protocol handler
		// Create response capture
		var responseData []byte
		responseWriter := &responseCapture{data: &responseData}
		
		// Temporarily replace stdout to capture response
		oldStdout := protocol.stdout
		protocol.stdout = responseWriter
		defer func() { protocol.stdout = oldStdout }()

		// Handle the request
		if err := protocol.handleRequest(r.Context(), mcpReq); err != nil {
			log.Error().Err(err).Str("method", req.Method).Msg("Failed to handle MCP request")
			http.Error(w, fmt.Sprintf("Internal error: %v", err), http.StatusInternalServerError)
			return
		}

		// Parse captured response
		// Response may contain multiple JSON objects (response + notification)
		// We need to parse only the first one (the actual response)
		responseStr := string(responseData)
		
		// Find the first complete JSON object (ends with newline or is the only object)
		lines := strings.Split(responseStr, "\n")
		var firstJSONLine string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				firstJSONLine = line
				break
			}
		}
		
		if firstJSONLine == "" {
			log.Error().Str("raw_response", responseStr).Msg("No JSON found in response")
			http.Error(w, "Internal error: empty response", http.StatusInternalServerError)
			return
		}
		
		if err := json.Unmarshal([]byte(firstJSONLine), &mcpResponse); err != nil {
			log.Error().Err(err).Str("raw_response", firstJSONLine).Msg("Failed to parse MCP response")
			http.Error(w, fmt.Sprintf("Internal error: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Send response
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(mcpResponse)
}

// responseCapture captures written data
type responseCapture struct {
	data *[]byte
}

func (w *responseCapture) Write(data []byte) (int, error) {
	*w.data = append(*w.data, data...)
	return len(data), nil
}

