package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/day0ops/agentgateway-demo-wizard/internal/gateway"
)

func TestRequestHandlerProxiesToGateway(t *testing.T) {
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"chatcmpl-1"}`))
	}))
	defer gatewayServer.Close()

	client := gateway.NewClient("http://unused", gatewayServer.URL, "test-password")
	s := NewServer(WithGateway(client))

	reqBody := `{"identity":"anonymous","method":"POST","path":"/openai","body":"{}"}`
	req := httptest.NewRequest(http.MethodPost, "/api/request", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"statusCode":200`) {
		t.Fatalf("expected statusCode 200 in response, got: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "chatcmpl-1") {
		t.Fatalf("expected the raw gateway body to pass through, got: %s", rec.Body.String())
	}
}

func TestRequestHandlerStreamsWhenBodyRequestsStreaming(t *testing.T) {
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: hello\n\n"))
		flusher.Flush()
	}))
	defer gatewayServer.Close()

	client := gateway.NewClient("http://unused", gatewayServer.URL, "test-password")
	s := NewServer(WithGateway(client))

	reqBody := `{"identity":"anonymous","method":"POST","path":"/openai","body":"{\"stream\":true}"}`
	req := httptest.NewRequest(http.MethodPost, "/api/request", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "data: hello") {
		t.Fatalf("expected the streamed SSE body to pass through, got: %s", rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("expected text/event-stream content type, got %q", rec.Header().Get("Content-Type"))
	}
}

func TestRequestHandlerSurfacesTransportErrors(t *testing.T) {
	client := gateway.NewClient("http://unused", "http://127.0.0.1:1", "test-password")
	s := NewServer(WithGateway(client))

	reqBody := `{"identity":"anonymous","method":"POST","path":"/openai","body":"{}"}`
	req := httptest.NewRequest(http.MethodPost, "/api/request", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestRequestHandlerPerformsMCPHandshakeForMCPPaths is the regression test
// for a live bug: a preset's bare JSON-RPC body hitting a path ending in
// /mcp used to be forwarded as a single one-shot request with no session,
// which agentgateway's Streamable HTTP transport rejects. Paths ending in
// /mcp now go through the gateway client's handshake-aware ProxyMCP.
func TestRequestHandlerPerformsMCPHandshakeForMCPPaths(t *testing.T) {
	var sawMethods []string

	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var decoded struct {
			Method string `json:"method"`
		}
		_ = json.Unmarshal(body, &decoded)
		sawMethods = append(sawMethods, decoded.Method)

		switch decoded.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "test-session-789")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`))
		}
	}))
	defer gatewayServer.Close()

	client := gateway.NewClient("http://unused", gatewayServer.URL, "test-password")
	s := NewServer(WithGateway(client))

	reqBody := `{"identity":"anonymous","method":"POST","path":"/mcp","body":"{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/list\"}"}`
	req := httptest.NewRequest(http.MethodPost, "/api/request", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(sawMethods) != 3 || sawMethods[0] != "initialize" || sawMethods[1] != "notifications/initialized" || sawMethods[2] != "tools/list" {
		t.Fatalf("expected a full MCP handshake before tools/list, got: %v", sawMethods)
	}
}
