package api

import (
	"encoding/json"
	"net/http"

	"github.com/day0ops/agentgateway-demo-wizard/internal/scenarios"
)

// WithScenarios registers GET /api/scenarios, backed by reg.
func WithScenarios(reg []scenarios.Pillar) Option {
	return func(s *Server) {
		s.scenarios = reg
	}
}

func (s *Server) handleScenarios(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.scenarios)
}
