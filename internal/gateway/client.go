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

// Client acquires tokens and proxies requests to a live agentgateway/Keycloak pair.
type Client struct {
	httpClient       *http.Client
	keycloakBaseURL  string
	gatewayBaseURL   string
	demoUserPassword string
}

// NewClient builds a Client. keycloakBaseURL and gatewayBaseURL must each
// include a scheme (e.g. "https://keycloak.demo.example.com").
// demoUserPassword is the shared password for every demoUsers entry.
func NewClient(keycloakBaseURL, gatewayBaseURL, demoUserPassword string) *Client {
	return &Client{
		httpClient:       &http.Client{Timeout: 15 * time.Second},
		keycloakBaseURL:  strings.TrimRight(keycloakBaseURL, "/"),
		gatewayBaseURL:   strings.TrimRight(gatewayBaseURL, "/"),
		demoUserPassword: demoUserPassword,
	}
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

	method := req.Method
	if method == "" {
		method = http.MethodPost
	}

	endpoint := c.gatewayBaseURL + req.Path
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
