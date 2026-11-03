package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/day0ops/agentgateway-demo-wizard/internal/k8s"
	"github.com/day0ops/agentgateway-demo-wizard/internal/k8s/k8stest"
	"github.com/day0ops/agentgateway-demo-wizard/internal/scenarios"
)

func newApplierForGuardrailsTest() *k8s.Applier {
	dyn := k8stest.WithApplyFallback(dynamicfake.NewSimpleDynamicClient(scheme.Scheme))
	mapper := meta.NewDefaultRESTMapper(nil)
	mapper.Add(schema.GroupVersionKind{Group: "enterpriseagentgateway.solo.io", Version: "v1alpha1", Kind: "EnterpriseAgentgatewayPolicy"}, meta.RESTScopeNamespace)
	return k8s.NewApplier(dyn, mapper)
}

// basePromptGuardFixture mirrors guardrails-prompt-guard's own shape from
// internal/manifests/templates/guardrails/prompt-guard.yaml.tmpl, as if
// step 1 had already been applied.
const basePromptGuardFixture = `
apiVersion: enterpriseagentgateway.solo.io/v1alpha1
kind: EnterpriseAgentgatewayPolicy
metadata:
  name: guardrails-prompt-guard
  namespace: agentgateway-system
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      name: guardrails-openai
  backend:
    ai:
      promptGuard:
        request:
          - regex:
              action: Reject
              matches:
                - 'credit card'
              builtins:
                - Ssn
                - CreditCard
            response:
              message: 'Request rejected due to policy violation'
        response:
          - regex:
              action: Mask
              builtins:
                - Ssn
                - CreditCard
                - Email
`

func findGuardrailsPolicy(t *testing.T, id string) scenarios.Policy {
	t.Helper()
	for _, pillar := range scenarios.Registry() {
		for _, step := range pillar.Steps {
			for _, p := range step.Policies {
				if p.ID == id {
					return p
				}
			}
		}
	}
	t.Fatalf("policy %q not found in the registry", id)
	return scenarios.Policy{}
}

func TestApplyGuardrailsMergeExtendsBasePolicyRulesWithoutDroppingExisting(t *testing.T) {
	applier := newApplierForGuardrailsTest()
	ctx := context.Background()
	if _, err := applier.Apply(ctx, "guardrails-prompt-guard", [][]byte{[]byte(basePromptGuardFixture)}); err != nil {
		t.Fatalf("seeding base policy: %v", err)
	}

	s := &Server{applier: applier}
	policy := findGuardrailsPolicy(t, "guardrails-banking-inbound")
	params := scenarios.ManifestParams{Namespace: "agentgateway-system"}

	if _, err := s.applyGuardrailsMerge(ctx, policy, params); err != nil {
		t.Fatalf("applyGuardrailsMerge: %v", err)
	}

	base, err := applier.Get(ctx, guardrailsPolicyGVR, "agentgateway-system", "guardrails-prompt-guard")
	if err != nil {
		t.Fatalf("getting merged base policy: %v", err)
	}

	request, _, err := unstructured.NestedSlice(base.Object, "spec", "backend", "ai", "promptGuard", "request")
	if err != nil {
		t.Fatalf("reading merged request rules: %v", err)
	}
	if len(request) != 2 {
		t.Fatalf("expected the original rule plus banking-inbound's own rule, got %d: %+v", len(request), request)
	}
	joined := fmt.Sprintf("%+v", request)
	if !strings.Contains(joined, "credit card") {
		t.Fatalf("expected the original prompt-guard rule to survive the merge, got: %s", joined)
	}
	if !strings.Contains(joined, "another customer's account") {
		t.Fatalf("expected banking-inbound's own rule to be merged in, got: %s", joined)
	}

	targetRefs, _, err := unstructured.NestedSlice(base.Object, "spec", "targetRefs")
	if err != nil {
		t.Fatalf("reading targetRefs: %v", err)
	}
	if len(targetRefs) != 1 {
		t.Fatalf("expected targetRefs to survive the merge untouched, got: %+v", targetRefs)
	}
}

func TestApplyGuardrailsMergeErrorsWhenBasePolicyNotYetApplied(t *testing.T) {
	applier := newApplierForGuardrailsTest()
	s := &Server{applier: applier}
	policy := findGuardrailsPolicy(t, "guardrails-banking-inbound")
	params := scenarios.ManifestParams{Namespace: "agentgateway-system"}

	if _, err := s.applyGuardrailsMerge(context.Background(), policy, params); err == nil {
		t.Fatal("expected an error when guardrails-prompt-guard hasn't been applied yet")
	} else if !strings.Contains(err.Error(), "Not everything should get through") {
		t.Fatalf("expected the error to point at the prerequisite step, got: %v", err)
	}
}

