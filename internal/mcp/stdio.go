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
	// Force log output - this MUST appear
	fmt.Fprintf(os.Stderr, "[STEP 6] handleRequest: ENTRY - method=%s, id=%v\n", req.Method, req.ID)
	log.Info().
		Str("step", "STEP 6").
		Str("method", req.Method).
		Interface("id", req.ID).
		Msg("handleRequest: ENTRY")
	log.Info().
		Str("method", req.Method).
		Interface("id", req.ID).
		Msg("handleRequest: processing request")

	switch req.Method {
	case "initialize":
		fmt.Fprintf(os.Stderr, "[STEP 6] Routing to handleInitialize\n")
		log.Info().Str("step", "STEP 6").Msg("Routing to handleInitialize")
		return m.handleInitialize(req)
	case "tools/list":
		fmt.Fprintf(os.Stderr, "[STEP 6] Routing to handleToolsList\n")
		log.Info().Str("step", "STEP 6").Msg("Routing to handleToolsList")
		return m.handleToolsList(req)
	case "tools/call":
		fmt.Fprintf(os.Stderr, "[STEP 6] Routing to handleToolCall\n")
		log.Info().Str("step", "STEP 6").Msg("Routing to handleToolCall")
		log.Info().Str("method", req.Method).Msg("handleRequest: routing to handleToolCall")
		return m.handleToolCall(ctx, req)
	default:
		fmt.Fprintf(os.Stderr, "[STEP 6 ERROR] Unknown method: %s\n", req.Method)
		log.Error().Str("step", "STEP 6").Str("method", req.Method).Msg("Unknown method")
		log.Warn().Str("method", req.Method).Msg("handleRequest: unknown method")
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
	// Skip in HTTP mode (notifications are not sent in HTTP responses)
	// Only send notification if skipNotifications is false (stdio mode)
	log.Info().Bool("skip_notifications", m.skipNotifications).Msg("Checking whether to send initialized notification")
	if !m.skipNotifications {
		log.Info().Msg("Sending initialized notification (stdio mode)")
		if err := m.sendInitialized(); err != nil {
			return fmt.Errorf("failed to send initialized notification: %w", err)
		}
	} else {
		log.Info().Bool("skip_notifications", m.skipNotifications).Msg("Skipping initialized notification (HTTP mode)")
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
	fmt.Fprintf(os.Stderr, "[STEP 7] handleToolCall: ENTRY - params_len=%d\n", len(req.Params))
	log.Info().Str("step", "STEP 7").Int("params_len", len(req.Params)).Msg("handleToolCall: ENTRY")

	var callReq ToolCallRequest
	fmt.Fprintf(os.Stderr, "[STEP 7.1] Unmarshaling params...\n")
	log.Info().Str("step", "STEP 7.1").Msg("Unmarshaling params")
	if err := json.Unmarshal(req.Params, &callReq); err != nil {
		fmt.Fprintf(os.Stderr, "[STEP 7.1 ERROR] Failed to unmarshal: %v\n", err)
		log.Error().Str("step", "STEP 7.1").Err(err).Msg("Failed to unmarshal")
		log.Error().Err(err).Msg("Failed to unmarshal tool call request")
		m.sendError(req.ID, -32602, "Invalid params", err.Error())
		return err
	}
	fmt.Fprintf(os.Stderr, "[STEP 7.1 OK] Unmarshaled: tool_name=%s, args_count=%d\n", callReq.Name, len(callReq.Arguments))
	log.Info().
		Str("step", "STEP 7.1").
		Str("tool_name", callReq.Name).
		Int("args_count", len(callReq.Arguments)).
		Msg("Unmarshaled successfully")

	// Log tool call request
	fmt.Fprintf(os.Stderr, "[STEP 7.2] Tool call: name=%s\n", callReq.Name)
	log.Info().Str("step", "STEP 7.2").Str("tool_name", callReq.Name).Msg("Tool call")
	log.Info().
		Str("tool_name", callReq.Name).
		Interface("arguments", callReq.Arguments).
		Msg("handleToolCall: received tool call request")

	// Route to appropriate handler
	var result interface{}
	var err error

	fmt.Fprintf(os.Stderr, "[STEP 7.3] Switching on tool name: %s\n", callReq.Name)
	log.Info().Str("step", "STEP 7.3").Str("tool_name", callReq.Name).Msg("Switching on tool name")
	switch callReq.Name {
	case "logc_get_event_log":
		fmt.Fprintf(os.Stderr, "[STEP 7.3] Case: logc_get_event_log - calling handleGetEventLogTool\n")
		log.Info().Str("step", "STEP 7.3").Str("tool", callReq.Name).Msg("Case: logc_get_event_log - calling handleGetEventLogTool")
		log.Info().Str("tool", callReq.Name).Interface("args", callReq.Arguments).Msg("Routing to handleGetEventLogTool")
		result, err = m.handleGetEventLogTool(ctx, callReq.Arguments)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[STEP 7.3 ERROR] handleGetEventLogTool returned error: %v\n", err)
			log.Error().Str("step", "STEP 7.3").Err(err).Msg("handleGetEventLogTool returned error")
			log.Error().Err(err).Str("tool", callReq.Name).Msg("handleGetEventLogTool returned error")
		} else {
			resultStr := fmt.Sprintf("%v", result)
			fmt.Fprintf(os.Stderr, "[STEP 7.3 OK] handleGetEventLogTool returned: type=%T, len=%d, preview=%.100s\n", result, len(resultStr), resultStr)
			log.Info().
				Str("step", "STEP 7.3").
				Str("tool", callReq.Name).
				Str("result_type", fmt.Sprintf("%T", result)).
				Int("result_len", len(resultStr)).
				Str("result_preview", fmt.Sprintf("%.200v", result)).
				Msg("handleGetEventLogTool returned successfully")
			log.Info().
				Str("tool", callReq.Name).
				Str("result_type", fmt.Sprintf("%T", result)).
				Str("result_preview", fmt.Sprintf("%.200v", result)).
				Int("result_len", len(resultStr)).
				Msg("handleGetEventLogTool returned successfully")
		}
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

	fmt.Fprintf(os.Stderr, "[STEP 7.4] Checking for errors...\n")
	log.Info().Str("step", "STEP 7.4").Msg("Checking for errors")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[STEP 7.4 ERROR] Tool handler returned error: %v\n", err)
		log.Error().Str("step", "STEP 7.4").Err(err).Msg("Tool handler returned error")
		log.Error().Err(err).Str("tool", callReq.Name).Msg("Tool handler returned error")
		m.sendError(req.ID, -32603, "Internal error", err.Error())
		return err
	}
	fmt.Fprintf(os.Stderr, "[STEP 7.4 OK] No errors\n")
	log.Info().Str("step", "STEP 7.4").Msg("No errors")

	// Log result before formatting
	resultStr := fmt.Sprintf("%v", result)
	fmt.Fprintf(os.Stderr, "[STEP 7.5] Formatting result: type=%T, len=%d, preview=%.100s\n", result, len(resultStr), resultStr)
	log.Info().
		Str("step", "STEP 7.5").
		Str("result_type", fmt.Sprintf("%T", result)).
		Int("result_len", len(resultStr)).
		Msg("Formatting result")
	log.Info().
		Str("tool", callReq.Name).
		Str("result_type", fmt.Sprintf("%T", result)).
		Str("result_value", fmt.Sprintf("%.500v", result)).
		Msg("Tool handler completed successfully")

	// Format result as string
	// If result is already a string (from GetEventLog/GetTechLog), use it directly
	var resultText string
	if str, ok := result.(string); ok {
		resultText = str
	} else {
		resultText = fmt.Sprintf("%v", result)
	}
	
	if resultText == "" {
		fmt.Fprintf(os.Stderr, "[STEP 7.5 WARN] Result is empty string, converting to []\n")
		log.Warn().Str("step", "STEP 7.5").Msg("Result is empty string, converting to []")
		resultText = "[]" // Return empty array JSON instead of empty string
		log.Warn().Str("tool", callReq.Name).Msg("Handler returned empty string, converting to empty array")
	}
	fmt.Fprintf(os.Stderr, "[STEP 7.5 OK] Result text: len=%d, preview=%.100s\n", len(resultText), resultText)
	log.Info().Str("step", "STEP 7.5").Int("result_text_len", len(resultText)).Msg("Result text formatted")

	fmt.Fprintf(os.Stderr, "[STEP 7.6] Creating response object\n")
	log.Info().Str("step", "STEP 7.6").Msg("Creating response object")
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
	fmt.Fprintf(os.Stderr, "[STEP 7.6 OK] Response created\n")
	log.Info().Str("step", "STEP 7.6").Msg("Response created")

	fmt.Fprintf(os.Stderr, "[STEP 7.7] Writing JSON response\n")
	log.Info().Str("step", "STEP 7.7").Msg("Writing JSON response")
	log.Debug().Interface("response", response).Msg("Sending tool call response")
	err = m.writeJSON(response)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[STEP 7.7 ERROR] writeJSON failed: %v\n", err)
		log.Error().Str("step", "STEP 7.7").Err(err).Msg("writeJSON failed")
	} else {
		fmt.Fprintf(os.Stderr, "[STEP 7.7 OK] writeJSON completed\n")
		log.Info().Str("step", "STEP 7.7").Msg("writeJSON completed")
	}
	return err
}

