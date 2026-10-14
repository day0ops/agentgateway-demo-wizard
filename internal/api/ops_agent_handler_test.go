package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/day0ops/agentgateway-demo-wizard/internal/config"
	"github.com/day0ops/agentgateway-demo-wizard/internal/oauthlogin"
)

func TestOpsAgentURLResolvesConfiguredAgents(t *testing.T) {
	cfg := &config.Config{
		AccountsAgentURL:             "http://accounts-agent/run",
		TransferOrchestratorAgentURL: "http://transfer-orchestrator-agent/run",
		StatementsAgentURL:           "http://statements-agent/run",
	}
	s := NewServer(WithOAuthLogin("https://keycloak.demo.example.com", "/dev/null"), WithConfig(cfg))

	cases := []struct {
		agent   string
		wantURL string
	}{
		{"accounts", cfg.AccountsAgentURL},
		{"transfer-orchestrator", cfg.TransferOrchestratorAgentURL},
		{"statements", cfg.StatementsAgentURL},
	}
	for _, tc := range cases {
		got, err := s.opsAgentURL(tc.agent)
		if err != nil {
			t.Fatalf("opsAgentURL(%q): unexpected error: %v", tc.agent, err)
		}
		if got != tc.wantURL {
			t.Fatalf("opsAgentURL(%q) = %q, want %q", tc.agent, got, tc.wantURL)
		}
	}

	if _, err := s.opsAgentURL("nonexistent"); err == nil {
		t.Fatalf("opsAgentURL(%q): expected an error for an unknown agent", "nonexistent")
	}
}

func TestOpsAgentRunRequiresLogin(t *testing.T) {
	s := NewServer(WithOAuthLogin("https://keycloak.demo.example.com", "/dev/null"), WithConfig(&config.Config{}))

	reqBody := `{"identity":"team-alpha","agent":"accounts","mode":"","query":"what's my balance"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ops-agent/run", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when no real session token has been captured, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOpsAgentRunForwardsTokenAndModeToTheTargetAgent(t *testing.T) {
	var gotBody string
	agentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"answer":"Checking balance: $4,210.55","extra":{"chain":{"sub":"team-alpha"}}}`))
	}))
	defer agentServer.Close()

	s := NewServer(
		WithOAuthLogin("https://keycloak.demo.example.com", "/dev/null"),
		WithConfig(&config.Config{AccountsAgentURL: agentServer.URL + "/run"}),
	)
	s.oauthStore.PutToken("session-1", "team-alpha", oauthlogin.Token{AccessToken: "real-token", ExpiresAt: time.Now().Add(time.Hour)})

	reqBody := `{"identity":"team-alpha","agent":"accounts","mode":"sts-delegation","query":"what's my balance"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ops-agent/run", strings.NewReader(reqBody))
	req.AddCookie(&http.Cookie{Name: oauthlogin.SessionCookieName, Value: "session-1"})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(gotBody, `"token":"real-token"`) || !strings.Contains(gotBody, `"mode":"sts-delegation"`) {
		t.Fatalf("expected the captured token and mode to be forwarded, got: %s", gotBody)
	}
	if !strings.Contains(rec.Body.String(), "Checking balance: $4,210.55") || !strings.Contains(rec.Body.String(), "chain") {
		t.Fatalf("expected the agent's answer and extra fields to pass through, got: %s", rec.Body.String())
	}
}

func TestOpsAgentRunRejectsUnknownAgent(t *testing.T) {
	s := NewServer(WithOAuthLogin("https://keycloak.demo.example.com", "/dev/null"), WithConfig(&config.Config{}))
	s.oauthStore.PutToken("session-1", "team-alpha", oauthlogin.Token{AccessToken: "real-token", ExpiresAt: time.Now().Add(time.Hour)})

	reqBody := `{"identity":"team-alpha","agent":"nonexistent","query":"hi"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ops-agent/run", strings.NewReader(reqBody))
	req.AddCookie(&http.Cookie{Name: oauthlogin.SessionCookieName, Value: "session-1"})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unknown agent, got %d: %s", rec.Code, rec.Body.String())
	}
}
