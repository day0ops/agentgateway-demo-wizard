package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/day0ops/agentgateway-demo-wizard/internal/gateway"
	"github.com/day0ops/agentgateway-demo-wizard/internal/mcp"
)

func TestRunCallsToolThenReturnsFinalAnswer(t *testing.T) {
	var chatCallCount int

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
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "tools/call") {
			t.Fatalf("expected a tools/call request, got: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"AAPL: $150"}]}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	gatewayClient := gateway.NewClient("http://unused", server.URL)
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

func TestRunSurfacesModelErrorAsErrorHop(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/openai/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"budget exceeded"}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	gatewayClient := gateway.NewClient("http://unused", server.URL)
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

	gatewayClient := gateway.NewClient("http://unused", server.URL)
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
