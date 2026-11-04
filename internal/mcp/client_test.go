package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// newTestServer stands up a real, spec-compliant MCP server - using the same
// SDK the client wraps - with one "get_stock_price" tool registered, so
// tests exercise the actual Streamable HTTP wire protocol (initialize
// handshake, session IDs, SSE framing) rather than a hand-rolled fake of it.
func newTestServer(t *testing.T, handler sdkmcp.ToolHandler) (*httptest.Server, *capturedRequests) {
	t.Helper()
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test-server", Version: "0.0.1"}, nil)
	server.AddTool(&sdkmcp.Tool{
		Name:        "get_stock_price",
		Description: "stock lookup",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, handler)
	mcpHandler := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server { return server }, nil)

	captured := &capturedRequests{}
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		captured.record(r, body)
		mcpHandler.ServeHTTP(w, r)
	})
	httpServer := httptest.NewServer(mux)
	t.Cleanup(httpServer.Close)
	return httpServer, captured
}

type recordedRequest struct {
	method        string // the JSON-RPC method, e.g. "tools/call" - empty for non-JSON-RPC requests (e.g. session DELETE)
	authorization string
	accept        string
	sessionID     string
}

type capturedRequests struct {
	mu   sync.Mutex
	reqs []recordedRequest
}

func (c *capturedRequests) record(r *http.Request, body []byte) {
	var rpc struct {
		Method string `json:"method"`
	}
	_ = json.Unmarshal(body, &rpc)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reqs = append(c.reqs, recordedRequest{
		method:        rpc.Method,
		authorization: r.Header.Get("Authorization"),
		accept:        r.Header.Get("Accept"),
		sessionID:     r.Header.Get("Mcp-Session-Id"),
	})
}

func (c *capturedRequests) find(method string) (recordedRequest, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, r := range c.reqs {
		if r.method == method {
			return r, true
		}
	}
	return recordedRequest{}, false
}

func failIfCalled(t *testing.T) sdkmcp.ToolHandler {
	return func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		t.Fatal("tool handler should not have been called")
		return nil, nil
	}
}

func TestListToolsReturnsToolsFromRealServer(t *testing.T) {
	server, _ := newTestServer(t, failIfCalled(t))

	client := NewClient(server.URL, "/mcp")
	tools, err := client.ListTools(context.Background(), "")
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "get_stock_price" {
		t.Fatalf("unexpected tools: %+v", tools)
	}

	// Verify InputSchema is populated from the server and contains valid JSON.
	if len(tools[0].InputSchema) == 0 {
		t.Fatal("InputSchema is empty, expected schema from server")
	}
	var schema map[string]any
	if err := json.Unmarshal(tools[0].InputSchema, &schema); err != nil {
		t.Fatalf("InputSchema not valid JSON: %v (raw: %s)", err, tools[0].InputSchema)
	}
	if schema["type"] != "object" {
		t.Fatalf("InputSchema type = %v, want object", schema["type"])
	}
}

func TestListToolsWithEmptyTokenSendsNoAuthorizationHeader(t *testing.T) {
	server, captured := newTestServer(t, failIfCalled(t))

	client := NewClient(server.URL, "/mcp")
	if _, err := client.ListTools(context.Background(), ""); err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	call, ok := captured.find("tools/list")
	if !ok {
		t.Fatal("expected a tools/list request to reach the server")
	}
	if call.authorization != "" {
		t.Fatalf("expected Authorization header to be absent with empty token, got %q", call.authorization)
	}
}

func TestCallToolReturnsContentAndSetsHeaders(t *testing.T) {
	server, captured := newTestServer(t, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "AAPL: $150"}}}, nil
	})

	client := NewClient(server.URL, "/mcp")
	result, err := client.CallTool(context.Background(), "test-token", "get_stock_price", map[string]any{"symbol": "AAPL"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if len(result.Content) != 1 || result.Content[0].Text != "AAPL: $150" {
		t.Fatalf("unexpected result: %+v", result)
	}

	call, ok := captured.find("tools/call")
	if !ok {
		t.Fatal("expected a tools/call request to reach the server")
	}
	if call.authorization != "Bearer test-token" {
		t.Fatalf("expected Authorization header to be set, got %q", call.authorization)
	}
	// The Streamable HTTP transport requires both values in Accept - a server
	// returns 406 otherwise, regardless of which one the response ends up using.
	if !strings.Contains(call.accept, "application/json") || !strings.Contains(call.accept, "text/event-stream") {
		t.Fatalf("expected Accept to list both response types, got %q", call.accept)
	}
}

// TestCallToolSendsTheSessionIDFromInitialize is the regression test for the
// bug found against the real stock-server-mcp: agentgateway-demo-wizard was
// calling tools/call without the Mcp-Session-Id the server assigned on
// initialize, and the server correctly rejected it. The official SDK handles
// session propagation internally - this confirms our wrapper doesn't bypass it.
func TestCallToolSendsTheSessionIDFromInitialize(t *testing.T) {
	server, captured := newTestServer(t, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "ok"}}}, nil
	})

	client := NewClient(server.URL, "/mcp")
	if _, err := client.CallTool(context.Background(), "test-token", "get_stock_price", nil); err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	call, ok := captured.find("tools/call")
	if !ok {
		t.Fatal("expected a tools/call request to reach the server")
	}
	if call.sessionID == "" {
		t.Fatal("expected the session ID the server assigned on initialize to be sent on tools/call")
	}
}

func TestCallToolSurfacesToolError(t *testing.T) {
	server, _ := newTestServer(t, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		result := &sdkmcp.CallToolResult{}
		result.SetError(fmt.Errorf("tool not allowed for this identity"))
		return result, nil
	})

	client := NewClient(server.URL, "/mcp")
	if _, err := client.CallTool(context.Background(), "test-token", "get_stock_price", nil); err == nil {
		t.Fatal("expected an error from the tool's IsError result")
	} else if !strings.Contains(err.Error(), "tool not allowed") {
		t.Fatalf("expected the tool's error message, got: %v", err)
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
		t.Fatal("expected an error for an unauthenticated request")
	} else if !strings.Contains(err.Error(), "Unauthorized") {
		t.Fatalf("expected the 401 status in the error, got: %v", err)
	}
}

func TestToolInputSchemaIsJSON(t *testing.T) {
	// A Tool's InputSchema must be valid JSON (or empty), so the agent loop can
	// hand it to the model as OpenAI function "parameters" unchanged.
	tool := Tool{
		Name:        "get_balance",
		Description: "Look up an account balance",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"account":{"type":"string"}}}`),
	}
	var m map[string]any
	if err := json.Unmarshal(tool.InputSchema, &m); err != nil {
		t.Fatalf("InputSchema is not valid JSON: %v", err)
	}
	if m["type"] != "object" {
		t.Fatalf("expected object schema, got %v", m["type"])
	}
}
