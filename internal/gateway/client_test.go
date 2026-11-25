package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClientUsesAGenerousTimeoutForLiveLLMCalls(t *testing.T) {
	client := NewClient("http://unused", "http://unused", "test-password")
	if client.httpClient.Timeout < 60*time.Second {
		t.Fatalf("expected a timeout of at least 60s to tolerate slow live LLM responses, got %s", client.httpClient.Timeout)
	}
}

func TestTokenReturnsAccessTokenForKnownIdentity(t *testing.T) {
	keycloak := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/realms/agw-dev/protocol/openid-connect/token" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "username=user1") {
			t.Fatalf("expected user1 in form body, got: %s", body)
		}
		if !strings.Contains(string(body), "password=test-password") {
			t.Fatalf("expected the configured demo password in form body, got: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "test-token"})
	}))
	defer keycloak.Close()

	client := NewClient(keycloak.URL, "http://unused", "test-password")

	token, err := client.Token(context.Background(), "team-alpha")
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if token != "test-token" {
		t.Fatalf("expected test-token, got %q", token)
	}
}

func TestTokenRejectsUnknownIdentity(t *testing.T) {
	client := NewClient("http://unused", "http://unused", "test-password")
	if _, err := client.Token(context.Background(), "nobody"); err == nil {
		t.Fatalf("expected an error for an unknown identity")
	}
}

