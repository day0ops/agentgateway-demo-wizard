package scenarios

import (
	"strings"
	"testing"
)

func TestRegistryIncludesGuardrailsPillar(t *testing.T) {
	reg := Registry()
	if len(reg) != 7 {
		t.Fatalf("expected 7 pillars (intro, routing, auth, guardrails, cost, agent, wrap), got %d", len(reg))
	}

	guardrails := reg[3]
	if guardrails.ID != "guardrails" {
		t.Fatalf("expected pillar[3].ID 'guardrails', got %q", guardrails.ID)
	}
	if len(guardrails.Steps) != 3 {
		t.Fatalf("expected 3 guardrails steps, got %d", len(guardrails.Steps))
	}
	if guardrails.Steps[0].ID != "guardrails-prompt-guard" {
		t.Fatalf("expected step[0].ID 'guardrails-prompt-guard', got %q", guardrails.Steps[0].ID)
	}
	if guardrails.Steps[1].ID != "guardrails-banking-inbound" {
		t.Fatalf("expected step[1].ID 'guardrails-banking-inbound', got %q", guardrails.Steps[1].ID)
	}
	if len(guardrails.Steps[1].Policies) != 1 {
		t.Fatalf("expected 1 policy on the banking-inbound step, got %d", len(guardrails.Steps[1].Policies))
	}
	if got := guardrails.Steps[1].Policies[0].ManifestPath; got != "guardrails/banking-inbound.yaml.tmpl" {
		t.Fatalf("expected banking-inbound step's policy ManifestPath 'guardrails/banking-inbound.yaml.tmpl', got %q", got)
	}
	if len(guardrails.Steps[1].Presets) < 2 {
		t.Fatalf("expected at least 2 presets (a safe ask, a prohibited ask) on the banking-inbound step, got %d", len(guardrails.Steps[1].Presets))
	}
	if guardrails.Steps[2].ID != "guardrails-banking-outbound-mask" {
		t.Fatalf("expected step[2].ID 'guardrails-banking-outbound-mask', got %q", guardrails.Steps[2].ID)
	}
	if !strings.Contains(guardrails.Steps[2].Explanation, "The caller was fully authorized. The gateway still won't let the full account number leave.") {
		t.Fatalf("expected banking-outbound-mask step's explanation to carry the pillar bridge line, got: %s", guardrails.Steps[2].Explanation)
	}
	if len(guardrails.Steps[2].Policies) != 1 {
		t.Fatalf("expected 1 policy on the banking-outbound-mask step, got %d", len(guardrails.Steps[2].Policies))
	}
	if got := guardrails.Steps[2].Policies[0].ManifestPath; got != "guardrails/banking-outbound-mask.yaml.tmpl" {
		t.Fatalf("expected banking-outbound-mask step's policy ManifestPath 'guardrails/banking-outbound-mask.yaml.tmpl', got %q", got)
	}
	if len(guardrails.Steps[2].Presets) < 1 {
		t.Fatalf("expected at least 1 preset (an authorized ask whose answer contains a full number) on the banking-outbound-mask step, got %d", len(guardrails.Steps[2].Presets))
	}
}