// Helper methods to convert tool arguments to handler params and call handlers
// These methods will convert the generic map[string]interface{} to specific handler params

func (m *MCPProtocol) handleGetEventLogTool(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	fmt.Fprintf(os.Stderr, "[STEP 8] handleGetEventLogTool: ENTRY - args_count=%d\n", len(args))
	log.Info().Str("step", "STEP 8").Int("args_count", len(args)).Msg("handleGetEventLogTool: ENTRY")

	params := handlers.EventLogParams{}

	fmt.Fprintf(os.Stderr, "[STEP 8.1] Extracting cluster_guid\n")
	log.Info().Str("step", "STEP 8.1").Msg("Extracting cluster_guid")
	if v, ok := args["cluster_guid"].(string); ok {
		params.ClusterGUID = v
		fmt.Fprintf(os.Stderr, "[STEP 8.1 OK] cluster_guid=%s\n", v)
		log.Info().Str("step", "STEP 8.1").Str("cluster_guid", v).Msg("cluster_guid extracted")
	} else {
		fmt.Fprintf(os.Stderr, "[STEP 8.1 WARN] cluster_guid not found or not string\n")
		log.Warn().Str("step", "STEP 8.1").Msg("cluster_guid not found or not string")
	}

	fmt.Fprintf(os.Stderr, "[STEP 8.2] Extracting infobase_guid\n")
	log.Info().Str("step", "STEP 8.2").Msg("Extracting infobase_guid")
	if v, ok := args["infobase_guid"].(string); ok {
		params.InfobaseGUID = v
		fmt.Fprintf(os.Stderr, "[STEP 8.2 OK] infobase_guid=%s\n", v)
		log.Info().Str("step", "STEP 8.2").Str("infobase_guid", v).Msg("infobase_guid extracted")
	} else {
		fmt.Fprintf(os.Stderr, "[STEP 8.2 WARN] infobase_guid not found or not string\n")
		log.Warn().Str("step", "STEP 8.2").Msg("infobase_guid not found or not string")
	}

	// Set default mode to "minimal" if not specified
	if v, ok := args["mode"].(string); ok && v != "" {
		params.Mode = v
	} else {
		params.Mode = "minimal"
	}

	// Set default level to "Error" if not specified
	// Note: English values (Error, Warning, Information, Note) are automatically
	// converted to Russian (Ошибка, Предупреждение, Информация, Примечание) in the handler
	if v, ok := args["level"].(string); ok && v != "" {
		params.Level = v
	} else {
		params.Level = "Error"
	}

	// Set default time range to last 10 minutes if not specified
	now := time.Now()
	if v, ok := args["from"].(string); ok && v != "" {
		// Try RFC3339 first, then try without timezone (assume UTC)
		var t time.Time
		var err error
		if t, err = time.Parse(time.RFC3339, v); err != nil {
			// Try parsing without timezone (assume UTC)
			if t, err = time.Parse("2006-01-02T15:04:05", v); err == nil {
				t = t.UTC()
			}
		}
		if err == nil {
			params.From = t
			fmt.Fprintf(os.Stderr, "[STEP 8.3] from parsed from args: %v\n", params.From)
			log.Info().Str("step", "STEP 8.3").Time("from", params.From).Msg("from parsed from args")
		} else {
			// If parsing failed, use default
			params.From = now.Add(-10 * time.Minute)
			fmt.Fprintf(os.Stderr, "[STEP 8.3 WARN] from parsing failed, using default: %v (error: %v)\n", params.From, err)
			log.Warn().Str("step", "STEP 8.3").Time("from", params.From).Err(err).Str("input", v).Msg("from parsing failed, using default")
		}
	} else {
		params.From = now.Add(-10 * time.Minute)
		fmt.Fprintf(os.Stderr, "[STEP 8.3] from not specified, using default (last 10 minutes): %v\n", params.From)
		log.Info().Str("step", "STEP 8.3").Time("from", params.From).Msg("from not specified, using default (last 10 minutes)")
	}

	if v, ok := args["to"].(string); ok && v != "" {
		// Try RFC3339 first, then try without timezone (assume UTC)
		var t time.Time
		var err error
		if t, err = time.Parse(time.RFC3339, v); err != nil {
			// Try parsing without timezone (assume UTC)
			if t, err = time.Parse("2006-01-02T15:04:05", v); err == nil {
				t = t.UTC()
			}
		}
		if err == nil {
			params.To = t
			fmt.Fprintf(os.Stderr, "[STEP 8.3] to parsed from args: %v\n", params.To)
			log.Info().Str("step", "STEP 8.3").Time("to", params.To).Msg("to parsed from args")
		} else {
			// If parsing failed, use default
			params.To = now
			fmt.Fprintf(os.Stderr, "[STEP 8.3 WARN] to parsing failed, using default: %v (error: %v)\n", params.To, err)
			log.Warn().Str("step", "STEP 8.3").Time("to", params.To).Err(err).Str("input", v).Msg("to parsing failed, using default")
		}
	} else {
		params.To = now
		fmt.Fprintf(os.Stderr, "[STEP 8.3] to not specified, using default (now): %v\n", params.To)
		log.Info().Str("step", "STEP 8.3").Time("to", params.To).Msg("to not specified, using default (now)")
	}

	// Parse limit - can be int or float64 from JSON
	if v, ok := args["limit"].(float64); ok {
		params.Limit = int(v)
	} else if v, ok := args["limit"].(int); ok {
		params.Limit = v
	} else if v, ok := args["limit"].(int64); ok {
		params.Limit = int(v)
	}

	// Log parameters before calling handler
	fmt.Fprintf(os.Stderr, "[STEP 8.3] Final params: cluster=%s, infobase=%s, level=%s, mode=%s, from=%v, to=%v, limit=%d\n",
		params.ClusterGUID, params.InfobaseGUID, params.Level, params.Mode, params.From, params.To, params.Limit)
	log.Info().
		Str("step", "STEP 8.3").
		Str("cluster_guid", params.ClusterGUID).
		Str("infobase_guid", params.InfobaseGUID).
		Str("level", params.Level).
		Str("mode", params.Mode).
		Int("limit", params.Limit).
		Time("from", params.From).
		Time("to", params.To).
		Msg("Final params")
	log.Info().
		Str("cluster_guid", params.ClusterGUID).
		Str("infobase_guid", params.InfobaseGUID).
		Str("level", params.Level).
		Str("mode", params.Mode).
		Int("limit", params.Limit).
		Time("from", params.From).
		Time("to", params.To).
		Msg("Calling GetEventLog handler")

	fmt.Fprintf(os.Stderr, "[STEP 8.4] Calling GetEventLog handler...\n")
	log.Info().Str("step", "STEP 8.4").Msg("Calling GetEventLog handler")
	result, err := m.server.eventLogHandler.GetEventLog(ctx, params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[STEP 8.4 ERROR] GetEventLog returned error: %v\n", err)
		log.Error().Str("step", "STEP 8.4").Err(err).Msg("GetEventLog returned error")
		log.Error().Err(err).Msg("GetEventLog handler returned error")
		return nil, err
	}
	fmt.Fprintf(os.Stderr, "[STEP 8.4 OK] GetEventLog returned: len=%d, preview=%.100s\n", len(result), result)
	log.Info().
		Str("step", "STEP 8.4").
		Int("result_len", len(result)).
		Str("result_preview", func() string {
			if len(result) > 100 {
				return result[:100]
			}
			return result
		}()).
		Msg("GetEventLog returned successfully")

	// Log result preview
	if result != "" {
		previewLen := 200
		if len(result) < previewLen {
			previewLen = len(result)
		}
		log.Info().
			Int("result_length", len(result)).
			Str("result_preview", result[:previewLen]).
			Msg("GetEventLog handler returned result")
	} else {
		fmt.Fprintf(os.Stderr, "[STEP 8.4 WARN] GetEventLog returned empty string\n")
		log.Warn().Str("step", "STEP 8.4").Msg("GetEventLog returned empty string")
		log.Warn().Msg("GetEventLog handler returned empty string")
	}

	fmt.Fprintf(os.Stderr, "[STEP 8 OK] handleGetEventLogTool returning: len=%d\n", len(result))
	log.Info().Str("step", "STEP 8").Int("result_len", len(result)).Msg("handleGetEventLogTool returning")
	return result, nil
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
	fmt.Fprintf(os.Stderr, "[writeJSON] Marshaling response...\n")
	log.Info().Msg("writeJSON: Marshaling response")
	data, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[writeJSON ERROR] Failed to marshal JSON: %v\n", err)
		log.Error().Err(err).Msg("Failed to marshal JSON")
		return err
	}
	data = append(data, '\n')
	fmt.Fprintf(os.Stderr, "[writeJSON] Marshaled %d bytes, writing to stdout...\n", len(data))
	log.Info().Int("json_bytes", len(data)).Str("preview", string(data[:min(len(data), 200)])).Msg("writeJSON: Writing to stdout")

	// Write to stdout and flush to ensure data is sent immediately
	n, err := m.stdout.Write(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[writeJSON ERROR] Failed to write to stdout: %v (wrote %d bytes)\n", err, n)
		log.Error().Err(err).Int("bytes_written", n).Msg("Failed to write to stdout")
		return err
	}
	fmt.Fprintf(os.Stderr, "[writeJSON OK] Wrote %d bytes to stdout\n", n)
	log.Info().Int("bytes_written", n).Msg("writeJSON: Wrote to stdout")

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
