// Package api: GET /api/config/view surfaces pre-provisioned configuration a
// ReadOnly Policy only displays, never applies - see
// docs/superpowers/specs/2026-10-08-demo-wizard-field-kit-integration-design.md
// in agentgateway-field-kit.
package api

import (
	"fmt"
	"net/http"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/yaml"

	"github.com/day0ops/agentgateway-demo-wizard/internal/scenarios"
)

// configViewNamespace matches the hardcoded namespace every other handler in
// this package already assumes (see manifestParams() in
// config_apply_handler.go) - this repo has no multi-namespace concept yet.
const configViewNamespace = "agentgateway-system"

type viewRefResult struct {
	Name  string `json:"name"`
	Found bool   `json:"found"`
	YAML  string `json:"yaml,omitempty"`
}

func (s *Server) handleConfigView(w http.ResponseWriter, r *http.Request) {
	policyID := r.URL.Query().Get("policyId")

	policy, ok := s.findPolicy(policyID)
	if !ok {
		http.Error(w, fmt.Sprintf("unknown policy %q", policyID), http.StatusNotFound)
		return
	}
	if !policy.ReadOnly {
		s.handleConfigViewManifest(w, policy)
		return
	}

	results := make([]viewRefResult, 0, len(policy.ViewRefs))
	for _, ref := range policy.ViewRefs {
		if ref.GVR.Resource == "secrets" {
			http.Error(w, fmt.Sprintf("policy %q: refusing to view a Secret resource", policyID), http.StatusBadRequest)
			return
		}

		obj, err := s.applier.Get(r.Context(), ref.GVR, configViewNamespace, ref.Name)
		if apierrors.IsNotFound(err) {
			results = append(results, viewRefResult{Name: ref.Name, Found: false})
			continue
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err)
			return
		}

		stripAppliedState(obj.Object)
		docYAML, err := yaml.Marshal(obj.Object)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err)
			return
		}
		results = append(results, viewRefResult{Name: ref.Name, Found: true, YAML: string(docYAML)})
	}

	writeJSON(w, http.StatusOK, map[string]any{"refs": results})
}

// stripAppliedState removes fields a server assigns on write (managedFields,
// resourceVersion, uid, generation, creationTimestamp) and the status
// subresource from obj, in place - a policy's ViewRefs point at
// already-applied cluster objects, but the point of this view is showing a
// reader raw config they could apply themselves, not a dump of this
// particular cluster's applied state.
func stripAppliedState(obj map[string]any) {
	if metadata, ok := obj["metadata"].(map[string]any); ok {
		for _, field := range []string{"managedFields", "resourceVersion", "uid", "generation", "creationTimestamp", "selfLink"} {
			delete(metadata, field)
		}
	}
	delete(obj, "status")
}

// handleConfigViewManifest previews a non-ReadOnly policy's manifest by
// rendering its template directly - no cluster round-trip, so it returns the
// same content whether or not the policy has actually been applied yet.
func (s *Server) handleConfigViewManifest(w http.ResponseWriter, policy scenarios.Policy) {
	docs, err := policy.Render(s.manifestParams())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	results := make([]viewRefResult, 0, len(docs))
	for _, doc := range docs {
		var parsed struct {
			Kind     string `json:"kind"`
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		}
		name := policy.Title
		if err := yaml.Unmarshal(doc, &parsed); err == nil && parsed.Kind != "" {
			name = fmt.Sprintf("%s/%s", parsed.Kind, parsed.Metadata.Name)
		}
		if parsed.Kind == "Secret" {
			// Refuse to leak secret material through this preview path - omit
			// the doc entirely rather than failing the whole policy, since a
			// policy's other docs (routes, policies, etc.) are still safe to show.
			continue
		}
		results = append(results, viewRefResult{Name: name, Found: true, YAML: string(doc)})
	}

	writeJSON(w, http.StatusOK, map[string]any{"refs": results})
}
