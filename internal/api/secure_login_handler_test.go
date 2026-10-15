package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecureLoginRedirectsBackToTheRequestedPillar(t *testing.T) {
	s := NewServer()

	req := httptest.NewRequest(http.MethodGet, "/secure/login?returnTo=auth", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/?returnTo=auth" {
		t.Fatalf("expected redirect to /?returnTo=auth, got %q", loc)
	}
}

func TestSecureLoginDefaultsReturnToEmpty(t *testing.T) {
	s := NewServer()

	req := httptest.NewRequest(http.MethodGet, "/secure/login", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if loc := rec.Header().Get("Location"); loc != "/?returnTo=" {
		t.Fatalf("expected redirect to /?returnTo=, got %q", loc)
	}
}
