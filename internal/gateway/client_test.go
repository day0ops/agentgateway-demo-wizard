package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
