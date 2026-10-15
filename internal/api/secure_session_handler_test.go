package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecureSessionRequiresAuthorizationHeader(t *testing.T) {
	s := NewServer()

	req := httptest.NewRequest(http.MethodGet, "/secure/session", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no Authorization header, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSecureSessionReturnsIdentityFromTheJWT(t *testing.T) {
	s := NewServer()

	req := httptest.NewRequest(http.MethodGet, "/secure/session", nil)
	req.Header.Set("Authorization", fakeBearerToken("team-alpha"))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != `{"identity":"team-alpha"}`+"\n" {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}
