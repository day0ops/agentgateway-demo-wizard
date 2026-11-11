// Package agent runs a model-plus-MCP-tools loop entirely server-side: it
// calls the model through agentgateway, executes any tool calls the model
// requests against an MCP server (also through agentgateway), feeds the
// results back, and repeats until the model gives a final answer. Each hop
// is emitted on a channel so the HTTP layer can stream it to the UI live.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/day0ops/agentgateway-demo-wizard/internal/gateway"
	"github.com/day0ops/agentgateway-demo-wizard/internal/mcp"
)

const maxHops = 5

// ErrModeUnavailable wraps a RunToolMode failure that happened while listing
// the mode's tools - most often because the gateway route backing that
// toolMode isn't provisioned yet. Callers distinguish it with errors.Is to
// show a clearer message than a mid-run model or tool-call failure.
var ErrModeUnavailable = errors.New("tool mode unavailable")

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
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// RunResult is the outcome of one agent run, including usage for the savings
// scoreboard.
type RunResult struct {
	Answer           string
	PromptTokens     int
	CompletionTokens int
	Hops             int // model turns taken
	ToolCalls        int // model-visible tool invocations
}

// Run executes the agent loop, emitting a HopEvent on hops for every model
// call, tool call, tool result, and the final answer (or an error). hops is
// always closed before Run returns, so callers can safely range over it.
func (r *Runner) Run(ctx context.Context, req Request, hops chan<- HopEvent) (string, error) {
	defer close(hops)
	tools := toOpenAITools(req.Tools)
	res, err := r.loop(ctx, req, tools, r.mcpClient, func(e HopEvent) { hops <- e })
	return res.Answer, err
}

// RunToolMode runs the loop against a specific MCP route (one toolMode),
// discovering the exposed tools live, and returns the usage tally. No
// streaming - the scoreboard only needs the final counts.
func (r *Runner) RunToolMode(ctx context.Context, req Request, mcpClient *mcp.Client) (RunResult, error) {
	token, err := r.mcpToken(ctx, req.Identity)
	if err != nil {
		return RunResult{}, err
	}
	listed, err := mcpClient.ListTools(ctx, token)
	if err != nil {
		return RunResult{}, fmt.Errorf("%w: listing tools for mode: %w", ErrModeUnavailable, err)
	}
	specs := make([]ToolSpec, 0, len(listed))
	for _, t := range listed {
		params := t.InputSchema
		if len(params) == 0 {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		specs = append(specs, ToolSpec{Name: t.Name, Description: t.Description, Parameters: params})
	}
	return r.loop(ctx, req, toOpenAITools(specs), mcpClient, nil)
}

// loop drives the model-plus-tools agent loop shared by Run and RunToolMode.
// emit may be nil, in which case no HopEvents are sent (RunToolMode's case -
// the scoreboard only needs the final tally).
func (r *Runner) loop(ctx context.Context, req Request, tools []openAITool, mcpClient *mcp.Client, emit func(HopEvent)) (RunResult, error) {
	send := func(e HopEvent) {
		if emit != nil {
			emit(e)
		}
	}
	var res RunResult

	mcpToken, err := r.mcpToken(ctx, req.Identity)
	if err != nil {
		send(HopEvent{Type: "error", Message: err.Error()})
		return res, err
	}

	messages := []chatMessage{{Role: "user", Content: req.Prompt}}
	for i := 0; i < maxHops; i++ {
		send(HopEvent{Type: "model_call", Message: fmt.Sprintf("Calling the model (hop %d)...", i+1)})
		reply, pt, ct, err := r.callModel(ctx, req.Model, req.Identity, messages, tools)
		if err != nil {
			send(HopEvent{Type: "error", Message: err.Error()})
			return res, err
		}
		res.Hops++
		res.PromptTokens += pt
		res.CompletionTokens += ct

		if len(reply.ToolCalls) == 0 {
			res.Answer = reply.Content
			send(HopEvent{Type: "final", Message: reply.Content})
			return res, nil
		}
		messages = append(messages, *reply)

		for _, call := range reply.ToolCalls {
			res.ToolCalls++
			var args map[string]any
			if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
				wrapped := fmt.Errorf("parsing arguments for tool %q: %w", call.Function.Name, err)
				send(HopEvent{Type: "error", Message: wrapped.Error()})
				return res, wrapped
			}
			send(HopEvent{Type: "tool_call", Message: fmt.Sprintf("Model requested tool %q", call.Function.Name), Detail: call.Function.Arguments})
			result, err := mcpClient.CallTool(ctx, mcpToken, call.Function.Name, args)
			if err != nil {
				send(HopEvent{Type: "error", Message: err.Error()})
				return res, err
			}
			resultText := resultToText(result)
			send(HopEvent{Type: "tool_result", Message: fmt.Sprintf("Tool %q returned a result", call.Function.Name), Detail: resultText})
			messages = append(messages, chatMessage{Role: "tool", ToolCallID: call.ID, Content: resultText})
		}
	}
	runErr := fmt.Errorf("exceeded %d hops without a final answer", maxHops)
	send(HopEvent{Type: "error", Message: runErr.Error()})
	return res, runErr
}

func toOpenAITools(specs []ToolSpec) []openAITool {
	tools := make([]openAITool, 0, len(specs))
	for _, t := range specs {
		tools = append(tools, openAITool{Type: "function", Function: openAIFunction(t)})
	}
	return tools
}

func (r *Runner) callModel(ctx context.Context, model, identity string, messages []chatMessage, tools []openAITool) (*chatMessage, int, int, error) {
	reqBody, err := json.Marshal(chatRequest{Model: model, Messages: messages, Tools: tools})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("encoding chat request: %w", err)
	}

	resp, err := r.gatewayClient.Proxy(ctx, gateway.Request{
		Identity: identity,
		Method:   http.MethodPost,
		Path:     r.chatPath,
		Body:     string(reqBody),
	})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("calling model: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, 0, 0, fmt.Errorf("model call returned status %d: %s", resp.StatusCode, resp.Body)
	}

	var parsed chatResponse
	if err := json.Unmarshal([]byte(resp.Body), &parsed); err != nil {
		return nil, 0, 0, fmt.Errorf("parsing chat response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, 0, 0, fmt.Errorf("model returned no choices")
	}
	return &parsed.Choices[0].Message, parsed.Usage.PromptTokens, parsed.Usage.CompletionTokens, nil
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
