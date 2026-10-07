package scenarios

import "testing"

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

func TestRoutingPoliciesAreReadOnlyAndNeverApply(t *testing.T) {
	routing := Registry()[1]

	wantViewRefNames := map[string][]string{
		"routing-by-name":  {"routing-by-name-openai", "routing-by-name-anthropic"},
		"routing-dynamic":  {"routing-dynamic-fast", "routing-dynamic-premium"},
		"routing-fallback": {"routing-fallback"},
	}

	for _, step := range routing.Steps {
		want, hasPolicy := wantViewRefNames[step.ID]
		if !hasPolicy {
			continue // routing-streaming has no policy of its own
		}
		if len(step.Policies) != 1 {
			t.Fatalf("step %q: expected exactly 1 policy, got %d", step.ID, len(step.Policies))
		}
		p := step.Policies[0]

		if !p.ReadOnly {
			t.Fatalf("step %q: expected policy to be ReadOnly", step.ID)
		}
		if p.ManifestPath != "" {
			t.Fatalf("step %q: expected empty ManifestPath on a read-only policy, got %q", step.ID, p.ManifestPath)
		}

		if len(p.ViewRefs) != len(want) {
			t.Fatalf("step %q: expected %d ViewRefs, got %d", step.ID, len(want), len(p.ViewRefs))
		}
		for i, name := range want {
			if p.ViewRefs[i].Name != name {
				t.Fatalf("step %q: ViewRefs[%d].Name = %q, want %q", step.ID, i, p.ViewRefs[i].Name, name)
			}
			if p.ViewRefs[i].GVR.Resource != "enterpriseagentgatewaybackends" {
				t.Fatalf("step %q: ViewRefs[%d].GVR.Resource = %q, want enterpriseagentgatewaybackends", step.ID, i, p.ViewRefs[i].GVR.Resource)
			}
		}

		docs, err := p.Render(ManifestParams{Namespace: "agentgateway-system"})
		if err != nil {
			t.Fatalf("step %q: Render errored: %v", step.ID, err)
		}
		if docs != nil {
			t.Fatalf("step %q: expected a read-only policy to render nothing, got %v", step.ID, docs)
		}
	}
}
