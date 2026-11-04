package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/day0ops/agentgateway-demo-wizard/internal/scenarios"
)

// TestToolModeRunRejectsUnknownMode asserts the handler resolves mode keys
// against the scenario registry and never lets an unrecognized key (or,
// transitively, a raw path) reach the gateway.
func TestToolModeRunRejectsUnknownMode(t *testing.T) {
	srv := NewServer(WithScenarios(scenarios.Registry()))
	req := httptest.NewRequest(http.MethodPost, "/api/toolmode/run", strings.NewReader(`{"mode":"nope","prompt":"hi"}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// TestToolModeRunRejectsInvalidBody asserts a malformed JSON body is a 400,
// not a 500 or an unhandled panic.
func TestToolModeRunRejectsInvalidBody(t *testing.T) {
	srv := NewServer(WithScenarios(scenarios.Registry()))
	req := httptest.NewRequest(http.MethodPost, "/api/toolmode/run", strings.NewReader(`not json`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
