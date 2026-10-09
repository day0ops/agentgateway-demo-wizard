package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/day0ops/agentgateway-demo-wizard/internal/k8s"
	"github.com/day0ops/agentgateway-demo-wizard/internal/scenarios"
)

// defaultDeploymentReadyTimeout bounds how long handleConfigApply waits for
// each Deployment a policy applies to become ready before emitting "done".
const defaultDeploymentReadyTimeout = 60 * time.Second

// WithApplier registers the Configure-pane routes (apply/revert/reset),
// backed by applier.
func WithApplier(applier *k8s.Applier) Option {
	return func(s *Server) {
		s.applier = applier
	}
}

// WithDeploymentReadyTimeout overrides how long handleConfigApply waits for a
// policy's Deployments to become ready before giving up. Tests use this to
// avoid waiting out defaultDeploymentReadyTimeout; production never sets it.
func WithDeploymentReadyTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.deploymentReadyTimeout = timeout
	}
}

type policyActionRequest struct {
	PolicyID string `json:"policyId"`
}

func (s *Server) handleConfigApply(w http.ResponseWriter, r *http.Request) {
	var req policyActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	policy, ok := s.findPolicy(req.PolicyID)
	if !ok {
		http.Error(w, fmt.Sprintf("unknown policy %q", req.PolicyID), http.StatusNotFound)
		return
	}
	if policy.ReadOnly {
		http.Error(w, fmt.Sprintf("policy %q is read-only, no config to apply/revert", policy.ID), http.StatusBadRequest)
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

	writeSSE(w, flusher, sseEvent{Type: "progress", Message: fmt.Sprintf("Rendering manifests for %q...", policy.Title)})

	params := s.manifestParams()

	docs, err := policy.Render(params)
	if err != nil {
		writeSSE(w, flusher, sseEvent{Type: "error", Message: err.Error()})
		return
	}
	patchDocs, err := policy.RenderPatch(params)
	if err != nil {
		writeSSE(w, flusher, sseEvent{Type: "error", Message: err.Error()})
		return
	}

	writeSSE(w, flusher, sseEvent{Type: "progress", Message: "Applying to cluster..."})

	var applied []k8s.AppliedResource
	if len(docs) > 0 {
		a, err := s.applier.Apply(r.Context(), policy.ID, docs)
		if err != nil {
			writeSSE(w, flusher, sseEvent{Type: "error", Message: err.Error()})
			return
		}
		applied = append(applied, a...)
	}
	if len(patchDocs) > 0 {
		a, err := s.applier.ApplyPatch(r.Context(), policy.ID, patchDocs)
		if err != nil {
			writeSSE(w, flusher, sseEvent{Type: "error", Message: err.Error()})
			return
		}
		applied = append(applied, a...)
	}

	dtos := make([]appliedResourceDTO, 0, len(applied))
	for _, a := range applied {
		dtos = append(dtos, appliedResourceDTO{Resource: a.GVR.Resource, Namespace: a.Namespace, Name: a.Name})
	}

	for _, a := range applied {
		if a.GVR.Resource != "deployments" {
			continue
		}
		writeSSE(w, flusher, sseEvent{Type: "progress", Message: fmt.Sprintf("Waiting for %s to start...", a.Name)})
		if err := s.applier.WaitForDeploymentReady(r.Context(), a.Namespace, a.Name, s.deploymentReadyTimeout); err != nil {
			writeSSE(w, flusher, sseEvent{Type: "error", Message: err.Error()})
			return
		}
	}

	allDocs := append(append([][]byte{}, docs...), patchDocs...)
	yamlDocs := make([]string, 0, len(allDocs))
	for _, d := range allDocs {
		yamlDocs = append(yamlDocs, string(d))
	}

	writeSSE(w, flusher, sseEvent{
		Type:    "done",
		Message: fmt.Sprintf("Applied %d resource(s)", len(applied)),
		Applied: dtos,
		YAML:    strings.Join(yamlDocs, "\n---\n"),
	})
}

func (s *Server) handleConfigRevert(w http.ResponseWriter, r *http.Request) {
	var req policyActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	policy, ok := s.findPolicy(req.PolicyID)
	if !ok {
		http.Error(w, fmt.Sprintf("unknown policy %q", req.PolicyID), http.StatusNotFound)
		return
	}
	if policy.ReadOnly {
		http.Error(w, fmt.Sprintf("policy %q is read-only, no config to apply/revert", policy.ID), http.StatusBadRequest)
		return
	}

	if err := s.applier.Revert(r.Context(), policy.ID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	if policy.PatchRevertManifestPath != "" {
		revertDocs, err := policy.RenderPatchRevert(s.manifestParams())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err)
			return
		}
		if err := s.applier.RevertPatch(r.Context(), policy.ID, revertDocs); err != nil {
			writeJSONError(w, http.StatusInternalServerError, err)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleConfigReset(w http.ResponseWriter, r *http.Request) {
	if err := s.applier.ResetAll(r.Context()); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	for _, pillar := range s.scenarios {
		for _, step := range pillar.Steps {
			for _, policy := range step.Policies {
				if policy.PatchRevertManifestPath == "" || !s.applier.IsApplied(policy.ID) {
					continue
				}
				revertDocs, err := policy.RenderPatchRevert(s.manifestParams())
				if err != nil {
					writeJSONError(w, http.StatusInternalServerError, err)
					return
				}
				if err := s.applier.RevertPatch(r.Context(), policy.ID, revertDocs); err != nil {
					writeJSONError(w, http.StatusInternalServerError, err)
					return
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) findPolicy(policyID string) (scenarios.Policy, bool) {
	for _, pillar := range s.scenarios {
		for _, step := range pillar.Steps {
			for _, p := range step.Policies {
				if p.ID == policyID {
					return p, true
				}
			}
		}
	}
	return scenarios.Policy{}, false
}

func (s *Server) manifestParams() scenarios.ManifestParams {
	params := scenarios.ManifestParams{
		Namespace:   "agentgateway-system",
		GatewayName: "agentgateway-gw",
		GatewayNS:   "agentgateway-system",
	}
	if s.cfg != nil {
		params.GatewayHost = s.cfg.GatewayHost
		params.KeycloakHost = s.cfg.KeycloakHost
		params.OpenAIKey = s.cfg.OpenAIKey
		params.StockMCPImage = s.cfg.StockMCPImage
		params.CurrencyMCPImage = s.cfg.CurrencyMCPImage
		params.OAuthTokenExchangeClientSecret = s.cfg.OAuthTokenExchangeClientSecret
		params.STSHost = s.cfg.STSHost
	}
	return params
}
