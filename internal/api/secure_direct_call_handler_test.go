package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/day0ops/agentgateway-demo-wizard/internal/config"
	"github.com/day0ops/agentgateway-demo-wizard/internal/gateway"
)

func TestSecureDirectCallRequiresAuthorizationHeader(t *testing.T) {
	client := gateway.NewClient("http://unused", "http://unused", "unused")
	s := NewServer(WithGateway(client))

	reqBody := `{"path":"/core-banking-delegation/mcp","tool":"transfer","arguments":{}}`
	req := httptest.NewRequest(http.MethodPost, "/secure/direct-call", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no Authorization header, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSecureDirectCallForwardsTheRealTokenWithNoExchange(t *testing.T) {
	var sawAuth string
	var sawMethods []string
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		sawMethods = append(sawMethods, string(body))

		w.Header().Set("Content-Type", "text/event-stream")
		switch {
		case strings.Contains(string(body), "initialize") && !strings.Contains(string(body), "initialized"):
			w.Header().Set("Mcp-Session-Id", "session-xyz")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}\n\n"))
		case strings.Contains(string(body), "notifications/initialized"):
			w.WriteHeader(http.StatusAccepted)
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":2,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"ok\"}],\"isError\":false}}\n\n"))
		}
	}))
	defer gatewayServer.Close()

	client := gateway.NewClient("http://unused", gatewayServer.URL, "unused")
	s := NewServer(WithGateway(client), WithConfig(&config.Config{}))

	reqBody := `{"path":"/core-banking-delegation/mcp","tool":"transfer","arguments":{"from_account":"savings","to_account":"checking","amount":500}}`
	req := httptest.NewRequest(http.MethodPost, "/secure/direct-call", strings.NewReader(reqBody))
	req.Header.Set("Authorization", fakeBearerToken("team-alpha"))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(sawAuth, "header.") {
		t.Fatalf("expected the real forwarded token on the upstream call, got: %q", sawAuth)
	}
	if !strings.Contains(rec.Body.String(), "isError") {
		t.Fatalf("expected the tools/call result in the response, got: %s", rec.Body.String())
	}
}
