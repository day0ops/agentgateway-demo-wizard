package api

import (
	"encoding/json"
	"net/http"

	"github.com/day0ops/agentgateway-demo-wizard/internal/config"
)

// WithConfig registers GET /api/config, backed by cfg.
func WithConfig(cfg *config.Config) Option {
	return func(s *Server) {
		s.cfg = cfg
	}
}

type configResponse struct {
	GatewayHost  string   `json:"gatewayHost"`
	KeycloakHost string   `json:"keycloakHost"`
	Missing      []string `json:"missing"`
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	resp := configResponse{Missing: []string{}}
	if s.cfg != nil {
		resp.GatewayHost = s.cfg.GatewayHost
		resp.KeycloakHost = s.cfg.KeycloakHost
		resp.Missing = s.cfg.Missing()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
