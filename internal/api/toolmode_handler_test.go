package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/day0ops/agentgateway-demo-wizard/internal/agent"
	"github.com/day0ops/agentgateway-demo-wizard/internal/config"
	"github.com/day0ops/agentgateway-demo-wizard/internal/gateway"
	"github.com/day0ops/agentgateway-demo-wizard/internal/mcp"
	"github.com/day0ops/agentgateway-demo-wizard/internal/scenarios"
)

// TestToolModeRunRejectsUnknownMode asserts the handler resolves mode keys
// against the scenario registry and never lets an unrecognized key (or,
// transitively, a raw path) reach the gateway.
func TestToolModeRunRejectsUnknownMode(t *testing.T) {
	srv := NewServer(WithScenarios(scenarios.Registry()))
	req := httptest.NewRequest(http.MethodPost, "/api/toolmode/run", strings.NewReader(`{"mode":"nope","prompt":"hi"}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// TestToolModeRunRejectsInvalidBody asserts a malformed JSON body is a 400,
// not a 500 or an unhandled panic.
func TestToolModeRunRejectsInvalidBody(t *testing.T) {
	srv := NewServer(WithScenarios(scenarios.Registry()))
	req := httptest.NewRequest(http.MethodPost, "/api/toolmode/run", strings.NewReader(`not json`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// TestToolModeRunSucceeds drives the handler's real success path end to end -
// mode resolution, the mcpClientFactory seam, agent.Runner.RunToolMode against
// a real (fake-backed) MCP server, and the RunResult->JSON mapping - rather
// than only the validation-failure paths above.
func TestToolModeRunSucceeds(t *testing.T) {
	mcpServer := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test-server", Version: "0.0.1"}, nil)
	mcpServer.AddTool(&sdkmcp.Tool{
		Name:        "get_balance",
		Description: "look up an account balance",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "$10"}}}, nil
	})
	mcpHandler := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server { return mcpServer }, nil)

	var chatCalls int
	mux := http.NewServeMux()
	mux.HandleFunc("/agent-chat", func(w http.ResponseWriter, r *http.Request) {
		chatCalls++
		w.Header().Set("Content-Type", "application/json")
		if chatCalls == 1 {
			_, _ = w.Write([]byte(`{"usage":{"prompt_tokens":100,"completion_tokens":20},"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"name":"get_balance","arguments":"{}"}}]}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"usage":{"prompt_tokens":60,"completion_tokens":8},"choices":[{"message":{"role":"assistant","content":"Your balance is $10."}}]}`))
	})
	mux.Handle("/mcp", mcpHandler)
	server := httptest.NewServer(mux)
	defer server.Close()

	gatewayClient := gateway.NewClient("http://unused", server.URL, "test-password")
	mcpClient := mcp.NewClient(server.URL, "/mcp")
	runner := agent.NewRunner(gatewayClient, mcpClient, "/agent-chat")

	srv := NewServer(
		WithScenarios(scenarios.Registry()),
		WithConfig(&config.Config{GatewayHost: "unused"}),
		WithAgent(runner, nil),
	)
	srv.mcpClientFactory = func(path string) *mcp.Client { return mcpClient }

	req := httptest.NewRequest(http.MethodPost, "/api/toolmode/run", strings.NewReader(`{"mode":"standard","prompt":"balance?"}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got toolModeRunResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if got.Answer != "Your balance is $10." {
		t.Fatalf("answer = %q", got.Answer)
	}
	if got.PromptTokens != 160 || got.CompletionTokens != 28 {
		t.Fatalf("tokens = %d/%d, want 160/28", got.PromptTokens, got.CompletionTokens)
	}
	if got.Hops != 2 || got.ToolCalls != 1 {
		t.Fatalf("hops/toolCalls = %d/%d, want 2/1", got.Hops, got.ToolCalls)
	}
	if len(got.ToolsExposed) != 1 || got.ToolsExposed[0] != "get_balance" {
		t.Fatalf("toolsExposed = %v, want [get_balance]", got.ToolsExposed)
	}
	if len(got.ToolInvocations) != 1 || got.ToolInvocations[0].Name != "get_balance" || got.ToolInvocations[0].Arguments != "{}" {
		t.Fatalf("toolInvocations = %+v, want one get_balance call with arguments \"{}\"", got.ToolInvocations)
	}
}

// TestToolModeRunReturnsFriendlyMessageWhenModeUnavailable asserts a mode
// whose MCP route can't be listed (standing in for a not-yet-provisioned
// field-kit toolMode usecase) gets a clear, actionable message instead of a
// raw wrapped connection error - and that the body is the package's usual
// {"error": "..."} JSON shape, not plain text.
func TestToolModeRunReturnsFriendlyMessageWhenModeUnavailable(t *testing.T) {
	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer badServer.Close()

	gatewayClient := gateway.NewClient("http://unused", "http://unused", "test-password")
	mcpClient := mcp.NewClient(badServer.URL, "/not-a-real-route")
	runner := agent.NewRunner(gatewayClient, mcpClient, "/agent-chat")

	srv := NewServer(
		WithScenarios(scenarios.Registry()),
		WithConfig(&config.Config{GatewayHost: "unused"}),
		WithAgent(runner, nil),
	)
	srv.mcpClientFactory = func(path string) *mcp.Client { return mcpClient }

	req := httptest.NewRequest(http.MethodPost, "/api/toolmode/run", strings.NewReader(`{"mode":"standard","prompt":"hi"}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding error body: %v (body: %s)", err, rec.Body.String())
	}
	if !strings.Contains(body["error"], "toolMode usecase") {
		t.Fatalf("expected a friendly message mentioning the toolMode usecase, got: %q", body["error"])
	}
}
