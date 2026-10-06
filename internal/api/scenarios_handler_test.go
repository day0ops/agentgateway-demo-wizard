package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/day0ops/agentgateway-demo-wizard/internal/scenarios"
)

func TestScenariosHandler(t *testing.T) {
	reg := []scenarios.Pillar{
		{ID: "intro", Title: "Welcome", Steps: []scenarios.Step{{ID: "welcome", Title: "Hi"}}},
	}
	s := NewServer(WithScenarios(reg))

	req := httptest.NewRequest(http.MethodGet, "/api/scenarios", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var got []scenarios.Pillar
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(got) != 1 || got[0].ID != "intro" {
		t.Fatalf("unexpected response: %+v", got)
	}
}
