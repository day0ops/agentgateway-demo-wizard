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
	Answer           string              `json:"answer"`
	PromptTokens     int                 `json:"promptTokens"`
	CompletionTokens int                 `json:"completionTokens"`
	Hops             int                 `json:"hops"`
	ToolCalls        int                 `json:"toolCalls"`
	ToolsExposed     []string            `json:"toolsExposed"`
	ToolInvocations  []toolInvocationDTO `json:"toolInvocations"`
}

// toolInvocationDTO is one tool call the model made, with its raw (unparsed)
// arguments - for Code/CodeSearch this is where the model's generated code
// shows up, as the arguments to run_code.
type toolInvocationDTO struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
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

	invocations := make([]toolInvocationDTO, 0, len(res.ToolInvocations))
	for _, inv := range res.ToolInvocations {
		invocations = append(invocations, toolInvocationDTO{Name: inv.Name, Arguments: inv.Arguments})
	}
	writeJSON(w, http.StatusOK, toolModeRunResponse{
		Answer:           res.Answer,
		PromptTokens:     res.PromptTokens,
		CompletionTokens: res.CompletionTokens,
		Hops:             res.Hops,
		ToolCalls:        res.ToolCalls,
		ToolsExposed:     res.ToolsExposed,
		ToolInvocations:  invocations,
	})
}
