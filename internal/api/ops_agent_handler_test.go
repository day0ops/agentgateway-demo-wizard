package api

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/day0ops/agentgateway-demo-wizard/internal/config"
)

// fakeBearerToken builds an unsigned JWT carrying preferred_username, for
// tests only - decodeJWTClaim never validates the signature, matching what
// agentgateway's ext-auth has already done before forwarding it.
func fakeBearerToken(preferredUsername string) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"preferred_username":"` + preferredUsername + `"}`))
	return "Bearer header." + payload + ".sig"
}

func TestOpsAgentURLResolvesConfiguredAgents(t *testing.T) {
	cfg := &config.Config{
		AccountsAgentURL:             "http://accounts-agent/run",
		TransferOrchestratorAgentURL: "http://transfer-orchestrator-agent/run",
		StatementsAgentURL:           "http://statements-agent/run",
	}
	s := NewServer(WithConfig(cfg))

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

func TestOpsAgentRunRequiresAuthorizationHeader(t *testing.T) {
	s := NewServer(WithConfig(&config.Config{}))

	reqBody := `{"agent":"accounts","mode":"","query":"what's my balance"}`
	req := httptest.NewRequest(http.MethodPost, "/secure/ops-agent/run", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no Authorization header, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOpsAgentRunForwardsTokenAndDerivesIdentityFromJWT(t *testing.T) {
	var gotBody string
	agentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"answer":"Checking balance: $4,210.55"}`))
	}))
	defer agentServer.Close()

	s := NewServer(WithConfig(&config.Config{AccountsAgentURL: agentServer.URL + "/run"}))

	reqBody := `{"agent":"accounts","mode":"sts-delegation","query":"what's my balance"}`
	req := httptest.NewRequest(http.MethodPost, "/secure/ops-agent/run", strings.NewReader(reqBody))
	req.Header.Set("Authorization", fakeBearerToken("team-alpha"))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(gotBody, `"token":"header.`) || !strings.Contains(gotBody, `"mode":"sts-delegation"`) {
		t.Fatalf("expected the forwarded token and mode in the agent request, got: %s", gotBody)
	}
	if !strings.Contains(rec.Body.String(), "Checking balance: $4,210.55") {
		t.Fatalf("expected the agent's answer to pass through, got: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"identity":"team-alpha"`) {
		t.Fatalf("expected identity derived from the JWT claim in the response, got: %s", rec.Body.String())
	}
}

func TestOpsAgentRunRejectsUnknownAgent(t *testing.T) {
	s := NewServer(WithConfig(&config.Config{}))

	reqBody := `{"agent":"nonexistent","query":"hi"}`
	req := httptest.NewRequest(http.MethodPost, "/secure/ops-agent/run", strings.NewReader(reqBody))
	req.Header.Set("Authorization", fakeBearerToken("team-alpha"))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unknown agent, got %d: %s", rec.Code, rec.Body.String())
	}
}
