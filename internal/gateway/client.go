// Package gateway acquires Keycloak tokens for the wizard's fixed demo
// identities and proxies requests to agentgateway, returning the raw
// response as-is - a 401 or 429 is frequently the point of a demo step, not
// a failure to hide.
package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const keycloakRealm = "agw-dev"
const keycloakClientID = "agw-client-public"

// requestTimeout bounds every call this client makes, including live,
// non-streaming LLM completions through Keycloak, agentgateway, and the
// upstream provider - those can legitimately take tens of seconds since the
// full completion must be generated before the response headers go out.
const requestTimeout = 60 * time.Second

// demoUsers maps the wizard's fixed identities to the Keycloak usernames
// seeded in the agw-dev realm by agentgateway-field-kit's keycloak addon
// (config.realms[].users in a profile like
// config/profiles/eks-enterprise-agentgateway-complete.yaml) - "user1" and
// "user2" carry the team_id attributes "team-alpha"/"team-beta"
// respectively. Every seeded user shares one realm-wide default password
// (field-kit's realm.defaultPassword), supplied here via Client.demoUserPassword
// rather than hardcoded. "anonymous" and "" are handled separately - they
// skip token acquisition entirely.
var demoUsers = map[string]string{
	"team-alpha": "user1",
	"team-beta":  "user2",
}

// tenantIdentities maps the multi-tenant isolation pillar's fixed tenant
// identities to the Keycloak org realm and client-credentials client ID
// seeded for each by field-kit's keycloak addon (config.realms[].clients).
// Unlike demoUsers, these authenticate via client-credentials grant rather
// than a real end-user login, so there is no per-tenant username/password -
// see Client.tenantClientSecrets for the per-tenant secret.
var tenantIdentities = map[string]struct{ realm, clientID string }{
	"org-acme":    {"org-acme", "acme-team-eng"},
	"org-globex":  {"org-globex", "globex-team-sales"},
	"org-initech": {"org-initech", "initech-team-dev"},
}

// Client acquires tokens and proxies requests to a live agentgateway/Keycloak pair.
type Client struct {
	httpClient            *http.Client
	keycloakBaseURL       string
	gatewayBaseURL        string
	demoUserPassword      string
	tenantsGatewayBaseURL string
	tenantClientSecrets   map[string]string
}

// Option configures optional Client behavior not covered by NewClient's
// required parameters.
type Option func(*Client)

// WithTenantsGatewayBaseURL configures the base URL ProxyTenant forwards
// requests to - a separate, dedicated Gateway from the shared gatewayBaseURL.
func WithTenantsGatewayBaseURL(url string) Option {
	return func(c *Client) {
		c.tenantsGatewayBaseURL = strings.TrimRight(url, "/")
	}
}

// WithTenantClientSecrets configures the client-credentials secret for each
// tenant identity. Keys are tenant identities (e.g. "org-acme") matching
// tenantIdentities; a tenant with no entry here cannot acquire a token via
// tenantToken.
func WithTenantClientSecrets(secrets map[string]string) Option {
	return func(c *Client) {
		c.tenantClientSecrets = secrets
	}
}

// NewClient builds a Client. keycloakBaseURL and gatewayBaseURL must each
// include a scheme (e.g. "https://keycloak.demo.example.com").
// demoUserPassword is the shared password for every demoUsers entry. opts
// configure the optional tenant auth path used by ProxyTenant.
func NewClient(keycloakBaseURL, gatewayBaseURL, demoUserPassword string, opts ...Option) *Client {
	c := &Client{
		httpClient:       &http.Client{Timeout: requestTimeout},
		keycloakBaseURL:  strings.TrimRight(keycloakBaseURL, "/"),
		gatewayBaseURL:   strings.TrimRight(gatewayBaseURL, "/"),
		demoUserPassword: demoUserPassword,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Token acquires an access token for one of the wizard's fixed demo
// identities via a Keycloak password grant.
func (c *Client) Token(ctx context.Context, identity string) (string, error) {
	username, ok := demoUsers[identity]
	if !ok {
		return "", fmt.Errorf("unknown identity %q", identity)
	}

	form := url.Values{
		"client_id":  {keycloakClientID},
		"grant_type": {"password"},
		"username":   {username},
		"password":   {c.demoUserPassword},
	}

	endpoint := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", c.keycloakBaseURL, keycloakRealm)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("building token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("requesting token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("keycloak token request failed with status %d: %s", resp.StatusCode, body)
	}

	var parsed struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parsing token response: %w", err)
	}
	return parsed.AccessToken, nil
}

// tenantToken acquires an access token for one of the multi-tenant
// isolation pillar's org identities via a Keycloak client-credentials
// grant against that identity's own realm, using the secret configured via
// WithTenantClientSecrets.
func (c *Client) tenantToken(ctx context.Context, identity string) (string, error) {
	tenant, ok := tenantIdentities[identity]
	if !ok {
		return "", fmt.Errorf("unknown tenant identity %q", identity)
	}
	secret, ok := c.tenantClientSecrets[identity]
	if !ok || secret == "" {
		return "", fmt.Errorf("no client secret configured for tenant identity %q", identity)
	}

	form := url.Values{
		"client_id":     {tenant.clientID},
		"client_secret": {secret},
		"grant_type":    {"client_credentials"},
	}

	endpoint := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", c.keycloakBaseURL, tenant.realm)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("building tenant token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("requesting tenant token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading tenant token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("keycloak tenant token request failed with status %d: %s", resp.StatusCode, body)
	}

	var parsed struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parsing tenant token response: %w", err)
	}
	return parsed.AccessToken, nil
}

