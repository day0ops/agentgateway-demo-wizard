package scenarios

import (
	"strings"
	"testing"
)

func TestRegistryIncludesPillar3Cost(t *testing.T) {
	reg := Registry()
	if len(reg) != 7 {
		t.Fatalf("expected 7 pillars (intro, routing, auth, guardrails, cost, agent, wrap), got %d", len(reg))
	}

	cost := reg[4]
	if cost.ID != "cost" {
		t.Fatalf("expected pillar[4].ID 'cost', got %q", cost.ID)
	}
	if len(cost.Steps) != 2 {
		t.Fatalf("expected 2 cost steps, got %d", len(cost.Steps))
	}

	wantStepIDs := []string{"cost-attribution", "cost-budgets"}
	for i, want := range wantStepIDs {
		if cost.Steps[i].ID != want {
			t.Fatalf("expected step[%d].ID %q, got %q", i, want, cost.Steps[i].ID)
		}
	}
}

func TestCostAttributionRendersBothManifestAndPatch(t *testing.T) {
	params := ManifestParams{
		Namespace:   "agentgateway-system",
		GatewayName: "agentgateway-gw",
		GatewayNS:   "agentgateway-system",
		OpenAIKey:   "sk-test",
	}

	policy := Registry()[4].Steps[0].Policies[0]

	docs, err := policy.Render(params)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	joined := strings.Join(toStrings(docs), "\n")
	if !strings.Contains(joined, "EnterpriseAgentgatewayParameters") {
		t.Fatalf("expected the generic manifest to include EnterpriseAgentgatewayParameters")
	}
	if !strings.Contains(joined, params.Namespace) {
		t.Fatalf("expected the rendered namespace %q to appear, got: %s", params.Namespace, joined)
	}
	if !strings.Contains(joined, params.OpenAIKey) {
		t.Fatalf("expected the rendered OpenAI key %q to appear, got: %s", params.OpenAIKey, joined)
	}
	if strings.Contains(joined, "{{") {
		t.Fatalf("unresolved template variable in output: %s", joined)
	}

	patchDocs, err := policy.RenderPatch(params)
	if err != nil {
		t.Fatalf("RenderPatch: %v", err)
	}
	patchJoined := strings.Join(toStrings(patchDocs), "\n")
	if !strings.Contains(patchJoined, "parametersRef") {
		t.Fatalf("expected the patch manifest to set parametersRef")
	}
	if !strings.Contains(patchJoined, params.GatewayName) {
		t.Fatalf("expected the rendered gateway name %q to appear, got: %s", params.GatewayName, patchJoined)
	}
	if strings.Contains(patchJoined, "{{") {
		t.Fatalf("unresolved template variable in output: %s", patchJoined)
	}

	revertDocs, err := policy.RenderPatchRevert(params)
	if err != nil {
		t.Fatalf("RenderPatchRevert: %v", err)
	}
	revertJoined := strings.Join(toStrings(revertDocs), "\n")
	if strings.Contains(revertJoined, "parametersRef") {
		t.Fatalf("expected the revert manifest to omit parametersRef")
	}
	if !strings.Contains(revertJoined, params.GatewayName) {
		t.Fatalf("expected the rendered gateway name %q to appear, got: %s", params.GatewayName, revertJoined)
	}
	if strings.Contains(revertJoined, "{{") {
		t.Fatalf("unresolved template variable in output: %s", revertJoined)
	}
}

func TestCostBudgetsPolicyMergesTrafficFragments(t *testing.T) {
	params := ManifestParams{
		Namespace:   "agentgateway-system",
		GatewayName: "agentgateway-gw",
		GatewayNS:   "agentgateway-system",
		OpenAIKey:   "sk-test",
	}

	policy := Registry()[4].Steps[1].Policies[0]
	docs, err := policy.Render(params)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	joined := strings.Join(toStrings(docs), "\n")
	for _, want := range []string{"apiKeyAuthentication", "entRateLimit", "entBudgetEnforcement"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected the rendered policy to contain %q, got: %s", want, joined)
		}
	}
	if !strings.Contains(joined, params.Namespace) {
		t.Fatalf("expected the rendered namespace %q to appear, got: %s", params.Namespace, joined)
	}
	if !strings.Contains(joined, params.GatewayName) {
		t.Fatalf("expected the rendered gateway name %q to appear, got: %s", params.GatewayName, joined)
	}
	if !strings.Contains(joined, params.OpenAIKey) {
		t.Fatalf("expected the rendered OpenAI key %q to appear, got: %s", params.OpenAIKey, joined)
	}
	if strings.Contains(joined, "{{") {
		t.Fatalf("unresolved template variable in output: %s", joined)
	}
}
