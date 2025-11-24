package mcp

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/SteelMorgan/1c-log-checker/internal/handlers"
	"github.com/rs/zerolog/log"
)

//go:embed tools.json
var toolsJSONData []byte

// MCPProtocol implements Model Context Protocol over stdio (JSON-RPC)
type MCPProtocol struct {
	server *Server
	stdin  *bufio.Scanner
	stdout io.Writer
	tools  []Tool // Cached tools loaded from tools.json
}

// MCPRequest represents a JSON-RPC request
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPResponse represents a JSON-RPC response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents a JSON-RPC error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// InitializeRequest represents MCP initialize request
type InitializeRequest struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"clientInfo"`
}

// InitializeResponse represents MCP initialize response
type InitializeResponse struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ToolsListResponse represents tools/list response
type ToolsListResponse struct {
	Tools []Tool `json:"tools"`
}

// ToolCallRequest represents tools/call request
type ToolCallRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// NewMCPProtocol creates a new MCP protocol handler
func NewMCPProtocol(server *Server) (*MCPProtocol, error) {
	// Load tools from tools.json
	tools, err := loadToolsFromJSON()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load tools from tools.json, using fallback")
		// Fallback to empty tools if file not found
		tools = []Tool{}
	}

	return &MCPProtocol{
		server: server,
		stdin:  bufio.NewScanner(os.Stdin),
		stdout: os.Stdout,
		tools:  tools,
	}, nil
}

// loadToolsFromJSON loads tools from embedded tools.json
func loadToolsFromJSON() ([]Tool, error) {
	if len(toolsJSONData) == 0 {
		return nil, fmt.Errorf("embedded tools.json is empty")
	}

	var toolsData struct {
		Tools []Tool `json:"tools"`
	}

	if err := json.Unmarshal(toolsJSONData, &toolsData); err != nil {
		return nil, fmt.Errorf("failed to parse embedded tools.json: %w", err)
	}

	log.Info().Int("count", len(toolsData.Tools)).Msg("Loaded tools from embedded tools.json")
	if len(toolsData.Tools) > 0 {
		for i, tool := range toolsData.Tools {
			log.Debug().Int("index", i).Str("name", tool.Name).Msg("Loaded tool")
		}
	} else {
		log.Warn().Msg("No tools loaded from embedded tools.json!")
	}
	return toolsData.Tools, nil
}

// Start starts the MCP stdio protocol server
func (m *MCPProtocol) Start(ctx context.Context) error {
	log.Info().Msg("MCP stdio protocol server starting...")

	// Process requests from stdin
	for m.stdin.Scan() {
		line := m.stdin.Text()
		if line == "" {
			continue
		}

		log.Debug().Str("raw_line", line).Msg("Received request")

		var req MCPRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			log.Error().Err(err).Str("line", line).Msg("Failed to parse JSON-RPC request")
			m.sendError(nil, -32700, "Parse error", err.Error())
			continue
		}

		log.Info().Str("method", req.Method).Interface("id", req.ID).Msg("Handling request")

		// Handle request
		if err := m.handleRequest(ctx, &req); err != nil {
			log.Error().Err(err).Str("method", req.Method).Msg("Failed to handle request")
		}
	}

	if err := m.stdin.Err(); err != nil {
		return fmt.Errorf("stdin scanner error: %w", err)
	}

	return nil
}

// sendInitialized sends the initialized notification
func (m *MCPProtocol) sendInitialized() error {
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "initialized",
	}
	return m.writeJSON(notification)
}

// handleRequest handles a JSON-RPC request
func (m *MCPProtocol) handleRequest(ctx context.Context, req *MCPRequest) error {
	switch req.Method {
	case "initialize":
		return m.handleInitialize(req)
	case "tools/list":
		return m.handleToolsList(req)
	case "tools/call":
		return m.handleToolCall(ctx, req)
	default:
		m.sendError(req.ID, -32601, "Method not found", fmt.Sprintf("Unknown method: %s", req.Method))
		return nil
	}
}

// handleInitialize handles the initialize request
func (m *MCPProtocol) handleInitialize(req *MCPRequest) error {
	log.Info().Msg("Handling initialize request")
	
	var initReq InitializeRequest
	if err := json.Unmarshal(req.Params, &initReq); err != nil {
		log.Error().Err(err).Msg("Failed to parse initialize params")
		m.sendError(req.ID, -32602, "Invalid params", err.Error())
		return err
	}

	log.Info().
		Str("client_name", initReq.ClientInfo.Name).
		Str("client_version", initReq.ClientInfo.Version).
		Str("protocol_version", initReq.ProtocolVersion).
		Msg("Initialize request received")

	response := MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: InitializeResponse{
			ProtocolVersion: "2024-11-05",
			Capabilities: map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			ServerInfo: struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			}{
				Name:    "1c-log-checker",
				Version: "0.1.0",
			},
		},
	}

	if err := m.writeJSON(response); err != nil {
		return err
	}

	// Send initialized notification AFTER responding to initialize
	log.Info().Msg("Sending initialized notification")
	if err := m.sendInitialized(); err != nil {
		return fmt.Errorf("failed to send initialized notification: %w", err)
	}

	return nil
}

// handleToolsList handles the tools/list request
func (m *MCPProtocol) handleToolsList(req *MCPRequest) error {
	log.Info().Int("tools_count", len(m.tools)).Msg("Handling tools/list request")
	
	// Use cached tools loaded from tools.json
	tools := m.tools
	
	// If tools are empty (fallback case), return empty list
	if len(tools) == 0 {
		log.Warn().Msg("No tools loaded, returning empty list")
	} else {
		log.Info().Msgf("Returning %d tools", len(tools))
		for i, tool := range tools {
			log.Debug().Int("index", i).Str("name", tool.Name).Msg("Tool")
		}
	}

	response := MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: ToolsListResponse{
			Tools: tools,
		},
	}

	log.Debug().Interface("response", response).Msg("Sending tools/list response")
	return m.writeJSON(response)
}

// handleToolCall handles the tools/call request
func (m *MCPProtocol) handleToolCall(ctx context.Context, req *MCPRequest) error {
	var callReq ToolCallRequest
	if err := json.Unmarshal(req.Params, &callReq); err != nil {
		m.sendError(req.ID, -32602, "Invalid params", err.Error())
		return err
	}

	// Route to appropriate handler
	var result interface{}
	var err error

	switch callReq.Name {
	case "logc_get_event_log":
		result, err = m.handleGetEventLogTool(ctx, callReq.Arguments)
	case "logc_get_tech_log":
		result, err = m.handleGetTechLogTool(ctx, callReq.Arguments)
	case "logc_configure_techlog":
		result, err = m.handleConfigureTechLogTool(ctx, callReq.Arguments)
	case "logc_save_techlog":
		result, err = m.handleSaveTechLogTool(ctx, callReq.Arguments)
	case "logc_restore_techlog":
		result, err = m.handleRestoreTechLogTool(ctx, callReq.Arguments)
	case "logc_disable_techlog":
		result, err = m.handleDisableTechLogTool(ctx, callReq.Arguments)
	case "logc_get_techlog_config":
		result, err = m.handleGetTechLogConfigTool(ctx, callReq.Arguments)
	case "logc_get_actual_log_timestamp":
		result, err = m.handleGetActualLogTimestampTool(ctx, callReq.Arguments)
	default:
		m.sendError(req.ID, -32601, "Method not found", fmt.Sprintf("Unknown tool: %s", callReq.Name))
		return nil
	}

	if err != nil {
		m.sendError(req.ID, -32603, "Internal error", err.Error())
		return err
	}

	response := MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": fmt.Sprintf("%v", result),
				},
			},
		},
	}

	return m.writeJSON(response)
}

// Helper methods to convert tool arguments to handler params and call handlers
// These methods will convert the generic map[string]interface{} to specific handler params

func (m *MCPProtocol) handleGetEventLogTool(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	params := handlers.EventLogParams{}

	if v, ok := args["cluster_guid"].(string); ok {
		params.ClusterGUID = v
	}
	if v, ok := args["infobase_guid"].(string); ok {
		params.InfobaseGUID = v
	}

	// Set default mode to "minimal" if not specified
	if v, ok := args["mode"].(string); ok && v != "" {
		params.Mode = v
	} else {
		params.Mode = "minimal"
	}

	// Set default level to "Error" if not specified
	if v, ok := args["level"].(string); ok && v != "" {
		params.Level = v
	} else {
		params.Level = "Error"
	}

	// Set default time range to last 10 minutes if not specified
	now := time.Now()
	if v, ok := args["from"].(string); ok && v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			params.From = t
		} else {
			// If parsing failed, use default
			params.From = now.Add(-10 * time.Minute)
		}
	} else {
		params.From = now.Add(-10 * time.Minute)
	}

	if v, ok := args["to"].(string); ok && v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			params.To = t
		} else {
			// If parsing failed, use default
			params.To = now
		}
	} else {
		params.To = now
	}

	if v, ok := args["limit"].(float64); ok {
		params.Limit = int(v)
	}

	return m.server.eventLogHandler.GetEventLog(ctx, params)
}

func (m *MCPProtocol) handleGetTechLogTool(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	params := handlers.TechLogParams{}

	if v, ok := args["cluster_guid"].(string); ok {
		params.ClusterGUID = v
	}
	if v, ok := args["infobase_guid"].(string); ok {
		params.InfobaseGUID = v
	}
	if v, ok := args["from"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			params.From = t
		}
	}
	if v, ok := args["to"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			params.To = t
		}
	}
	if v, ok := args["name"].(string); ok {
		params.Name = v
	}
	if v, ok := args["mode"].(string); ok {
		params.Mode = v
	}
	if v, ok := args["limit"].(float64); ok {
		params.Limit = int(v)
	}

	return m.server.techLogHandler.GetTechLog(ctx, params)
}

func (m *MCPProtocol) handleConfigureTechLogTool(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	params := handlers.ConfigureTechLogParams{}

	if v, ok := args["cluster_guid"].(string); ok {
		params.ClusterGUID = v
	}
	if v, ok := args["infobase_guid"].(string); ok {
		params.InfobaseGUID = v
	}
	if v, ok := args["location"].(string); ok {
		params.Location = v
	}
	if v, ok := args["history"].(float64); ok {
		params.History = int(v)
	}
	if v, ok := args["format"].(string); ok {
		params.Format = v
	}
	if v, ok := args["events"].([]interface{}); ok {
		events := make([]string, len(v))
		for i, e := range v {
			if s, ok := e.(string); ok {
				events[i] = s
			}
		}
		params.Events = events
	}
	if v, ok := args["properties"].([]interface{}); ok {
		properties := make([]string, len(v))
		for i, p := range v {
			if s, ok := p.(string); ok {
				properties[i] = s
			}
		}
		params.Properties = properties
	}
	if v, ok := args["config_path"].(string); ok {
		params.ConfigPath = v
	}

	return m.server.configureTechHandler.ConfigureTechLog(ctx, params)
}

func (m *MCPProtocol) handleSaveTechLogTool(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	configPath := ""
	if v, ok := args["config_path"].(string); ok {
		configPath = v
	}
	err := m.server.saveTechHandler.SaveTechLog(ctx, configPath)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"status": "saved", "config_path": configPath}, nil
}

func (m *MCPProtocol) handleRestoreTechLogTool(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	configPath := ""
	if v, ok := args["config_path"].(string); ok {
		configPath = v
	}
	err := m.server.restoreTechHandler.RestoreTechLog(ctx, configPath)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"status": "restored", "config_path": configPath}, nil
}

func (m *MCPProtocol) handleDisableTechLogTool(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	configPath := ""
	if v, ok := args["config_path"].(string); ok {
		configPath = v
	}
	err := m.server.disableTechHandler.DisableTechLog(ctx, configPath)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"status": "disabled", "config_path": configPath}, nil
}

func (m *MCPProtocol) handleGetTechLogConfigTool(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	configPath := ""
	if v, ok := args["config_path"].(string); ok {
		configPath = v
	}
	return m.server.getTechCfgHandler.GetTechLogConfig(ctx, configPath)
}

func (m *MCPProtocol) handleGetActualLogTimestampTool(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	baseID := ""
	if v, ok := args["base_id"].(string); ok {
		baseID = v
	}
	return m.server.getActualLogTimestampHandler.GetActualLogTimestamp(ctx, baseID)
}

// sendError sends a JSON-RPC error response
func (m *MCPProtocol) sendError(id interface{}, code int, message string, data interface{}) error {
	response := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	return m.writeJSON(response)
}

// writeJSON writes a JSON object to stdout
func (m *MCPProtocol) writeJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal JSON")
		return err
	}
	data = append(data, '\n')
	
	// Write to stdout and flush to ensure data is sent immediately
	n, err := m.stdout.Write(data)
	if err != nil {
		log.Error().Err(err).Int("bytes_written", n).Msg("Failed to write to stdout")
		return err
	}
	
	// Flush stdout if it's a Flusher (buffered writer)
	if flusher, ok := m.stdout.(interface{ Flush() error }); ok {
		if err := flusher.Flush(); err != nil {
			log.Error().Err(err).Msg("Failed to flush stdout")
			return err
		}
	}
	
	log.Debug().Int("bytes_written", n).Msg("JSON written to stdout")
	return nil
}
