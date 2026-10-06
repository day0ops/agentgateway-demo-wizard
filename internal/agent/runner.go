// Package agent runs a model-plus-MCP-tools loop entirely server-side: it
// calls the model through agentgateway, executes any tool calls the model
// requests against an MCP server (also through agentgateway), feeds the
// results back, and repeats until the model gives a final answer. Each hop
// is emitted on a channel so the HTTP layer can stream it to the UI live.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/day0ops/agentgateway-demo-wizard/internal/gateway"
	"github.com/day0ops/agentgateway-demo-wizard/internal/mcp"
)

const maxHops = 5

// ToolSpec is one tool the model is allowed to call, described in OpenAI's
// function-calling schema. The wizard's demo content supplies a fixed list -
// the agent does not discover tools dynamically from the MCP server.
type ToolSpec struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

// HopEvent is one step of the agent loop, emitted for live display.
type HopEvent struct {
	Type    string `json:"type"` // "model_call" | "tool_call" | "tool_result" | "final" | "error"
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// Request is one agent run.
type Request struct {
	Identity string
	Prompt   string
	Model    string
	Tools    []ToolSpec
}

// Runner drives the agent loop against a specific gateway chat path.
type Runner struct {
	gatewayClient *gateway.Client
	mcpClient     *mcp.Client
	chatPath      string
}

// NewRunner builds a Runner. chatPath is the OpenAI-compatible chat
// completions path the gateway exposes, e.g. "/agent-chat".
func NewRunner(gatewayClient *gateway.Client, mcpClient *mcp.Client, chatPath string) *Runner {
	return &Runner{gatewayClient: gatewayClient, mcpClient: mcpClient, chatPath: chatPath}
}

type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function toolCallFunc `json:"function"`
}

type toolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAITool struct {
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

type openAIFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Tools    []openAITool  `json:"tools,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// Run executes the agent loop, emitting a HopEvent on hops for every model
// call, tool call, tool result, and the final answer (or an error). hops is
// always closed before Run returns, so callers can safely range over it.
func (r *Runner) Run(ctx context.Context, req Request, hops chan<- HopEvent) (string, error) {
	defer close(hops)

	mcpToken, err := r.mcpToken(ctx, req.Identity)
	if err != nil {
		hops <- HopEvent{Type: "error", Message: err.Error()}
		return "", err
	}

	messages := []chatMessage{{Role: "user", Content: req.Prompt}}
	tools := make([]openAITool, 0, len(req.Tools))
	for _, t := range req.Tools {
		tools = append(tools, openAITool{
			Type:     "function",
			Function: openAIFunction(t),
		})
	}

	for i := 0; i < maxHops; i++ {
		hops <- HopEvent{Type: "model_call", Message: fmt.Sprintf("Calling the model (hop %d)...", i+1)}

		reply, err := r.callModel(ctx, req.Model, req.Identity, messages, tools)
		if err != nil {
			hops <- HopEvent{Type: "error", Message: err.Error()}
			return "", err
		}

		if len(reply.ToolCalls) == 0 {
			hops <- HopEvent{Type: "final", Message: reply.Content}
			return reply.Content, nil
		}

		messages = append(messages, *reply)

		for _, call := range reply.ToolCalls {
			var args map[string]any
			if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
				wrapped := fmt.Errorf("parsing arguments for tool %q: %w", call.Function.Name, err)
				hops <- HopEvent{Type: "error", Message: wrapped.Error()}
				return "", wrapped
			}

			hops <- HopEvent{
				Type:    "tool_call",
				Message: fmt.Sprintf("Model requested tool %q", call.Function.Name),
				Detail:  call.Function.Arguments,
			}

			result, err := r.mcpClient.CallTool(ctx, mcpToken, call.Function.Name, args)
			if err != nil {
				hops <- HopEvent{Type: "error", Message: err.Error()}
				return "", err
			}

			resultText := resultToText(result)
			hops <- HopEvent{
				Type:    "tool_result",
				Message: fmt.Sprintf("Tool %q returned a result", call.Function.Name),
				Detail:  resultText,
			}

			messages = append(messages, chatMessage{Role: "tool", ToolCallID: call.ID, Content: resultText})
		}
	}

	runErr := fmt.Errorf("exceeded %d hops without a final answer", maxHops)
	hops <- HopEvent{Type: "error", Message: runErr.Error()}
	return "", runErr
}

func (r *Runner) callModel(ctx context.Context, model, identity string, messages []chatMessage, tools []openAITool) (*chatMessage, error) {
	reqBody, err := json.Marshal(chatRequest{Model: model, Messages: messages, Tools: tools})
	if err != nil {
		return nil, fmt.Errorf("encoding chat request: %w", err)
	}

	resp, err := r.gatewayClient.Proxy(ctx, gateway.Request{
		Identity: identity,
		Method:   http.MethodPost,
		Path:     r.chatPath,
		Body:     string(reqBody),
	})
	if err != nil {
		return nil, fmt.Errorf("calling model: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("model call returned status %d: %s", resp.StatusCode, resp.Body)
	}

	var parsed chatResponse
	if err := json.Unmarshal([]byte(resp.Body), &parsed); err != nil {
		return nil, fmt.Errorf("parsing chat response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("model returned no choices")
	}
	return &parsed.Choices[0].Message, nil
}

func (r *Runner) mcpToken(ctx context.Context, identity string) (string, error) {
	if identity == "" || identity == "anonymous" {
		return "", nil
	}
	return r.gatewayClient.Token(ctx, identity)
}

func resultToText(result *mcp.ToolResult) string {
	parts := make([]string, 0, len(result.Content))
	for _, c := range result.Content {
		parts = append(parts, c.Text)
	}
	return strings.Join(parts, "\n")
}