func TestRevertGuardrailsMergeRemovesOnlyThisStepsRules(t *testing.T) {
	applier := newApplierForGuardrailsTest()
	ctx := context.Background()
	if _, err := applier.Apply(ctx, "guardrails-prompt-guard", [][]byte{[]byte(basePromptGuardFixture)}); err != nil {
		t.Fatalf("seeding base policy: %v", err)
	}

	s := &Server{applier: applier}
	params := scenarios.ManifestParams{Namespace: "agentgateway-system"}
	inbound := findGuardrailsPolicy(t, "guardrails-banking-inbound")
	outbound := findGuardrailsPolicy(t, "guardrails-banking-outbound-mask")

	if _, err := s.applyGuardrailsMerge(ctx, inbound, params); err != nil {
		t.Fatalf("applying banking-inbound: %v", err)
	}
	if _, err := s.applyGuardrailsMerge(ctx, outbound, params); err != nil {
		t.Fatalf("applying banking-outbound-mask: %v", err)
	}

	if err := s.revertGuardrailsMerge(ctx, inbound, params); err != nil {
		t.Fatalf("revertGuardrailsMerge: %v", err)
	}

	base, err := applier.Get(ctx, guardrailsPolicyGVR, "agentgateway-system", "guardrails-prompt-guard")
	if err != nil {
		t.Fatalf("getting policy after revert: %v", err)
	}
	request, _, err := unstructured.NestedSlice(base.Object, "spec", "backend", "ai", "promptGuard", "request")
	if err != nil {
		t.Fatalf("reading request rules after revert: %v", err)
	}
	joined := fmt.Sprintf("%+v", request)
	if strings.Contains(joined, "another customer's account") {
		t.Fatalf("expected banking-inbound's rule to be removed, got: %s", joined)
	}
	if !strings.Contains(joined, "credit card") {
		t.Fatalf("expected the base prompt-guard rule to survive, got: %s", joined)
	}

	response, _, err := unstructured.NestedSlice(base.Object, "spec", "backend", "ai", "promptGuard", "response")
	if err != nil {
		t.Fatalf("reading response rules after revert: %v", err)
	}
	responseJoined := fmt.Sprintf("%+v", response)
	if !strings.Contains(responseJoined, "\\d{12}") {
		t.Fatalf("expected banking-outbound-mask's rule (reverted separately) to survive, got: %s", responseJoined)
	}
}

func TestConfigApplyUsesGuardrailsMergeForBankingPolicies(t *testing.T) {
	applier := newApplierForGuardrailsTest()
	ctx := context.Background()
	if _, err := applier.Apply(ctx, "guardrails-prompt-guard", [][]byte{[]byte(basePromptGuardFixture)}); err != nil {
		t.Fatalf("seeding base policy: %v", err)
	}

	s := NewServer(WithScenarios(scenarios.Registry()), WithApplier(applier))

	req := httptest.NewRequest(http.MethodPost, "/api/config/apply", strings.NewReader(`{"policyId":"guardrails-banking-inbound"}`))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"type":"done"`) {
		t.Fatalf("expected a done event, got: %s", rec.Body.String())
	}
	if !applier.IsApplied("guardrails-banking-inbound") {
		t.Fatalf("expected guardrails-banking-inbound to be marked applied")
	}

	revertReq := httptest.NewRequest(http.MethodPost, "/api/config/revert", strings.NewReader(`{"policyId":"guardrails-banking-inbound"}`))
	revertRec := httptest.NewRecorder()
	s.ServeHTTP(revertRec, revertReq)
	if revertRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on revert, got %d: %s", revertRec.Code, revertRec.Body.String())
	}
	if applier.IsApplied("guardrails-banking-inbound") {
		t.Fatalf("expected guardrails-banking-inbound to no longer be marked applied after revert")
	}
	if !applier.IsApplied("guardrails-prompt-guard") {
		t.Fatalf("expected the base policy to remain applied - revert must not delete it")
	}
}

// TestApplyGuardrailsMergeIsIdempotent covers a real scenario: IsApplied is
// tracked in memory, so a wizard pod restart (or a repeat click) can call
// Apply again when the rules are already merged in - it must not
// accumulate a duplicate copy of the same rule.
func TestApplyGuardrailsMergeIsIdempotent(t *testing.T) {
	applier := newApplierForGuardrailsTest()
	ctx := context.Background()
	if _, err := applier.Apply(ctx, "guardrails-prompt-guard", [][]byte{[]byte(basePromptGuardFixture)}); err != nil {
		t.Fatalf("seeding base policy: %v", err)
	}

	s := &Server{applier: applier}
	policy := findGuardrailsPolicy(t, "guardrails-banking-inbound")
	params := scenarios.ManifestParams{Namespace: "agentgateway-system"}

	if _, err := s.applyGuardrailsMerge(ctx, policy, params); err != nil {
		t.Fatalf("first applyGuardrailsMerge: %v", err)
	}
	if _, err := s.applyGuardrailsMerge(ctx, policy, params); err != nil {
		t.Fatalf("second applyGuardrailsMerge: %v", err)
	}

	base, err := applier.Get(ctx, guardrailsPolicyGVR, "agentgateway-system", "guardrails-prompt-guard")
	if err != nil {
		t.Fatalf("getting merged base policy: %v", err)
	}
	request, _, err := unstructured.NestedSlice(base.Object, "spec", "backend", "ai", "promptGuard", "request")
	if err != nil {
		t.Fatalf("reading merged request rules: %v", err)
	}
	if len(request) != 2 {
		t.Fatalf("expected re-applying to be a no-op (1 original + 1 banking-inbound rule), got %d: %+v", len(request), request)
	}
}
