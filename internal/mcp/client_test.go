package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListToolsParsesToolNames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		if req.Method != "tools/list" {
			t.Fatalf("expected tools/list, got %q", req.Method)
		}
		_ = json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0", ID: 1,
			Result: json.RawMessage(`{"tools":[{"name":"get_stock_price","description":"stock lookup"}]}`),
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "/mcp")
	tools, err := client.ListTools(context.Background(), "")
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "get_stock_price" {
		t.Fatalf("unexpected tools: %+v", tools)
	}
}

func TestCallToolReturnsContent(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		var req jsonRPCRequest
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		if req.Method != "tools/call" {
			t.Fatalf("expected tools/call, got %q", req.Method)
		}
		_ = json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0", ID: 1,
			Result: json.RawMessage(`{"content":[{"type":"text","text":"AAPL: $150"}]}`),
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "/mcp")
	result, err := client.CallTool(context.Background(), "test-token", "get_stock_price", map[string]any{"symbol": "AAPL"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if len(result.Content) != 1 || result.Content[0].Text != "AAPL: $150" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("expected Authorization header to be set, got %q", gotAuth)
	}
}

func TestListToolsSurfacesUnauthenticatedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="/.well-known/oauth-protected-resource/mcp"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(server.URL, "/mcp")
	if _, err := client.ListTools(context.Background(), ""); err == nil {
		t.Fatalf("expected an error for an unauthenticated request")
	} else if !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected the 401 status in the error, got: %v", err)
	}
}

func TestCallToolSurfacesJSONRPCError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0", ID: 1,
			Error: &jsonRPCError{Code: -32000, Message: "tool not allowed for this identity"},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "/mcp")
	if _, err := client.CallTool(context.Background(), "test-token", "get_stock_price", nil); err == nil {
		t.Fatalf("expected an error from the JSON-RPC error field")
	} else if !strings.Contains(err.Error(), "tool not allowed") {
		t.Fatalf("expected the JSON-RPC error message, got: %v", err)
	}
}

func TestListToolsWithEmptyTokenSendsNoAuthorizationHeader(t *testing.T) {
	var capturedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0", ID: 1,
			Result: json.RawMessage(`{"tools":[]}`),
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "/mcp")
	if _, err := client.ListTools(context.Background(), ""); err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if capturedAuth != "" {
		t.Fatalf("expected Authorization header to be absent with empty token, got %q", capturedAuth)
	}
}
