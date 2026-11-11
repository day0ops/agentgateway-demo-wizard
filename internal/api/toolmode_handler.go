package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/day0ops/agentgateway-demo-wizard/internal/agent"
)

// errModeUnavailableMessage is shown instead of the raw error when a mode's
// MCP route can't even be listed - the shape a toolMode usecase that hasn't
// been deployed yet takes, so presenters get an actionable hint instead of a
// wrapped connection error.
const errModeUnavailableMessage = "couldn't reach this mode's MCP route - if this is a fresh environment, confirm the field-kit's toolMode usecase is deployed"

type toolModeRunRequest struct {
	Mode   string `json:"mode"`
	Prompt string `json:"prompt"`
}

type toolModeRunResponse struct {
	Answer           string `json:"answer"`
	PromptTokens     int    `json:"promptTokens"`
	CompletionTokens int    `json:"completionTokens"`
	Hops             int    `json:"hops"`
	ToolCalls        int    `json:"toolCalls"`
}

// modePath resolves a mode key to its gateway MCP route from the scenario
// registry, so the client can never request an arbitrary path.
func (s *Server) modePath(mode string) (string, bool) {
	for _, p := range s.scenarios {
		for _, step := range p.Steps {
			if step.ToolModeDemo == nil {
				continue
			}
			for _, m := range step.ToolModeDemo.Modes {
				if m.Key == mode {
					return m.MCPPath, true
				}
			}
		}
	}
	return "", false
}

func (s *Server) handleToolModeRun(w http.ResponseWriter, r *http.Request) {
	var req toolModeRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	path, ok := s.modePath(req.Mode)
	if !ok {
		http.Error(w, "unknown mode", http.StatusBadRequest)
		return
	}
	if s.agentRunner == nil || s.cfg == nil {
		writeJSONError(w, http.StatusServiceUnavailable, errors.New("agent not configured"))
		return
	}

	mcpClient := s.mcpClientFactory(path)
	res, err := s.agentRunner.RunToolMode(r.Context(), agent.Request{
		Identity: "anonymous",
		Prompt:   req.Prompt,
		Model:    "gpt-4o-mini",
	}, mcpClient)
	if err != nil {
		if errors.Is(err, agent.ErrModeUnavailable) {
			writeJSONError(w, http.StatusBadGateway, errors.New(errModeUnavailableMessage))
			return
		}
		writeJSONError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, toolModeRunResponse{
		Answer:           res.Answer,
		PromptTokens:     res.PromptTokens,
		CompletionTokens: res.CompletionTokens,
		Hops:             res.Hops,
		ToolCalls:        res.ToolCalls,
	})
}
