package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/day0ops/agentgateway-demo-wizard/internal/gateway"
	"github.com/day0ops/agentgateway-demo-wizard/internal/mcp"
)

func TestRunCallsToolThenReturnsFinalAnswer(t *testing.T) {
	var chatCallCount int

	mcpServer := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test-server", Version: "0.0.1"}, nil)
	mcpServer.AddTool(&sdkmcp.Tool{
		Name:        "get_stock_price",
		Description: "look up a stock price",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "AAPL: $150"}}}, nil
	})
	mcpHandler := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server { return mcpServer }, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("/openai/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		chatCallCount++
		w.Header().Set("Content-Type", "application/json")
		if chatCallCount == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_stock_price","arguments":"{\"symbol\":\"AAPL\"}"}}]}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"AAPL is trading at $150."}}]}`))
	})
	mux.Handle("/mcp", mcpHandler)
	server := httptest.NewServer(mux)
	defer server.Close()

	gatewayClient := gateway.NewClient("http://unused", server.URL, "test-password")
	mcpClient := mcp.NewClient(server.URL, "/mcp")
	runner := NewRunner(gatewayClient, mcpClient, "/openai/v1/chat/completions")

	hops := make(chan HopEvent, 16)
	answer, err := runner.Run(context.Background(), Request{
		Identity: "anonymous",
		Prompt:   "What is AAPL trading at?",
		Model:    "gpt-4o-mini",
		Tools: []ToolSpec{
			{
				Name:        "get_stock_price",
				Description: "look up a stock price",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"symbol":{"type":"string"}}}`),
			},
		},
	}, hops)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if answer != "AAPL is trading at $150." {
		t.Fatalf("unexpected answer: %q", answer)
	}

	var types []string
	for event := range hops {
		types = append(types, event.Type)
	}
	want := []string{"model_call", "tool_call", "tool_result", "model_call", "final"}
	if len(types) != len(want) {
		t.Fatalf("expected hop sequence %v, got %v", want, types)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("expected hop sequence %v, got %v", want, types)
		}
	}
}

// newFakeGateway returns a *gateway.Client backed by an httptest.Server that
// replies to every Proxy call with the next body in bodies, in order, always
// with status 200 - a stand-in for agentgateway's chat completions endpoint.
func newFakeGateway(t *testing.T, bodies ...string) *gateway.Client {
	t.Helper()
	var i int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if i >= len(bodies) {
			t.Fatalf("unexpected extra gateway call %d (only %d bodies queued)", i+1, len(bodies))
		}
		body := bodies[i]
		i++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return gateway.NewClient("http://unused", server.URL, "test-password")
}

// newFakeMCP returns an *mcp.Client backed by a real, spec-compliant MCP
// server (built with the same SDK the client wraps) exposing one
// "get_balance" tool whose call always returns resultJSON's content blocks.
func newFakeMCP(t *testing.T, resultJSON string) *mcp.Client {
	t.Helper()
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		t.Fatalf("newFakeMCP: invalid result JSON: %v", err)
	}

	mcpServer := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test-server", Version: "0.0.1"}, nil)
	mcpServer.AddTool(&sdkmcp.Tool{
		Name:        "get_balance",
		Description: "look up an account balance",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		content := make([]sdkmcp.Content, 0, len(result.Content))
		for _, c := range result.Content {
			content = append(content, &sdkmcp.TextContent{Text: c.Text})
		}
		return &sdkmcp.CallToolResult{Content: content}, nil
	})
	mcpHandler := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server { return mcpServer }, nil)

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return mcp.NewClient(server.URL, "/mcp")
}

// TestRunToolModeTalliesUsageAndCounts drives one tool call then a final
// answer, each model response carrying a usage block, and asserts
// RunToolMode discovers the tool live and tallies tokens, hops, and tool
// calls correctly.
func TestRunToolModeTalliesUsageAndCounts(t *testing.T) {
	// Model turn 1 asks for a tool; turn 2 answers. Each carries usage.
	gw := newFakeGateway(t,
		`{"model":"gpt-4o-mini","usage":{"prompt_tokens":100,"completion_tokens":20},`+
			`"choices":[{"message":{"role":"assistant","tool_calls":[`+
			`{"id":"c1","type":"function","function":{"name":"get_balance","arguments":"{}"}}]}}]}`,
		`{"model":"gpt-4o-mini","usage":{"prompt_tokens":60,"completion_tokens":8},`+
			`"choices":[{"message":{"role":"assistant","content":"Your balance is $10."}}]}`,
	)
	mcpClient := newFakeMCP(t, `{"content":[{"type":"text","text":"10"}]}`)
	r := NewRunner(gw, mcpClient, "/agent-chat")

	got, err := r.RunToolMode(context.Background(),
		Request{Identity: "anonymous", Prompt: "balance?", Model: "gpt-4o-mini"}, mcpClient)
	if err != nil {
		t.Fatalf("RunToolMode: %v", err)
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
}

func TestRunSurfacesModelErrorAsErrorHop(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/openai/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"budget exceeded"}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	gatewayClient := gateway.NewClient("http://unused", server.URL, "test-password")
	mcpClient := mcp.NewClient(server.URL, "/mcp")
	runner := NewRunner(gatewayClient, mcpClient, "/openai/v1/chat/completions")

	hops := make(chan HopEvent, 16)
	_, err := runner.Run(context.Background(), Request{Identity: "anonymous", Prompt: "hi", Model: "gpt-4o-mini"}, hops)
	if err == nil {
		t.Fatalf("expected an error")
	}

	var sawError bool
	for event := range hops {
		if event.Type == "error" {
			sawError = true
		}
	}
	if !sawError {
		t.Fatalf("expected an error hop event")
	}
}

// TestRunSurfacesMalformedToolArgumentsAsErrorHop ensures a tool call whose
// arguments aren't valid JSON fails loudly (an error hop plus a returned
// error) instead of silently proceeding with nil arguments.
func TestRunSurfacesMalformedToolArgumentsAsErrorHop(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/openai/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_stock_price","arguments":"not-json"}}]}}]}`))
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("expected no tools/call request for malformed arguments")
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	gatewayClient := gateway.NewClient("http://unused", server.URL, "test-password")
	mcpClient := mcp.NewClient(server.URL, "/mcp")
	runner := NewRunner(gatewayClient, mcpClient, "/openai/v1/chat/completions")

	hops := make(chan HopEvent, 16)
	_, err := runner.Run(context.Background(), Request{
		Identity: "anonymous",
		Prompt:   "What is AAPL trading at?",
		Model:    "gpt-4o-mini",
		Tools: []ToolSpec{
			{Name: "get_stock_price", Description: "look up a stock price"},
		},
	}, hops)
	if err == nil {
		t.Fatalf("expected an error")
	}

	var sawError bool
	for event := range hops {
		if event.Type == "error" {
			sawError = true
		}
	}
	if !sawError {
		t.Fatalf("expected an error hop event")
	}
}
