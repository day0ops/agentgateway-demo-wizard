package api

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"github.com/day0ops/agentgateway-demo-wizard/internal/k8s"
	"github.com/day0ops/agentgateway-demo-wizard/internal/scenarios"
)

var guardrailsPolicyGVR = schema.GroupVersionResource{Group: "enterpriseagentgateway.solo.io", Version: "v1alpha1", Resource: "enterpriseagentgatewaypolicies"}

// guardrailsBasePolicyName is the guardrails-prompt-guard step's own policy -
// see internal/manifests/templates/guardrails/prompt-guard.yaml.tmpl.
const guardrailsBasePolicyName = "guardrails-prompt-guard"

// guardrailsMergePolicies are step IDs whose own manifest template still
// defines a full, standalone EnterpriseAgentgatewayPolicy (so its
// promptGuard rules can be extracted the same way as any other render), but
// must never be applied as that standalone object: agentgateway only
// enforces one promptGuard config per HTTPRoute target, so a second,
// independently targeted policy on guardrails-openai is accepted by the API
// (status: Attached) and then silently never enforced. Apply/Revert for
// these instead extend guardrails-prompt-guard's own
// promptGuard.request/response arrays.
var guardrailsMergePolicies = map[string]bool{
	"guardrails-banking-inbound":       true,
	"guardrails-banking-outbound-mask": true,
}

// extractPromptGuardRules reads the request/response promptGuard rule
// entries out of a rendered policy fragment - a full
// EnterpriseAgentgatewayPolicy document, as if it were being applied
// standalone - without applying it.
func extractPromptGuardRules(docs [][]byte) (request, response []any, err error) {
	if len(docs) == 0 {
		return nil, nil, fmt.Errorf("policy fragment rendered no documents")
	}

	var obj unstructured.Unstructured
	if err := yaml.Unmarshal(docs[0], &obj.Object); err != nil {
		return nil, nil, fmt.Errorf("parsing rendered policy fragment: %w", err)
	}

	request, _, err = unstructured.NestedSlice(obj.Object, "spec", "backend", "ai", "promptGuard", "request")
	if err != nil {
		return nil, nil, fmt.Errorf("reading request rules: %w", err)
	}
	response, _, err = unstructured.NestedSlice(obj.Object, "spec", "backend", "ai", "promptGuard", "response")
	if err != nil {
		return nil, nil, fmt.Errorf("reading response rules: %w", err)
	}
	return request, response, nil
}

// mergedGuardrailsPolicy builds the full guardrails-prompt-guard
// EnterpriseAgentgatewayPolicy object with ruleRequest/ruleResponse either
// appended to (add=true) or removed from (add=false) base's existing
// promptGuard.request/response arrays, preserving base's targetRefs and
// every other step's previously-merged rules untouched.
func mergedGuardrailsPolicy(base *unstructured.Unstructured, namespace string, ruleRequest, ruleResponse []any, add bool) (map[string]any, error) {
	targetRefs, _, err := unstructured.NestedSlice(base.Object, "spec", "targetRefs")
	if err != nil {
		return nil, fmt.Errorf("reading existing targetRefs: %w", err)
	}
	existingRequest, _, err := unstructured.NestedSlice(base.Object, "spec", "backend", "ai", "promptGuard", "request")
	if err != nil {
		return nil, fmt.Errorf("reading existing request rules: %w", err)
	}
	existingResponse, _, err := unstructured.NestedSlice(base.Object, "spec", "backend", "ai", "promptGuard", "response")
	if err != nil {
		return nil, fmt.Errorf("reading existing response rules: %w", err)
	}

	if add {
		// IsApplied is tracked in memory, so a wizard pod restart - or just a
		// repeat click - can call this again when the rules are already merged
		// in. Only append entries not already present, so re-applying is a
		// no-op instead of accumulating duplicate rules.
		existingRequest = append(existingRequest, newEntries(existingRequest, ruleRequest)...)
		existingResponse = append(existingResponse, newEntries(existingResponse, ruleResponse)...)
	} else {
		existingRequest = removeMatchingEntries(existingRequest, ruleRequest)
		existingResponse = removeMatchingEntries(existingResponse, ruleResponse)
	}

	return map[string]any{
		"apiVersion": "enterpriseagentgateway.solo.io/v1alpha1",
		"kind":       "EnterpriseAgentgatewayPolicy",
		"metadata": map[string]any{
			"name":      guardrailsBasePolicyName,
			"namespace": namespace,
			"labels":    map[string]any{"app.kubernetes.io/managed-by": "agentgateway-demo-wizard"},
		},
		"spec": map[string]any{
			"targetRefs": targetRefs,
			"backend": map[string]any{
				"ai": map[string]any{
					"promptGuard": map[string]any{
						"request":  existingRequest,
						"response": existingResponse,
					},
				},
			},
		},
	}, nil
}

