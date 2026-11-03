// Package mcp speaks to an MCP server reached through agentgateway. It knows
// nothing about Keycloak or demo identities - callers acquire a bearer token
// themselves and pass it in. The wire protocol itself (the Streamable HTTP
// initialize handshake, session ID propagation, SSE-vs-JSON response
// parsing) is handled by the official github.com/modelcontextprotocol/go-sdk,
// not hand-rolled here.
package mcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Client calls an MCP server's tools/list and tools/call methods.
type Client struct {
	baseURL string
	path    string
}

// NewClient builds a Client. baseURL must include a scheme (e.g.
// "https://agentgateway.demo.example.com"); path is the MCP route, e.g. "/mcp".
func NewClient(baseURL, path string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), path: path}
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

// bearerTransport sets the Authorization header on every request a session
// makes. The token varies per call - a Runner acquires it per demo identity -
// so it cannot be baked into a single shared http.Client.
type bearerTransport struct {
	token string
}

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.token != "" {
		req.Header.Set("Authorization", "Bearer "+t.token)
	}
	return http.DefaultTransport.RoundTrip(req)
}

// connect opens a fresh MCP session scoped to one call's token. A new
// session per call, rather than one long-lived session reused across calls,
// is required because the identity the MCP server sees varies on every call.
func (c *Client) connect(ctx context.Context, token string) (*sdkmcp.ClientSession, error) {
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "agentgateway-demo-wizard", Version: "1.0"}, nil)
	transport := &sdkmcp.StreamableClientTransport{
		Endpoint: c.baseURL + c.path,
		HTTPClient: &http.Client{
			Timeout:   15 * time.Second,
			Transport: &bearerTransport{token: token},
		},
		// No server-initiated messages (sampling, elicitation, list-changed
		// notifications) are relevant to this one-shot, per-call session.
		DisableStandaloneSSE: true,
	}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connecting to MCP server: %w", err)
	}
	return session, nil
}

// ListTools calls tools/list. token may be empty for an unauthenticated call.
func (c *Client) ListTools(ctx context.Context, token string) ([]Tool, error) {
	session, err := c.connect(ctx, token)
	if err != nil {
		return nil, err
	}
	defer session.Close()

	result, err := session.ListTools(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("listing MCP tools: %w", err)
	}

	tools := make([]Tool, 0, len(result.Tools))
	for _, t := range result.Tools {
		tools = append(tools, Tool{Name: t.Name, Description: t.Description})
	}
	return tools, nil
}

// CallTool calls tools/call with the given tool name and arguments.
func (c *Client) CallTool(ctx context.Context, token, name string, arguments map[string]any) (*ToolResult, error) {
	session, err := c.connect(ctx, token)
	if err != nil {
		return nil, err
	}
	defer session.Close()

	if arguments == nil {
		arguments = map[string]any{}
	}
	result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		return nil, fmt.Errorf("calling MCP tool %q: %w", name, err)
	}

	blocks := make([]ContentBlock, 0, len(result.Content))
	var errText []string
	for _, content := range result.Content {
		text, ok := content.(*sdkmcp.TextContent)
		if !ok {
			continue
		}
		blocks = append(blocks, ContentBlock{Type: "text", Text: text.Text})
		if result.IsError {
			errText = append(errText, text.Text)
		}
	}
	// Per the MCP spec, a tool's own failure is reported as a successful
	// CallToolResult with IsError set, not a JSON-RPC-level error - only the
	// latter comes back as err above.
	if result.IsError {
		return nil, fmt.Errorf("MCP tool %q returned an error: %s", name, strings.Join(errText, "; "))
	}
	return &ToolResult{Content: blocks}, nil
}
