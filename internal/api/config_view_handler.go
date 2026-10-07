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
		http.Error(w, fmt.Sprintf("policy %q is not read-only", policyID), http.StatusBadRequest)
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

		docYAML, err := yaml.Marshal(obj.Object)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err)
			return
		}
		results = append(results, viewRefResult{Name: ref.Name, Found: true, YAML: string(docYAML)})
	}

	writeJSON(w, http.StatusOK, map[string]any{"refs": results})
}
