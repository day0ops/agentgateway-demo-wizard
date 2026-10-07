package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/day0ops/agentgateway-demo-wizard/internal/agent"
	"github.com/day0ops/agentgateway-demo-wizard/internal/gateway"
	"github.com/day0ops/agentgateway-demo-wizard/internal/mcp"
)

func TestAgentRunHandlerStreamsHops(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/openai/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"done"}}]}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	gatewayClient := gateway.NewClient("http://unused", server.URL, "test-password")
	mcpClient := mcp.NewClient(server.URL, "/mcp")
	runner := agent.NewRunner(gatewayClient, mcpClient, "/openai/v1/chat/completions")

	s := NewServer(WithAgent(runner, nil))

	reqBody := `{"identity":"anonymous","prompt":"hi","model":"gpt-4o-mini"}`
	req := httptest.NewRequest(http.MethodPost, "/api/agent/run", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"type":"final"`) {
		t.Fatalf("expected a final event, got: %s", rec.Body.String())
	}
}