// Request describes a single HTTP call to proxy to agentgateway.
type Request struct {
	Identity string
	Method   string
	Path     string
	Headers  map[string]string
	Body     string
}

// Response is the gateway's raw response, returned as-is.
type Response struct {
	StatusCode int
	Body       string
	LatencyMS  int64
}

// Proxy acquires a token for req.Identity (unless it is "" or "anonymous")
// and forwards req to agentgateway, returning the raw response.
func (c *Client) Proxy(ctx context.Context, req Request) (*Response, error) {
	var token string
	if req.Identity != "" && req.Identity != "anonymous" {
		t, err := c.Token(ctx, req.Identity)
		if err != nil {
			return nil, fmt.Errorf("acquiring token for %q: %w", req.Identity, err)
		}
		token = t
	}
	return c.ProxyWithToken(ctx, req, token)
}

// ProxyWithToken forwards req to agentgateway using token directly as the
// bearer credential, skipping Token() entirely - for callers that already
// hold a real token rather than one of this client's own fixed demo
// identities. An empty token forwards the request with no Authorization
// header, same as Proxy does for "anonymous".
func (c *Client) ProxyWithToken(ctx context.Context, req Request, token string) (*Response, error) {
	return c.proxyToBaseURL(ctx, c.gatewayBaseURL, req, token)
}

// ProxyTenant acquires a token for req.Identity via tenantToken (unless it
// is "" or "anonymous", mirroring Proxy's anonymous-identity handling) and
// forwards req to the dedicated tenants Gateway configured via
// WithTenantsGatewayBaseURL, returning the raw response.
func (c *Client) ProxyTenant(ctx context.Context, req Request) (*Response, error) {
	var token string
	if req.Identity != "" && req.Identity != "anonymous" {
		t, err := c.tenantToken(ctx, req.Identity)
		if err != nil {
			return nil, fmt.Errorf("acquiring tenant token for %q: %w", req.Identity, err)
		}
		token = t
	}
	return c.proxyToBaseURL(ctx, c.tenantsGatewayBaseURL, req, token)
}

// proxyToBaseURL forwards req to baseURL using token as the bearer
// credential, shared by ProxyWithToken (the shared gatewayBaseURL) and
// ProxyTenant (the dedicated tenantsGatewayBaseURL). An empty token
// forwards the request with no Authorization header.
func (c *Client) proxyToBaseURL(ctx context.Context, baseURL string, req Request, token string) (*Response, error) {
	method := req.Method
	if method == "" {
		method = http.MethodPost
	}

	endpoint := baseURL + req.Path
	httpReq, err := http.NewRequestWithContext(ctx, method, endpoint, strings.NewReader(req.Body))
	if err != nil {
		return nil, fmt.Errorf("building gateway request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	if token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("calling gateway: %w", err)
	}
	defer resp.Body.Close()
	latency := time.Since(start)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading gateway response: %w", err)
	}

	return &Response{StatusCode: resp.StatusCode, Body: string(body), LatencyMS: latency.Milliseconds()}, nil
}

