package scenarios

import (
	"strings"
	"testing"
)

func TestRegistryIncludesPillar2Auth(t *testing.T) {
	reg := Registry()
	if len(reg) != 7 {
		t.Fatalf("expected 7 pillars (intro, routing, auth, guardrails, cost, agent, wrap), got %d", len(reg))
	}

	auth := reg[2]
	if auth.ID != "auth" {
		t.Fatalf("expected pillar[2].ID 'auth', got %q", auth.ID)
	}
	if len(auth.Steps) != 2 {
		t.Fatalf("expected 2 auth steps, got %d", len(auth.Steps))
	}

	wantStepIDs := []string{"auth-jwt", "auth-rbac"}
	for i, want := range wantStepIDs {
		if auth.Steps[i].ID != want {
			t.Fatalf("expected step[%d].ID %q, got %q", i, want, auth.Steps[i].ID)
		}
	}
}

func TestAuthPoliciesRenderValidManifests(t *testing.T) {
	params := ManifestParams{
		Namespace:    "agentgateway-system",
		GatewayName:  "agentgateway-gw",
		GatewayNS:    "agentgateway-system",
		KeycloakHost: "keycloak.demo.example.com",
	}

	auth := Registry()[2]

	for _, step := range auth.Steps {
		if len(step.Policies) == 0 {
			t.Fatalf("step %q has no policies", step.ID)
		}
		for _, policy := range step.Policies {
			docs, err := policy.Render(params)
			if err != nil {
				t.Fatalf("rendering policy %q: %v", policy.ID, err)
			}
			if len(docs) == 0 {
				t.Fatalf("policy %q rendered no documents", policy.ID)
			}
			joined := strings.Join(toStrings(docs), "\n---\n")
			if !strings.Contains(joined, "jwtAuthentication") {
				t.Fatalf("policy %q: expected jwtAuthentication, got: %s", policy.ID, joined)
			}
			if strings.Contains(joined, "{{") {
				t.Fatalf("policy %q: unresolved template variable in output: %s", policy.ID, joined)
			}
			if !strings.Contains(joined, params.KeycloakHost) {
				t.Fatalf("policy %q: expected the rendered Keycloak host %q to appear, got: %s", policy.ID, params.KeycloakHost, joined)
			}
		}
	}

	if len(auth.Steps) < 2 || len(auth.Steps[1].Policies) == 0 {
		t.Fatalf("expected auth.Steps[1] to have at least one policy")
	}
	rbacDocs, err := auth.Steps[1].Policies[0].Render(params)
	if err != nil {
		t.Fatalf("rendering auth-rbac: %v", err)
	}
	if len(rbacDocs) == 0 {
		t.Fatalf("policy %q rendered no documents", auth.Steps[1].Policies[0].ID)
	}
	if !strings.Contains(strings.Join(toStrings(rbacDocs), "\n"), "authorization") {
		t.Fatalf("expected the auth-rbac policy to contain an authorization block")
	}
}

func toStrings(docs [][]byte) []string {
	out := make([]string, len(docs))
	for i, d := range docs {
		out[i] = string(d)
	}
	return out
}
