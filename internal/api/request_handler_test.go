package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/day0ops/agentgateway-demo-wizard/internal/config"
	"github.com/day0ops/agentgateway-demo-wizard/internal/gateway"
	"github.com/day0ops/agentgateway-demo-wizard/internal/oauthlogin"
)

func TestRequestHandlerProxiesToGateway(t *testing.T) {
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"chatcmpl-1"}`))
	}))
	defer gatewayServer.Close()

	client := gateway.NewClient("http://unused", gatewayServer.URL, "test-password")
	s := NewServer(WithGateway(client))

	reqBody := `{"identity":"anonymous","method":"POST","path":"/openai","body":"{}"}`
	req := httptest.NewRequest(http.MethodPost, "/api/request", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"statusCode":200`) {
		t.Fatalf("expected statusCode 200 in response, got: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "chatcmpl-1") {
		t.Fatalf("expected the raw gateway body to pass through, got: %s", rec.Body.String())
	}
}

func TestRequestHandlerStreamsWhenBodyRequestsStreaming(t *testing.T) {
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: hello\n\n"))
		flusher.Flush()
	}))
	defer gatewayServer.Close()

	client := gateway.NewClient("http://unused", gatewayServer.URL, "test-password")
	s := NewServer(WithGateway(client))

	reqBody := `{"identity":"anonymous","method":"POST","path":"/openai","body":"{\"stream\":true}"}`
	req := httptest.NewRequest(http.MethodPost, "/api/request", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "data: hello") {
		t.Fatalf("expected the streamed SSE body to pass through, got: %s", rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("expected text/event-stream content type, got %q", rec.Header().Get("Content-Type"))
	}
}

func TestRequestHandlerSurfacesTransportErrors(t *testing.T) {
	client := gateway.NewClient("http://unused", "http://127.0.0.1:1", "test-password")
	s := NewServer(WithGateway(client))

	reqBody := `{"identity":"anonymous","method":"POST","path":"/openai","body":"{}"}`
	req := httptest.NewRequest(http.MethodPost, "/api/request", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRequestHandlerRequiresLoginForSessionTokenPresets(t *testing.T) {
	client := gateway.NewClient("http://unused", "http://unused", "test-password")
	s := NewServer(WithGateway(client), WithOAuthLogin("https://keycloak.demo.example.com", "/dev/null"))

	reqBody := `{"identity":"team-alpha","method":"GET","path":"/secure/exchange-standard","requiresSessionToken":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/request", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when no real session token has been captured, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "log in as") {
		t.Fatalf("expected a clear log-in-first error, got: %s", rec.Body.String())
	}
}

func TestRequestHandlerUsesCapturedSessionTokenWhenPresent(t *testing.T) {
	var gotAuth string
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"headers":{}}`))
	}))
	defer gatewayServer.Close()

	client := gateway.NewClient("http://unused", gatewayServer.URL, "test-password")
	s := NewServer(WithGateway(client), WithOAuthLogin("https://keycloak.demo.example.com", "/dev/null"))
	s.oauthStore.PutToken("session-1", "team-alpha", oauthlogin.Token{AccessToken: "real-token", ExpiresAt: time.Now().Add(time.Hour)})

	reqBody := `{"identity":"team-alpha","method":"GET","path":"/secure/exchange-standard","requiresSessionToken":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/request", strings.NewReader(reqBody))
	req.AddCookie(&http.Cookie{Name: oauthlogin.SessionCookieName, Value: "session-1"})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if gotAuth != "Bearer real-token" {
		t.Fatalf("expected the captured real token to be forwarded, got %q", gotAuth)
	}
}

func TestRequestHandlerExchangesViaSTSWhenRequested(t *testing.T) {
	sts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"sts-exchanged-token"}`))
	}))
	defer sts.Close()
	stsHost := strings.TrimPrefix(sts.URL, "http://")

	var gotAuth string
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer gatewayServer.Close()

	actorTokenPath := filepath.Join(t.TempDir(), "actor-token")
	if err := os.WriteFile(actorTokenPath, []byte("wizard-sa-token"), 0o600); err != nil {
		t.Fatalf("writing actor token fixture: %v", err)
	}

	client := gateway.NewClient("http://unused", gatewayServer.URL, "test-password")
	s := NewServer(
		WithGateway(client),
		WithOAuthLogin("https://keycloak.demo.example.com", actorTokenPath),
		WithConfig(&config.Config{STSHost: stsHost}),
	)
	s.oauthStore.PutToken("session-1", "team-alpha", oauthlogin.Token{AccessToken: "real-token", ExpiresAt: time.Now().Add(time.Hour)})

	reqBody := `{"identity":"team-alpha","method":"GET","path":"/secure/exchange-sts","requiresSessionToken":true,"exchangeViaSts":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/request", strings.NewReader(reqBody))
	req.AddCookie(&http.Cookie{Name: oauthlogin.SessionCookieName, Value: "session-1"})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if gotAuth != "Bearer sts-exchanged-token" {
		t.Fatalf("expected the STS-exchanged token to be forwarded, got %q", gotAuth)
	}
}
