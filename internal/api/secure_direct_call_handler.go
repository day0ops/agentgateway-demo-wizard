package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type secureDirectCallRequest struct {
	Path      string         `json:"path"`
	Tool      string         `json:"tool"`
	Arguments map[string]any `json:"arguments"`
}

// handleSecureDirectCall makes a raw MCP tool call using the caller's own
// real session token as-is, with no exchange and no delegation chain -
// demonstrating what a non-delegated call to a chain-gated route does,
// alongside the normal OpsAgent path that always goes through the proper
// chain by construction. Reached only once ext-auth has already let the
// request through (same guarantee as /secure/ops-agent/run).
func (s *Server) handleSecureDirectCall(w http.ResponseWriter, r *http.Request) {
	var req secureDirectCallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	bearer := r.Header.Get("Authorization")
	if bearer == "" {
		writeJSONError(w, http.StatusUnauthorized, fmt.Errorf("log in first - see the login step above"))
		return
	}

	resp, err := s.gatewayClient.ProxyMCPToolCall(r.Context(), req.Path, strings.TrimPrefix(bearer, "Bearer "), req.Tool, req.Arguments)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, driveResponse{StatusCode: resp.StatusCode, Body: resp.Body, LatencyMS: resp.LatencyMS})
}
