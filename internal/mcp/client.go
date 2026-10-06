// Package mcp speaks the MCP JSON-RPC-over-HTTP wire protocol to an MCP
// server reached through agentgateway. It knows nothing about Keycloak or
// demo identities - callers acquire a bearer token themselves and pass it in.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client calls an MCP server's tools/list and tools/call methods.
type Client struct {
	httpClient *http.Client
	baseURL    string
	path       string
}

// NewClient builds a Client. baseURL must include a scheme (e.g.
// "https://agentgateway.demo.example.com"); path is the MCP route, e.g. "/mcp".
func NewClient(baseURL, path string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		baseURL:    strings.TrimRight(baseURL, "/"),
		path:       path,
	}
}

// Tool is one tool an MCP server exposes.
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ToolResult is what calling a tool returned.
type ToolResult struct {
	Content []ContentBlock `json:"content"`
}

// ContentBlock is one piece of a ToolResult.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (c *Client) call(ctx context.Context, token, method string, params any, out any) error {
	reqBody, err := json.Marshal(jsonRPCRequest{JSONRPC: "2.0", ID: 1, Method: method, Params: params})
	if err != nil {
		return fmt.Errorf("encoding MCP request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+c.path, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("building MCP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("calling MCP server: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading MCP response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MCP server returned status %d: %s", resp.StatusCode, body)
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return fmt.Errorf("parsing MCP response: %w", err)
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("MCP error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	if out != nil {
		if err := json.Unmarshal(rpcResp.Result, out); err != nil {
			return fmt.Errorf("parsing MCP result: %w", err)
		}
	}
	return nil
}

// ListTools calls tools/list. token may be empty for an unauthenticated call.
func (c *Client) ListTools(ctx context.Context, token string) ([]Tool, error) {
	var result struct {
		Tools []Tool `json:"tools"`
	}
	if err := c.call(ctx, token, "tools/list", nil, &result); err != nil {
		return nil, err
	}
	return result.Tools, nil
}

// CallTool calls tools/call with the given tool name and arguments.
func (c *Client) CallTool(ctx context.Context, token, name string, arguments map[string]any) (*ToolResult, error) {
	if arguments == nil {
		arguments = map[string]any{}
	}
	var result ToolResult
	params := map[string]any{"name": name, "arguments": arguments}
	if err := c.call(ctx, token, "tools/call", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
