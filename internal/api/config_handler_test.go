package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/day0ops/agentgateway-demo-wizard/internal/config"
)

func TestConfigHandler(t *testing.T) {
	cfg := &config.Config{GatewayHost: "agw.example.com", KeycloakHost: "", OpenAIKey: "sk-x"}
	s := NewServer(WithConfig(cfg))

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body struct {
		GatewayHost string   `json:"gatewayHost"`
		Missing     []string `json:"missing"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body.GatewayHost != "agw.example.com" {
		t.Fatalf("unexpected gatewayHost: %q", body.GatewayHost)
	}
	if len(body.Missing) != 3 {
		t.Fatalf("expected 3 missing values, got %v", body.Missing)
	}
}
