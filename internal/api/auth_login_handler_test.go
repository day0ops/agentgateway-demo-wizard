package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/day0ops/agentgateway-demo-wizard/internal/oauthlogin"
)

func TestHandleAuthLoginRedirectsToKeycloakWithPKCE(t *testing.T) {
	s := NewServer(WithOAuthLogin("https://keycloak.demo.example.com", "/dev/null"))

	req := httptest.NewRequest(http.MethodGet, "/auth/login?identity=team-alpha&return_to=auth", nil)
	req.Host = "wizard.demo.example.com"
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parsing Location: %v", err)
	}
	if !strings.HasPrefix(loc.String(), "https://keycloak.demo.example.com/realms/agw-dev/protocol/openid-connect/auth") {
		t.Fatalf("expected a redirect to Keycloak's authorize endpoint, got %q", loc.String())
	}
	q := loc.Query()
	if q.Get("client_id") != "agw-client-public" {
		t.Fatalf("expected client_id=agw-client-public, got %q", q.Get("client_id"))
	}
	if q.Get("redirect_uri") != "https://wizard.demo.example.com/auth/callback" {
		t.Fatalf("expected the redirect_uri to be built from the request's own Host, got %q", q.Get("redirect_uri"))
	}
	if q.Get("code_challenge") == "" || q.Get("state") == "" {
		t.Fatalf("expected non-empty code_challenge and state, got %+v", q)
	}
	if q.Get("code_challenge_method") != "S256" {
		t.Fatalf("expected S256, got %q", q.Get("code_challenge_method"))
	}
}

func TestHandleAuthLoginRequiresIdentity(t *testing.T) {
	s := NewServer(WithOAuthLogin("https://keycloak.demo.example.com", "/dev/null"))

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a missing identity, got %d", rec.Code)
	}
}

func TestHandleAuthCallbackCompletesLoginAndSetsCookie(t *testing.T) {
	keycloak := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"real-token","expires_in":300}`))
	}))
	defer keycloak.Close()

	s := NewServer(WithOAuthLogin(keycloak.URL, "/dev/null"))

	// Drive handleAuthLogin first so a real state/verifier is on file -
	// the callback can't be exercised with a fabricated state (TakeFlow
	// would reject it).
	loginReq := httptest.NewRequest(http.MethodGet, "/auth/login?identity=team-alpha&return_to=auth", nil)
	loginReq.Host = "wizard.demo.example.com"
	loginRec := httptest.NewRecorder()
	s.ServeHTTP(loginRec, loginReq)
	loc, _ := url.Parse(loginRec.Header().Get("Location"))
	state := loc.Query().Get("state")

	callbackReq := httptest.NewRequest(http.MethodGet, "/auth/callback?code=auth-code-1&state="+state, nil)
	callbackReq.Host = "wizard.demo.example.com"
	callbackRec := httptest.NewRecorder()
	s.ServeHTTP(callbackRec, callbackReq)

	if callbackRec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d: %s", callbackRec.Code, callbackRec.Body.String())
	}
	if callbackRec.Header().Get("Location") != "/?returnTo=auth" {
		t.Fatalf("expected a redirect back into the SPA with returnTo=auth, got %q", callbackRec.Header().Get("Location"))
	}
	cookies := callbackRec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != oauthlogin.SessionCookieName {
		t.Fatalf("expected a %s cookie to be set, got %+v", oauthlogin.SessionCookieName, cookies)
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/auth/status?identity=team-alpha", nil)
	statusReq.AddCookie(cookies[0])
	statusRec := httptest.NewRecorder()
	s.ServeHTTP(statusRec, statusReq)
	if !strings.Contains(statusRec.Body.String(), `"loggedIn":true`) {
		t.Fatalf("expected loggedIn:true after a completed login, got: %s", statusRec.Body.String())
	}
}

func TestHandleAuthCallbackRejectsUnknownState(t *testing.T) {
	s := NewServer(WithOAuthLogin("https://keycloak.demo.example.com", "/dev/null"))

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=auth-code-1&state=never-issued", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unknown state, got %d", rec.Code)
	}
}

func TestHandleAuthStatusFalseWithNoSession(t *testing.T) {
	s := NewServer(WithOAuthLogin("https://keycloak.demo.example.com", "/dev/null"))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/status?identity=team-alpha", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `"loggedIn":false`) {
		t.Fatalf("expected loggedIn:false with no session cookie, got: %s", rec.Body.String())
	}
}
