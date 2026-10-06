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
	if len(guardrails.Steps) != 1 {
		t.Fatalf("expected 1 guardrails step, got %d", len(guardrails.Steps))
	}
	if guardrails.Steps[0].ID != "guardrails-prompt-guard" {
		t.Fatalf("expected step[0].ID 'guardrails-prompt-guard', got %q", guardrails.Steps[0].ID)
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
