package oauthlogin

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadActorTokenTrimsWhitespace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("sa-token-1\n"), 0o600); err != nil {
		t.Fatalf("writing fixture token: %v", err)
	}

	tok, err := ReadActorToken(path)
	if err != nil {
		t.Fatalf("ReadActorToken: %v", err)
	}
	if tok != "sa-token-1" {
		t.Fatalf("expected sa-token-1, got %q", tok)
	}
}

func TestReadActorTokenMissingFileErrors(t *testing.T) {
	_, err := ReadActorToken(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatalf("expected an error for a missing file")
	}
}

func TestExchangeViaSTSPostsExpectedFormAndParsesToken(t *testing.T) {
	var gotBody string
	sts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth2/token" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"sts-exchanged-token"}`))
	}))
	defer sts.Close()

	host := strings.TrimPrefix(sts.URL, "http://")
	token, err := ExchangeViaSTS(context.Background(), sts.Client(), host, "user-subject-token", "wizard-actor-token")
	if err != nil {
		t.Fatalf("ExchangeViaSTS: %v", err)
	}
	if token != "sts-exchanged-token" {
		t.Fatalf("expected sts-exchanged-token, got %q", token)
	}
	for _, want := range []string{
		"grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Atoken-exchange",
		"subject_token=user-subject-token",
		"actor_token=wizard-actor-token",
	} {
		if !strings.Contains(gotBody, want) {
			t.Fatalf("expected form body to contain %q, got: %s", want, gotBody)
		}
	}
}

func TestExchangeViaSTSSurfacesNonOKStatus(t *testing.T) {
	sts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"invalid_actor_token"}`))
	}))
	defer sts.Close()

	host := strings.TrimPrefix(sts.URL, "http://")
	_, err := ExchangeViaSTS(context.Background(), sts.Client(), host, "subject", "actor")
	if err == nil {
		t.Fatalf("expected an error for a non-200 response")
	}
	if !strings.Contains(err.Error(), "invalid_actor_token") {
		t.Fatalf("expected the error to include the raw response body, got: %v", err)
	}
}
