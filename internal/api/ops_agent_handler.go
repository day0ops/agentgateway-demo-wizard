package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type opsAgentRunRequest struct {
	Identity string `json:"identity"`
	Agent    string `json:"agent"`
	Mode     string `json:"mode"`
	Query    string `json:"query"`
}

type opsAgentRunResponse struct {
	Answer string         `json:"answer"`
	Extra  map[string]any `json:"extra,omitempty"`
}

var opsAgentHTTPClient = &http.Client{Timeout: 45 * time.Second}

func (s *Server) opsAgentURL(name string) (string, error) {
	switch name {
	case "repo":
		return s.cfg.RepoAgentURL, nil
	case "release-manager":
		return s.cfg.ReleaseManagerAgentURL, nil
	case "github":
		return s.cfg.GitHubAgentURL, nil
	default:
		return "", fmt.Errorf("unknown agent %q", name)
	}
}

// handleOpsAgentRun proxies to one of the ops-agents usecase's real external
// agent containers, forwarding the real session token captured by the auth
// pillar's login step. Unlike handleAgentRun (this repo's own in-process
// agent loop), the target is a separate HTTP service and the response is a
// single JSON object, not a stream.
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

	sessionID := s.sessionIDFromCookie(r)
	tok, ok := s.oauthStore.GetToken(sessionID, req.Identity)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, fmt.Errorf("log in as %q first - see the login step above", req.Identity))
		return
	}

	body, err := json.Marshal(map[string]string{
		"query": req.Query,
		"token": tok.AccessToken,
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

	writeJSON(w, http.StatusOK, parsed)
}
