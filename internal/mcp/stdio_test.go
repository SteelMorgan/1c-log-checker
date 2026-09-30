package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestHandleInitializeDoesNotSendInitializedNotification(t *testing.T) {
	protocol := &MCPProtocol{
		stdout: &bytes.Buffer{},
	}

	req := &MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: json.RawMessage(`{
			"protocolVersion":"2025-06-18",
			"capabilities":{},
			"clientInfo":{"name":"codex","version":"test"}
		}`),
	}

	if err := protocol.handleRequest(context.Background(), req); err != nil {
		t.Fatalf("handleRequest returned error: %v", err)
	}

	got := protocol.stdout.(*bytes.Buffer).String()
	const want = "{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"protocolVersion\":\"2025-06-18\",\"capabilities\":{\"tools\":{}},\"serverInfo\":{\"name\":\"1c-log-checker\",\"version\":\"0.1.0\"}}}\n"
	if got != want {
		t.Fatalf("unexpected initialize response\nwant: %q\ngot:  %q", want, got)
	}
}

func TestInitializedNotificationDoesNotWriteResponse(t *testing.T) {
	buf := &bytes.Buffer{}
	protocol := &MCPProtocol{
		stdout: buf,
	}

	req := &MCPRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}

	if err := protocol.handleRequest(context.Background(), req); err != nil {
		t.Fatalf("handleRequest returned error: %v", err)
	}

	if got := buf.String(); got != "" {
		t.Fatalf("expected no response for notification, got %q", got)
	}
}

func TestPingReturnsEmptyResult(t *testing.T) {
	buf := &bytes.Buffer{}
	protocol := &MCPProtocol{
		stdout: buf,
	}

	req := &MCPRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "ping",
	}

	if err := protocol.handleRequest(context.Background(), req); err != nil {
		t.Fatalf("handleRequest returned error: %v", err)
	}

	const want = "{\"jsonrpc\":\"2.0\",\"id\":2,\"result\":{}}\n"
	if got := buf.String(); got != want {
		t.Fatalf("unexpected ping response\nwant: %q\ngot:  %q", want, got)
	}
}