// mcpRequest is a minimal JSON-RPC 2.0 envelope for the MCP Streamable HTTP
// handshake ProxyMCPToolCall performs.
type mcpRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int            `json:"id,omitempty"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

// ProxyMCPToolCall performs the three-step MCP Streamable HTTP handshake
// (initialize -> notifications/initialized -> tools/call) against path using
// token as the bearer credential on every request, and returns the
// tools/call result as-is. Verified live against the real Core Banking MCP
// server: initialize returns an Mcp-Session-Id response header that must be
// threaded into both subsequent requests, or the server has no session to
// attach the call to.
func (c *Client) ProxyMCPToolCall(ctx context.Context, path, token, toolName string, arguments map[string]any) (*Response, error) {
	endpoint := c.gatewayBaseURL + path

	initBody, err := json.Marshal(mcpRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "wizard-direct-call", "version": "1.0"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("encoding MCP initialize: %w", err)
	}
	_, sessionID, err := c.doMCPRequest(ctx, endpoint, token, "", string(initBody))
	if err != nil {
		return nil, fmt.Errorf("MCP initialize: %w", err)
	}
	if sessionID == "" {
		return nil, fmt.Errorf("MCP initialize: server did not return an Mcp-Session-Id header")
	}

	initializedBody, err := json.Marshal(mcpRequest{JSONRPC: "2.0", Method: "notifications/initialized"})
	if err != nil {
		return nil, fmt.Errorf("encoding MCP notifications/initialized: %w", err)
	}
	if _, _, err := c.doMCPRequest(ctx, endpoint, token, sessionID, string(initializedBody)); err != nil {
		return nil, fmt.Errorf("MCP notifications/initialized: %w", err)
	}

	callBody, err := json.Marshal(mcpRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  map[string]any{"name": toolName, "arguments": arguments},
	})
	if err != nil {
		return nil, fmt.Errorf("encoding MCP tools/call: %w", err)
	}
	resp, _, err := c.doMCPRequest(ctx, endpoint, token, sessionID, string(callBody))
	if err != nil {
		return nil, fmt.Errorf("MCP tools/call: %w", err)
	}
	return resp, nil
}

// ProxyMCP acquires a token for req.Identity (same as Proxy) and performs
// the MCP Streamable HTTP handshake (initialize ->
// notifications/initialized) against req.Path before forwarding req.Body as
// the final JSON-RPC call, returning that response as-is. Needed because
// the transport requires an established session for any non-initialize
// request - a bare one-shot call, like the demo's raw request presets send,
// doesn't carry one on its own.
//
// If the handshake's own initialize call doesn't succeed (e.g. ext-auth
// rejects it for lacking a token), that response is returned as-is rather
// than treated as a transport error - for some demo steps, that rejection
// is the actual teaching point.
func (c *Client) ProxyMCP(ctx context.Context, req Request) (*Response, error) {
	var token string
	if req.Identity != "" && req.Identity != "anonymous" {
		t, err := c.Token(ctx, req.Identity)
		if err != nil {
			return nil, fmt.Errorf("acquiring token for %q: %w", req.Identity, err)
		}
		token = t
	}

	endpoint := c.gatewayBaseURL + req.Path

	initBody, err := json.Marshal(mcpRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "wizard-request-builder", "version": "1.0"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("encoding MCP initialize: %w", err)
	}
	initResp, sessionID, err := c.doMCPRequest(ctx, endpoint, token, "", string(initBody))
	if err != nil {
		return nil, fmt.Errorf("MCP initialize: %w", err)
	}
	if initResp.StatusCode != http.StatusOK {
		return initResp, nil
	}

	if sessionID != "" {
		initializedBody, err := json.Marshal(mcpRequest{JSONRPC: "2.0", Method: "notifications/initialized"})
		if err != nil {
			return nil, fmt.Errorf("encoding MCP notifications/initialized: %w", err)
		}
		if _, _, err := c.doMCPRequest(ctx, endpoint, token, sessionID, string(initializedBody)); err != nil {
			return nil, fmt.Errorf("MCP notifications/initialized: %w", err)
		}
	}

	resp, _, err := c.doMCPRequest(ctx, endpoint, token, sessionID, req.Body)
	if err != nil {
		return nil, fmt.Errorf("MCP request: %w", err)
	}
	return resp, nil
}

// doMCPRequest POSTs one raw JSON-RPC body to endpoint and returns the raw
// response plus any Mcp-Session-Id the server set in its reply.
func (c *Client) doMCPRequest(ctx context.Context, endpoint, token, sessionID, body string) (*Response, string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		return nil, "", fmt.Errorf("building MCP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", sessionID)
	}
	if token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, "", fmt.Errorf("calling gateway: %w", err)
	}
	defer resp.Body.Close()
	latency := time.Since(start)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("reading MCP response: %w", err)
	}

	return &Response{StatusCode: resp.StatusCode, Body: string(respBody), LatencyMS: latency.Milliseconds()}, resp.Header.Get("Mcp-Session-Id"), nil
}

// ProxyStream behaves like Proxy but copies the gateway's response body to w
// as it arrives, flushing after each chunk, instead of buffering the whole
// body first - for streaming (SSE) responses, where the point is showing
// output arrive incrementally rather than waiting for the full response.
func (c *Client) ProxyStream(ctx context.Context, req Request, w http.ResponseWriter) error {
	var token string
	if req.Identity != "" && req.Identity != "anonymous" {
		t, err := c.Token(ctx, req.Identity)
		if err != nil {
			return fmt.Errorf("acquiring token for %q: %w", req.Identity, err)
		}
		token = t
	}

	method := req.Method
	if method == "" {
		method = http.MethodPost
	}

	endpoint := c.gatewayBaseURL + req.Path
	httpReq, err := http.NewRequestWithContext(ctx, method, endpoint, strings.NewReader(req.Body))
	if err != nil {
		return fmt.Errorf("building gateway request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	if token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("calling gateway: %w", err)
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(resp.StatusCode)

	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 4096)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("writing streamed response: %w", writeErr)
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return fmt.Errorf("reading streamed response: %w", readErr)
		}
	}
}
