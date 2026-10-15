package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type opsAgentRunRequest struct {
	Agent string `json:"agent"`
	Mode  string `json:"mode"`
	Query string `json:"query"`
}

type opsAgentRunResponse struct {
	Answer string         `json:"answer"`
	Extra  map[string]any `json:"extra,omitempty"`
}

var opsAgentHTTPClient = &http.Client{Timeout: 45 * time.Second}

func (s *Server) opsAgentURL(name string) (string, error) {
	switch name {
	case "accounts":
		return s.cfg.AccountsAgentURL, nil
	case "transfer-orchestrator":
		return s.cfg.TransferOrchestratorAgentURL, nil
	case "statements":
		return s.cfg.StatementsAgentURL, nil
	default:
		return "", fmt.Errorf("unknown agent %q", name)
	}
}

// handleOpsAgentRun proxies to one of the ops-agents usecase's real external
// agent containers. The bearer token comes from agentgateway's own ext-auth
// (Authorization header, injected only on requests that passed through the
// /secure-prefixed route this handler is mounted under), and identity is
// derived from that token's own preferred_username claim rather than a
// client-supplied field.
func (s *Server) handleOpsAgentRun(w http.ResponseWriter, r *http.Request) {
	var req opsAgentRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	url, err := s.opsAgentURL(req.Agent)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}

	bearer := r.Header.Get("Authorization")
	if bearer == "" {
		writeJSONError(w, http.StatusUnauthorized, fmt.Errorf("log in first - see the login step above"))
		return
	}
	identity, err := decodeJWTClaim(bearer, "preferred_username")
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, fmt.Errorf("resolving identity from token: %w", err))
		return
	}

	body, err := json.Marshal(map[string]string{
		"query": req.Query,
		"token": strings.TrimPrefix(bearer, "Bearer "),
		"mode":  req.Mode,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	agentReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	agentReq.Header.Set("Content-Type", "application/json")

	resp, err := opsAgentHTTPClient.Do(agentReq)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err)
		return
	}
	defer resp.Body.Close()

	var parsed opsAgentRunResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		writeJSONError(w, http.StatusBadGateway, fmt.Errorf("decoding agent response: %w", err))
		return
	}
	if parsed.Extra == nil {
		parsed.Extra = map[string]any{}
	}
	parsed.Extra["identity"] = identity

	writeJSON(w, http.StatusOK, parsed)
}