// newEntries returns the entries in candidates not already deep-equal to one
// in existing.
func newEntries(existing, candidates []any) []any {
	var fresh []any
	for _, c := range candidates {
		present := false
		for _, e := range existing {
			if reflect.DeepEqual(e, c) {
				present = true
				break
			}
		}
		if !present {
			fresh = append(fresh, c)
		}
	}
	return fresh
}

// removeMatchingEntries returns existing with every entry deep-equal to one
// in toRemove dropped, leaving every other entry (another step's merged
// rules, or an unrelated one) untouched.
func removeMatchingEntries(existing, toRemove []any) []any {
	kept := make([]any, 0, len(existing))
	for _, e := range existing {
		matched := false
		for _, r := range toRemove {
			if reflect.DeepEqual(e, r) {
				matched = true
				break
			}
		}
		if !matched {
			kept = append(kept, e)
		}
	}
	return kept
}

// applyGuardrailsMerge extends guardrails-prompt-guard with policy's own
// promptGuard rules instead of applying policy's manifest as its own
// EnterpriseAgentgatewayPolicy - see guardrailsMergePolicies.
func (s *Server) applyGuardrailsMerge(ctx context.Context, policy scenarios.Policy, params scenarios.ManifestParams) ([]k8s.AppliedResource, error) {
	docs, err := policy.Render(params)
	if err != nil {
		return nil, err
	}
	newRequest, newResponse, err := extractPromptGuardRules(docs)
	if err != nil {
		return nil, err
	}

	base, err := s.applier.Get(ctx, guardrailsPolicyGVR, params.Namespace, guardrailsBasePolicyName)
	if err != nil {
		return nil, fmt.Errorf("the %q policy doesn't exist yet - apply \"Not everything should get through\" first: %w", guardrailsBasePolicyName, err)
	}

	merged, err := mergedGuardrailsPolicy(base, params.Namespace, newRequest, newResponse, true)
	if err != nil {
		return nil, err
	}
	doc, err := yaml.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("marshaling merged policy: %w", err)
	}
	return s.applier.ApplyPatch(ctx, policy.ID, [][]byte{doc})
}

// revertGuardrailsMerge removes policy's own rules (added by
// applyGuardrailsMerge) from guardrails-prompt-guard, leaving the base
// policy and every other step's merged rules untouched.
func (s *Server) revertGuardrailsMerge(ctx context.Context, policy scenarios.Policy, params scenarios.ManifestParams) error {
	docs, err := policy.Render(params)
	if err != nil {
		return err
	}
	oldRequest, oldResponse, err := extractPromptGuardRules(docs)
	if err != nil {
		return err
	}

	base, err := s.applier.Get(ctx, guardrailsPolicyGVR, params.Namespace, guardrailsBasePolicyName)
	if err != nil {
		return fmt.Errorf("the %q policy doesn't exist: %w", guardrailsBasePolicyName, err)
	}

	merged, err := mergedGuardrailsPolicy(base, params.Namespace, oldRequest, oldResponse, false)
	if err != nil {
		return err
	}
	doc, err := yaml.Marshal(merged)
	if err != nil {
		return fmt.Errorf("marshaling reverted policy: %w", err)
	}
	return s.applier.RevertPatch(ctx, policy.ID, [][]byte{doc})
}