func TestBankingInboundPolicyRendersValidManifest(t *testing.T) {
	params := ManifestParams{
		Namespace:   "agentgateway-system",
		GatewayName: "agentgateway-gw",
		GatewayNS:   "agentgateway-system",
		OpenAIKey:   "sk-test",
	}

	guardrails := Registry()[3]
	policy := guardrails.Steps[1].Policies[0]

	docs, err := policy.Render(params)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	// This policy must own exactly one resource: the EnterpriseAgentgatewayPolicy
	// itself. It targets the HTTPRoute the prompt-guard step's policy already
	// creates - it must never redeclare that Backend/Secret/HTTPRoute, since
	// Revert deletes exactly what a policy's own Apply created, and
	// redeclaring shared resources here would let reverting this step delete
	// resources the other step still depends on.
	if len(docs) != 1 {
		t.Fatalf("expected exactly 1 document (the EnterpriseAgentgatewayPolicy only), got %d", len(docs))
	}
	joined := strings.Join(toStrings(docs), "\n---\n")

	for _, want := range []string{"EnterpriseAgentgatewayPolicy", "promptGuard", "guardrails-openai"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected the rendered manifest to contain %q, got: %s", want, joined)
		}
	}
	for _, unwanted := range []string{"kind: Secret", "kind: EnterpriseAgentgatewayBackend"} {
		if strings.Contains(joined, unwanted) {
			t.Fatalf("expected the rendered manifest NOT to contain %q (that's the prompt-guard step's resource), got: %s", unwanted, joined)
		}
	}
	if strings.Contains(joined, "{{") {
		t.Fatalf("unresolved template variable in output: %s", joined)
	}
	if !strings.Contains(joined, params.Namespace) {
		t.Fatalf("expected the rendered namespace %q to appear, got: %s", params.Namespace, joined)
	}
}

func TestBankingOutboundMaskPolicyRendersValidManifest(t *testing.T) {
	params := ManifestParams{
		Namespace:   "agentgateway-system",
		GatewayName: "agentgateway-gw",
		GatewayNS:   "agentgateway-system",
		OpenAIKey:   "sk-test",
	}

	guardrails := Registry()[3]
	policy := guardrails.Steps[2].Policies[0]

	docs, err := policy.Render(params)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	// Same isolation reason as the banking-inbound step: this policy owns only
	// the EnterpriseAgentgatewayPolicy and targets the prompt-guard step's
	// HTTPRoute rather than redeclaring it, so Revert never touches resources
	// the other steps still depend on.
	if len(docs) != 1 {
		t.Fatalf("expected exactly 1 document (the EnterpriseAgentgatewayPolicy only), got %d", len(docs))
	}
	joined := strings.Join(toStrings(docs), "\n---\n")

	for _, want := range []string{"EnterpriseAgentgatewayPolicy", "promptGuard", "guardrails-openai", "response:"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected the rendered manifest to contain %q, got: %s", want, joined)
		}
	}
	for _, unwanted := range []string{"kind: Secret", "kind: EnterpriseAgentgatewayBackend"} {
		if strings.Contains(joined, unwanted) {
			t.Fatalf("expected the rendered manifest NOT to contain %q (that's the prompt-guard step's resource), got: %s", unwanted, joined)
		}
	}
	if strings.Contains(joined, "{{") {
		t.Fatalf("unresolved template variable in output: %s", joined)
	}
	if !strings.Contains(joined, params.Namespace) {
		t.Fatalf("expected the rendered namespace %q to appear, got: %s", params.Namespace, joined)
	}
}

func TestGuardrailsPolicyRendersValidManifest(t *testing.T) {
	params := ManifestParams{
		Namespace:   "agentgateway-system",
		GatewayName: "agentgateway-gw",
		GatewayNS:   "agentgateway-system",
		OpenAIKey:   "sk-test",
	}

	guardrails := Registry()[3]
	policy := guardrails.Steps[0].Policies[0]

	docs, err := policy.Render(params)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	joined := strings.Join(toStrings(docs), "\n---\n")

	for _, want := range []string{"EnterpriseAgentgatewayBackend", "HTTPRoute", "promptGuard", "CreditCard", "Ssn"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected the rendered manifest to contain %q, got: %s", want, joined)
		}
	}
	if strings.Contains(joined, "{{") {
		t.Fatalf("unresolved template variable in output: %s", joined)
	}
	if !strings.Contains(joined, params.Namespace) {
		t.Fatalf("expected the rendered namespace %q to appear, got: %s", params.Namespace, joined)
	}
	if !strings.Contains(joined, params.OpenAIKey) {
		t.Fatalf("expected the rendered OpenAI key %q to appear, got: %s", params.OpenAIKey, joined)
	}
}
