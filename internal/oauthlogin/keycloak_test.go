package oauthlogin

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExchangeCodePostsExpectedFormAndParsesToken(t *testing.T) {
	var gotBody string
	keycloak := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/realms/agw-dev/protocol/openid-connect/token" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"real-token","expires_in":300}`))
	}))
	defer keycloak.Close()

	tok, err := ExchangeCode(context.Background(), keycloak.Client(), keycloak.URL, "agw-client-public", "https://wizard.example.com/auth/callback", "auth-code-1", "verifier-1")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if tok.AccessToken != "real-token" {
		t.Fatalf("expected real-token, got %q", tok.AccessToken)
	}
	if time.Until(tok.ExpiresAt) <= 0 || time.Until(tok.ExpiresAt) > 300*time.Second {
		t.Fatalf("expected ExpiresAt roughly 300s out, got %v", tok.ExpiresAt)
	}
	for _, want := range []string{"grant_type=authorization_code", "client_id=agw-client-public", "code=auth-code-1", "code_verifier=verifier-1"} {
		if !strings.Contains(gotBody, want) {
			t.Fatalf("expected form body to contain %q, got: %s", want, gotBody)
		}
	}
}

func TestExchangeCodeSurfacesNonOKStatus(t *testing.T) {
	keycloak := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer keycloak.Close()

	_, err := ExchangeCode(context.Background(), keycloak.Client(), keycloak.URL, "agw-client-public", "https://wizard.example.com/auth/callback", "bad-code", "verifier-1")
	if err == nil {
		t.Fatalf("expected an error for a non-200 response")
	}
	if !strings.Contains(err.Error(), "invalid_grant") {
		t.Fatalf("expected the error to include the raw response body, got: %v", err)
	}
}