func TestProxyAttachesBearerTokenAndReturnsRawResponse(t *testing.T) {
	keycloak := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "test-token"})
	}))
	defer keycloak.Close()

	var gotAuth string
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"budget exceeded"}`))
	}))
	defer gatewayServer.Close()

	client := NewClient(keycloak.URL, gatewayServer.URL, "test-password")

	resp, err := client.Proxy(context.Background(), Request{
		Identity: "team-beta",
		Path:     "/openai",
		Body:     `{"model":"gpt-4o-mini"}`,
	})
	if err != nil {
		t.Fatalf("Proxy: %v", err)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("expected Authorization header to be set, got %q", gotAuth)
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected the raw 429 to pass through, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Body, "budget exceeded") {
		t.Fatalf("expected the raw error body to pass through, got %q", resp.Body)
	}
}

func TestProxyAnonymousIdentitySkipsToken(t *testing.T) {
	var gotAuth string
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer gatewayServer.Close()

	client := NewClient("http://unused", gatewayServer.URL, "test-password")

	resp, err := client.Proxy(context.Background(), Request{Identity: "anonymous", Path: "/openai", Body: "{}"})
	if err != nil {
		t.Fatalf("Proxy: %v", err)
	}
	if gotAuth != "" {
		t.Fatalf("expected no Authorization header for anonymous identity, got %q", gotAuth)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected the raw 401 to pass through, got %d", resp.StatusCode)
	}
}

func TestProxyStreamCopiesChunksAsTheyArrive(t *testing.T) {
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: chunk1\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte("data: chunk2\n\n"))
		flusher.Flush()
	}))
	defer gatewayServer.Close()

	client := NewClient("http://unused", gatewayServer.URL, "test-password")
	rec := httptest.NewRecorder()

	err := client.ProxyStream(context.Background(), Request{Identity: "anonymous", Path: "/openai", Body: `{"stream":true}`}, rec)
	if err != nil {
		t.Fatalf("ProxyStream: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "chunk1") || !strings.Contains(body, "chunk2") {
		t.Fatalf("expected both chunks in the streamed body, got: %s", body)
	}
}

func TestProxyWithTokenUsesSuppliedTokenWithoutCallingKeycloak(t *testing.T) {
	keycloakCalled := false
	keycloak := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keycloakCalled = true
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "ropc-token"})
	}))
	defer keycloak.Close()

	var gotAuth string
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer gatewayServer.Close()

	client := NewClient(keycloak.URL, gatewayServer.URL, "test-password")

	resp, err := client.ProxyWithToken(context.Background(), Request{Identity: "team-alpha", Path: "/secure/exchange-standard", Body: "{}"}, "real-captured-token")
	if err != nil {
		t.Fatalf("ProxyWithToken: %v", err)
	}
	if keycloakCalled {
		t.Fatalf("expected ProxyWithToken to never call Keycloak's token endpoint")
	}
	if gotAuth != "Bearer real-captured-token" {
		t.Fatalf("expected the supplied token to be forwarded as-is, got %q", gotAuth)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestProxyWithTokenEmptyTokenOmitsAuthorizationHeader(t *testing.T) {
	var gotAuth string
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer gatewayServer.Close()

	client := NewClient("http://unused", gatewayServer.URL, "test-password")

	if _, err := client.ProxyWithToken(context.Background(), Request{Path: "/secure/exchange-standard", Body: "{}"}, ""); err != nil {
		t.Fatalf("ProxyWithToken: %v", err)
	}
	if gotAuth != "" {
		t.Fatalf("expected no Authorization header for an empty token, got %q", gotAuth)
	}
}

func TestProxyMCPToolCallPerformsTheThreeStepHandshake(t *testing.T) {
	var sawSessionIDs []string
	var sawMethods []string
	var sawAuth []string

	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var decoded struct {
			Method string `json:"method"`
		}
		_ = json.Unmarshal(body, &decoded)
		sawMethods = append(sawMethods, decoded.Method)
		sawSessionIDs = append(sawSessionIDs, r.Header.Get("Mcp-Session-Id"))
		sawAuth = append(sawAuth, r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "text/event-stream")
		switch decoded.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "test-session-123")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}\n\n"))
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/call":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":2,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"ok\"}],\"isError\":false}}\n\n"))
		}
	}))
	defer gatewayServer.Close()

	client := NewClient("http://unused", gatewayServer.URL, "unused")
	resp, err := client.ProxyMCPToolCall(context.Background(), "/core-banking-delegation/mcp", "real-token", "transfer", map[string]any{"from_account": "savings", "to_account": "checking", "amount": 500})
	if err != nil {
		t.Fatalf("ProxyMCPToolCall: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Body, `"isError":false`) {
		t.Fatalf("expected the tools/call result in the body, got: %s", resp.Body)
	}

	if len(sawMethods) != 3 || sawMethods[0] != "initialize" || sawMethods[1] != "notifications/initialized" || sawMethods[2] != "tools/call" {
		t.Fatalf("expected initialize, notifications/initialized, tools/call in order, got: %v", sawMethods)
	}
	if sawSessionIDs[1] != "test-session-123" || sawSessionIDs[2] != "test-session-123" {
		t.Fatalf("expected the session ID threaded into both later requests, got: %v", sawSessionIDs)
	}
	for _, auth := range sawAuth {
		if auth != "Bearer real-token" {
			t.Fatalf("expected every request to carry the real token, got: %q", auth)
		}
	}
}

func TestProxyMCPToolCallErrorsWithoutASessionID(t *testing.T) {
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}\n\n"))
	}))
	defer gatewayServer.Close()

	client := NewClient("http://unused", gatewayServer.URL, "unused")
	if _, err := client.ProxyMCPToolCall(context.Background(), "/core-banking-delegation/mcp", "real-token", "transfer", map[string]any{}); err == nil {
		t.Fatalf("expected an error when the server never returns an Mcp-Session-Id")
	}
}

func TestProxyMCPToolCallPassesThroughANonSuccessToolsCallResponse(t *testing.T) {
	var sawMethods []string

	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var decoded struct {
			Method string `json:"method"`
		}
		_ = json.Unmarshal(body, &decoded)
		sawMethods = append(sawMethods, decoded.Method)

		w.Header().Set("Content-Type", "text/event-stream")
		switch decoded.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "test-session-123")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}\n\n"))
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/call":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":2,\"error\":{\"code\":-32000,\"message\":\"denied by policy\"}}\n\n"))
		}
	}))
	defer gatewayServer.Close()

	client := NewClient("http://unused", gatewayServer.URL, "unused")
	resp, err := client.ProxyMCPToolCall(context.Background(), "/core-banking-delegation/mcp", "real-token", "transfer", map[string]any{"from_account": "savings", "to_account": "checking", "amount": 500})
	if err != nil {
		t.Fatalf("ProxyMCPToolCall: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Body, "denied by policy") {
		t.Fatalf("expected the error message in the body, got: %s", resp.Body)
	}
	if len(sawMethods) != 3 || sawMethods[0] != "initialize" || sawMethods[1] != "notifications/initialized" || sawMethods[2] != "tools/call" {
		t.Fatalf("expected all three handshake steps, got: %v", sawMethods)
	}
}

// TestProxyMCPPerformsHandshakeThenForwardsRawBody is the regression test
// for a live bug: the wizard's generic raw-request presets POST a single
// bare JSON-RPC call (e.g. tools/list) straight to /mcp with no prior
// session, and agentgateway's Streamable HTTP transport rejects any
// non-initialize request with no session with 400. ProxyMCP performs the
// initialize/notifications/initialized handshake first, then forwards the
// caller's own body as the final step.
func TestProxyMCPPerformsHandshakeThenForwardsRawBody(t *testing.T) {
	var sawMethods []string
	var sawSessionIDs []string
	var sawAuth []string

	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var decoded struct {
			Method string `json:"method"`
		}
		_ = json.Unmarshal(body, &decoded)
		sawMethods = append(sawMethods, decoded.Method)
		sawSessionIDs = append(sawSessionIDs, r.Header.Get("Mcp-Session-Id"))
		sawAuth = append(sawAuth, r.Header.Get("Authorization"))

		switch decoded.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "test-session-456")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"stock_get_stock_price"}]}}`))
		}
	}))
	defer gatewayServer.Close()

	client := NewClient("http://unused", gatewayServer.URL, "unused")
	resp, err := client.ProxyMCP(context.Background(), Request{
		Identity: "",
		Method:   http.MethodPost,
		Path:     "/mcp",
		Body:     `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
	})
	if err != nil {
		t.Fatalf("ProxyMCP: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, resp.Body)
	}
	if !strings.Contains(resp.Body, "stock_get_stock_price") {
		t.Fatalf("expected the tools/list result in the body, got: %s", resp.Body)
	}
	if len(sawMethods) != 3 || sawMethods[0] != "initialize" || sawMethods[1] != "notifications/initialized" || sawMethods[2] != "tools/list" {
		t.Fatalf("expected initialize, notifications/initialized, tools/list in order, got: %v", sawMethods)
	}
	if sawSessionIDs[1] != "test-session-456" || sawSessionIDs[2] != "test-session-456" {
		t.Fatalf("expected the session ID threaded into both later requests, got: %v", sawSessionIDs)
	}
}

// TestProxyMCPPassesThroughANonSuccessInitializeResponse covers the "no
// token" teaching step: if ext-auth rejects the handshake's own initialize
// call, that rejection (e.g. a 401) is the real demo outcome and must be
// returned as-is, not hidden behind a generic transport error.
func TestProxyMCPPassesThroughANonSuccessInitializeResponse(t *testing.T) {
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer gatewayServer.Close()

	client := NewClient("http://unused", gatewayServer.URL, "unused")
	resp, err := client.ProxyMCP(context.Background(), Request{
		Method: http.MethodPost,
		Path:   "/mcp",
		Body:   `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
	})
	if err != nil {
		t.Fatalf("ProxyMCP: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected the initialize step's 401 to pass through, got %d", resp.StatusCode)
	}
}

// TestProxyMCPWorksWithoutServerAssignedSessionID covers a session-less
// server - per spec, session IDs are optional, so ProxyMCP must still
// forward the final request even when initialize never assigns one.
func TestProxyMCPWorksWithoutServerAssignedSessionID(t *testing.T) {
	var sawSessionHeaderOnFinalCall bool

	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var decoded struct {
			Method string `json:"method"`
		}
		_ = json.Unmarshal(body, &decoded)

		switch decoded.Method {
		case "initialize":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
		case "tools/list":
			if r.Header.Get("Mcp-Session-Id") != "" {
				sawSessionHeaderOnFinalCall = true
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`))
		}
	}))
	defer gatewayServer.Close()

	client := NewClient("http://unused", gatewayServer.URL, "unused")
	resp, err := client.ProxyMCP(context.Background(), Request{
		Method: http.MethodPost,
		Path:   "/mcp",
		Body:   `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
	})
	if err != nil {
		t.Fatalf("ProxyMCP: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, resp.Body)
	}
	if sawSessionHeaderOnFinalCall {
		t.Fatalf("expected no Mcp-Session-Id header when the server never assigned one")
	}
}
