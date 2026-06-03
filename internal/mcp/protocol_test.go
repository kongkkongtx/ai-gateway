package mcp

import (
	"encoding/json"
	"testing"
)

func TestNewSuccessResponse(t *testing.T) {
	resp := NewSuccessResponse("req-1", map[string]string{"status": "ok"})
	if resp.JSONRPC != JSONRPCVersion {
		t.Fatalf("expected %s, got %s", JSONRPCVersion, resp.JSONRPC)
	}
	if resp.ID != "req-1" {
		t.Fatalf("expected req-1, got %v", resp.ID)
	}
}

func TestNewErrorResponse(t *testing.T) {
	resp := NewErrorResponse("req-1", ErrCodeMethodNotFound, "Method not found", nil)
	if resp.JSONRPC != JSONRPCVersion {
		t.Fatalf("expected %s, got %s", JSONRPCVersion, resp.JSONRPC)
	}
	if resp.Error.Code != ErrCodeMethodNotFound {
		t.Fatalf("expected code %d, got %d", ErrCodeMethodNotFound, resp.Error.Code)
	}
	if resp.Error.Message != "Method not found" {
		t.Fatalf("expected 'Method not found', got %s", resp.Error.Message)
	}
}

func TestNewToolCallResult(t *testing.T) {
	result := NewToolCallResult("hello world")
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(result.Content))
	}
	if result.Content[0].Type != "text" {
		t.Fatalf("expected 'text' type, got %s", result.Content[0].Type)
	}
	if result.Content[0].Text != "hello world" {
		t.Fatalf("expected 'hello world', got %s", result.Content[0].Text)
	}
	if result.IsError {
		t.Fatal("expected isError=false")
	}
}

func TestNewToolCallError(t *testing.T) {
	result := NewToolCallError("something went wrong")
	if !result.IsError {
		t.Fatal("expected isError=true")
	}
}

func TestJSONRPCRequestMarshal(t *testing.T) {
	req := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      "1",
		Method:  MethodToolsList,
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	if result["method"] != "tools/list" {
		t.Fatalf("expected tools/list, got %v", result["method"])
	}
	if result["id"] != "1" {
		t.Fatalf("expected id=1, got %v", result["id"])
	}
}

func TestJSONRPCRequestUnmarshal(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"test","arguments":{"key":"value"}}}`)
	var req JSONRPCRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if req.Method != MethodToolsCall {
		t.Fatalf("expected tools/call, got %s", req.Method)
	}
	if req.ID.(float64) != 2 {
		t.Fatalf("expected id=2, got %v", req.ID)
	}

	var params ToolsCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		t.Fatalf("params unmarshal error: %v", err)
	}
	if params.Name != "test" {
		t.Fatalf("expected name=test, got %s", params.Name)
	}
}

func TestToolDefSerialization(t *testing.T) {
	tool := ToolDef{
		Name:        "get_weather",
		Description: "Get current weather",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}
	data, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded ToolDef
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Name != "get_weather" {
		t.Fatalf("expected get_weather, got %s", decoded.Name)
	}
}

func TestInitializeResult(t *testing.T) {
	result := InitializeResult{
		ProtocolVersion: "2025-06-18",
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{ListChanged: true},
		},
		ServerInfo: ServerInfo{
			Name:    "ai-gateway",
			Version: "3.0.0",
		},
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded InitializeResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.ServerInfo.Name != "ai-gateway" {
		t.Fatalf("expected ai-gateway, got %s", decoded.ServerInfo.Name)
	}
	if decoded.Capabilities.Tools == nil || !decoded.Capabilities.Tools.ListChanged {
		t.Fatal("expected Tools capability with ListChanged=true")
	}
}

func TestJSONRPCErrorResponseMarshal(t *testing.T) {
	errResp := NewErrorResponse("req-1", ErrCodeInvalidParams, "Invalid params", "missing field")
	data, err := json.Marshal(errResp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	if result["jsonrpc"] != "2.0" {
		t.Fatalf("expected 2.0, got %v", result["jsonrpc"])
	}
}
