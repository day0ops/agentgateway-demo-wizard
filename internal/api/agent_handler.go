package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/day0ops/agentgateway-demo-wizard/internal/agent"
)

// WithAgent registers POST /api/agent/run, backed by runner and the fixed
// set of tools available to this demo's agent loop.
func WithAgent(runner *agent.Runner, tools []agent.ToolSpec) Option {
	return func(s *Server) {
		s.agentRunner = runner
		s.agentTools = tools
	}
}

type agentRunRequest struct {
	Identity string `json:"identity"`
	Prompt   string `json:"prompt"`
	Model    string `json:"model"`
}

func (s *Server) handleAgentRun(w http.ResponseWriter, r *http.Request) {
	var req agentRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)

	hops := make(chan agent.HopEvent)
	done := make(chan struct{})
	var runErr error

	go func() {
		_, runErr = s.agentRunner.Run(r.Context(), agent.Request{
			Identity: req.Identity,
			Prompt:   req.Prompt,
			Model:    req.Model,
			Tools:    s.agentTools,
		}, hops)
		close(done)
	}()

	for event := range hops {
		data, _ := json.Marshal(event)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
	<-done

	if runErr != nil {
		log.Printf("agent run failed: %v", runErr)
	}
}
