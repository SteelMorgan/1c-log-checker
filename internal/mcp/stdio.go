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

// supportedProtocolVersions lists MCP protocol versions this server supports.
var supportedProtocolVersions = []string{
	"2025-06-18",
	"2024-11-05",
}

// MCPProtocol implements Model Context Protocol over stdio (JSON-RPC)
type MCPProtocol struct {
	server            *Server
	stdin             *bufio.Scanner
	stdout            io.Writer
	tools             []Tool // Cached tools loaded from tools.json
	skipNotifications bool   // If true, don't send notifications (for HTTP mode)
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
	return NewMCPProtocolWithOptions(server, false)
}

// NewMCPProtocolWithOptions creates a new MCP protocol handler with options
func NewMCPProtocolWithOptions(server *Server, skipNotifications bool) (*MCPProtocol, error) {
	// Load tools from tools.json
	tools, err := loadToolsFromJSON()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load tools from tools.json, using fallback")
		// Fallback to empty tools if file not found
		tools = []Tool{}
	}

	log.Info().Bool("skip_notifications", skipNotifications).Msg("Creating MCPProtocol with options")
	return &MCPProtocol{
		server:            server,
		stdin:             bufio.NewScanner(os.Stdin),
		stdout:            os.Stdout,
		tools:             tools,
		skipNotifications: skipNotifications,
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
	for i, tool := range toolsData.Tools {
		log.Debug().Int("index", i).Str("name", tool.Name).Msg("Loaded tool")
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

// handleRequest handles a JSON-RPC request
func (m *MCPProtocol) handleRequest(ctx context.Context, req *MCPRequest) error {
	switch req.Method {
	case "initialize":
		return m.handleInitialize(req)
	case "notifications/initialized":
		return m.handleInitializedNotification(req)
	case "ping":
		return m.handlePing(req)
	case "tools/list":
		return m.handleToolsList(req)
	case "tools/call":
		return m.handleToolCall(ctx, req)
	default:
		log.Warn().Str("method", req.Method).Msg("Unknown method")
		if req.ID == nil {
			return nil
		}
		m.sendError(req.ID, -32601, "Method not found", fmt.Sprintf("Unknown method: %s", req.Method))
		return nil
	}
}

// handleInitialize handles the initialize request
func (m *MCPProtocol) handleInitialize(req *MCPRequest) error {
	log.Info().Bool("skip_notifications", m.skipNotifications).Msg("Handling initialize request")

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
			ProtocolVersion: negotiatedProtocolVersion(initReq.ProtocolVersion),
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

	return m.writeJSON(response)
}

func (m *MCPProtocol) handleInitializedNotification(req *MCPRequest) error {
	log.Info().
		Bool("skip_notifications", m.skipNotifications).
		Bool("has_id", req.ID != nil).
		Msg("Received notifications/initialized")

	// notifications/initialized is a client notification and must not produce a response.
	return nil
}

func (m *MCPProtocol) handlePing(req *MCPRequest) error {
	if req.ID == nil {
		return nil
	}

	response := MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{},
	}

	return m.writeJSON(response)
}

// handleToolsList handles the tools/list request
func (m *MCPProtocol) handleToolsList(req *MCPRequest) error {
	log.Info().Int("tools_count", len(m.tools)).Msg("Handling tools/list request")

	response := MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: ToolsListResponse{
			Tools: m.tools,
		},
	}

	return m.writeJSON(response)
}

// handleToolCall handles the tools/call request
func (m *MCPProtocol) handleToolCall(ctx context.Context, req *MCPRequest) error {
	var callReq ToolCallRequest
	if err := json.Unmarshal(req.Params, &callReq); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal tool call request")
		m.sendError(req.ID, -32602, "Invalid params", err.Error())
		return err
	}

	log.Info().
		Str("tool_name", callReq.Name).
		Interface("arguments", callReq.Arguments).
		Msg("Tool call request")

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
		log.Error().Err(err).Str("tool", callReq.Name).Msg("Tool handler returned error")
		m.sendError(req.ID, -32603, "Internal error", err.Error())
		return err
	}

	// Format result as string
	var resultText string
	if str, ok := result.(string); ok {
		resultText = str
	} else {
		resultText = fmt.Sprintf("%v", result)
	}

	if resultText == "" {
		resultText = "[]"
	}

	log.Info().Str("tool", callReq.Name).Int("result_len", len(resultText)).Msg("Tool call completed")

	response := MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": resultText,
				},
			},
		},
	}

	return m.writeJSON(response)
}

// Helper methods to convert tool arguments to handler params and call handlers

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

	// Parse time range — if not specified, leave zero (handler will skip time filter)
	if v, ok := args["from"].(string); ok && v != "" {
		if t, err := parseTime(v); err == nil {
			params.From = t
		} else {
			log.Warn().Err(err).Str("input", v).Msg("Failed to parse 'from', ignoring")
		}
	}

	if v, ok := args["to"].(string); ok && v != "" {
		if t, err := parseTime(v); err == nil {
			params.To = t
		} else {
			log.Warn().Err(err).Str("input", v).Msg("Failed to parse 'to', ignoring")
		}
	}

	// Parse limit - can be int or float64 from JSON
	if v, ok := args["limit"].(float64); ok {
		params.Limit = int(v)
	} else if v, ok := args["limit"].(int); ok {
		params.Limit = v
	} else if v, ok := args["limit"].(int64); ok {
		params.Limit = int(v)
	}

	log.Info().
		Str("cluster_guid", params.ClusterGUID).
		Str("infobase_guid", params.InfobaseGUID).
		Str("level", params.Level).
		Str("mode", params.Mode).
		Int("limit", params.Limit).
		Time("from", params.From).
		Time("to", params.To).
		Msg("Calling GetEventLog")

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
	// Parse limit - can be int or float64 from JSON
	if v, ok := args["limit"].(float64); ok {
		params.Limit = int(v)
	} else if v, ok := args["limit"].(int); ok {
		params.Limit = v
	} else if v, ok := args["limit"].(int64); ok {
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
	if id == nil {
		return nil
	}

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

// negotiatedProtocolVersion returns the highest mutually supported protocol version.
func negotiatedProtocolVersion(clientVersion string) string {
	for _, v := range supportedProtocolVersions {
		if v == clientVersion {
			return v
		}
	}
	// Client version not supported — return our latest
	return supportedProtocolVersions[0]
}

// parseTime tries RFC3339 first, then without timezone (assumes UTC).
func parseTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("cannot parse time: %s", s)
}
