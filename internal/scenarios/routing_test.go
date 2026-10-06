package scenarios

import (
	"strings"
	"testing"
)

func TestRegistryIncludesPillar1Routing(t *testing.T) {
	reg := Registry()
	if len(reg) != 7 {
		t.Fatalf("expected 7 pillars (intro, routing, auth, guardrails, cost, agent, wrap), got %d", len(reg))
	}

	routing := reg[1]
	if routing.ID != "routing" {
		t.Fatalf("expected pillar[1].ID 'routing', got %q", routing.ID)
	}
	if len(routing.Steps) != 4 {
		t.Fatalf("expected 4 routing steps, got %d", len(routing.Steps))
	}

	wantStepIDs := []string{"routing-by-name", "routing-dynamic", "routing-fallback", "routing-streaming"}
	for i, want := range wantStepIDs {
		if routing.Steps[i].ID != want {
			t.Fatalf("expected step[%d].ID %q, got %q", i, want, routing.Steps[i].ID)
		}
	}
}

func TestRoutingPoliciesRenderValidManifests(t *testing.T) {
	params := ManifestParams{
		Namespace:    "agentgateway-system",
		GatewayName:  "agentgateway-gw",
		GatewayNS:    "agentgateway-system",
		OpenAIKey:    "sk-test",
		AnthropicKey: "sk-ant-test",
	}

	routing := Registry()[1]

	for _, step := range routing.Steps {
		if len(step.Policies) == 0 {
			continue // routing-streaming reuses routing-by-name's already-applied policy
		}
		for _, policy := range step.Policies {
			docs, err := policy.Render(params)
			if err != nil {
				t.Fatalf("rendering policy %q: %v", policy.ID, err)
			}
			if len(docs) == 0 {
				t.Fatalf("policy %q rendered no documents", policy.ID)
			}

			joined := string(docs[0])
			for _, d := range docs[1:] {
				joined += "\n---\n" + string(d)
			}
			if !strings.Contains(joined, "EnterpriseAgentgatewayBackend") {
				t.Fatalf("policy %q: expected an EnterpriseAgentgatewayBackend, got: %s", policy.ID, joined)
			}
			if !strings.Contains(joined, "HTTPRoute") {
				t.Fatalf("policy %q: expected an HTTPRoute, got: %s", policy.ID, joined)
			}
			if strings.Contains(joined, "{{") {
				t.Fatalf("policy %q: unresolved template variable in output: %s", policy.ID, joined)
			}
			if !strings.Contains(joined, params.Namespace) {
				t.Fatalf("policy %q: expected the rendered namespace %q to appear, got: %s", policy.ID, params.Namespace, joined)
			}
			if !strings.Contains(joined, params.GatewayName) {
				t.Fatalf("policy %q: expected the rendered gateway name %q to appear, got: %s", policy.ID, params.GatewayName, joined)
			}
		}
	}
}
