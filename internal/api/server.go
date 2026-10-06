// Package api wires together every HTTP route the wizard serves, built up
// incrementally via functional options as backend capabilities are added.
package api

import (
	"io/fs"
	"net/http"
	"time"

	"github.com/day0ops/agentgateway-demo-wizard/internal/agent"
	"github.com/day0ops/agentgateway-demo-wizard/internal/config"
	"github.com/day0ops/agentgateway-demo-wizard/internal/gateway"
	"github.com/day0ops/agentgateway-demo-wizard/internal/k8s"
	"github.com/day0ops/agentgateway-demo-wizard/internal/scenarios"
)

// Server is the wizard's HTTP server. Routes are registered in NewServer;
// later packages extend it by adding an Option that stores a new dependency
// and registers the routes that need it.
type Server struct {
	mux                    *http.ServeMux
	spaFS                  fs.FS
	cfg                    *config.Config
	scenarios              []scenarios.Pillar
	applier                *k8s.Applier
	deploymentReadyTimeout time.Duration
	gatewayClient          *gateway.Client
	agentRunner            *agent.Runner
	agentTools             []agent.ToolSpec
}

// Option configures a Server at construction time.
type Option func(*Server)

// NewServer builds the wizard's HTTP server with all routes registered.
func NewServer(opts ...Option) *Server {
	s := &Server{mux: http.NewServeMux(), deploymentReadyTimeout: defaultDeploymentReadyTimeout}

	for _, opt := range opts {
		opt(s)
	}

	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /api/config", s.handleConfig)
	s.mux.HandleFunc("GET /api/scenarios", s.handleScenarios)
	s.mux.HandleFunc("POST /api/config/apply", s.handleConfigApply)
	s.mux.HandleFunc("POST /api/config/revert", s.handleConfigRevert)
	s.mux.HandleFunc("POST /api/config/reset", s.handleConfigReset)
	s.mux.HandleFunc("POST /api/request", s.handleRequest)
	s.mux.HandleFunc("POST /api/agent/run", s.handleAgentRun)
	s.mux.HandleFunc("POST /api/virtual-keys", s.handleVirtualKeyCreate)
	s.mux.HandleFunc("POST /api/virtual-keys/{name}/rotate", s.handleVirtualKeyRotate)
	s.registerSPA()

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
