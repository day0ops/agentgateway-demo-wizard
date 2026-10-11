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
	if len(auth.Steps) != 8 {
		t.Fatalf("expected 8 auth steps, got %d", len(auth.Steps))
	}

	wantStepIDs := []string{
		"auth-login", "auth-rbac", "auth-exchange-standard", "auth-exchange-sts",
		"auth-agent-impersonation", "auth-agent-delegation", "auth-agent-chain", "auth-agent-elicitation",
	}
	for i, want := range wantStepIDs {
		if auth.Steps[i].ID != want {
			t.Fatalf("expected step[%d].ID %q, got %q", i, want, auth.Steps[i].ID)
		}
	}
}

func TestAuthLoginStepHasNoPoliciesOrPresets(t *testing.T) {
	auth := Registry()[2]
	login := auth.Steps[0]

	if !login.LoginDemo {
		t.Fatalf("expected the login step to set LoginDemo")
	}
	if len(login.Policies) != 0 {
		t.Fatalf("expected the login step to have no policies, got %d", len(login.Policies))
	}
	if len(login.Presets) != 0 {
		t.Fatalf("expected the login step to have no presets, got %d", len(login.Presets))
	}
}

func TestAuthPoliciesRenderValidManifests(t *testing.T) {
	params := ManifestParams{
		Namespace:    "agentgateway-system",
		GatewayName:  "agentgateway-gw",
		GatewayNS:    "agentgateway-system",
		KeycloakHost: "keycloak.demo.example.com",
		STSHost:      "sts.demo.example.com",
		STSPort:      "7777",
	}

	auth := Registry()[2]

	for _, step := range auth.Steps {
		if step.LoginDemo || step.OpsAgent != nil {
			continue // no templated manifest to render - see TestAuthOpsAgentStepsAreReadOnlyAndNeverApply
		}
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
		}
	}

	rbac := findStep(auth.Steps, "auth-rbac")
	rbacDocs, err := rbac.Policies[0].Render(params)
	if err != nil {
		t.Fatalf("rendering auth-rbac: %v", err)
	}
	if !strings.Contains(strings.Join(toStrings(rbacDocs), "\n"), "authorization") {
		t.Fatalf("expected the auth-rbac policy to contain an authorization block")
	}

	standard := findStep(auth.Steps, "auth-exchange-standard")
	standardDocs, err := standard.Policies[0].Render(params)
	if err != nil {
		t.Fatalf("rendering auth-exchange-standard: %v", err)
	}
	standardJoined := strings.Join(toStrings(standardDocs), "\n")
	if !strings.Contains(standardJoined, "oauthTokenExchange") {
		t.Fatalf("expected the standard-exchange policy to contain oauthTokenExchange")
	}
	if !strings.Contains(standardJoined, params.KeycloakHost) {
		t.Fatalf("expected the standard-exchange policy to reference the Keycloak host")
	}

	sts := findStep(auth.Steps, "auth-exchange-sts")
	stsDocs, err := sts.Policies[0].Render(params)
	if err != nil {
		t.Fatalf("rendering auth-exchange-sts: %v", err)
	}
	stsJoined := strings.Join(toStrings(stsDocs), "\n")
	if !strings.Contains(stsJoined, params.STSHost) {
		t.Fatalf("expected the STS-exchange policy to reference the STS host, got: %s", stsJoined)
	}
	if !strings.Contains(stsJoined, params.STSPort) {
		t.Fatalf("expected the STS-exchange policy to reference the STS port, got: %s", stsJoined)
	}
}

func TestAuthOpsAgentStepsAreReadOnlyAndNeverApply(t *testing.T) {
	auth := Registry()[2]

	wantViewRefNames := map[string][]string{
		"auth-agent-impersonation": {"repo-agent", "repo-mcp-standard"},
		"auth-agent-delegation":    {"repo-agent", "repo-mcp-delegation"},
		"auth-agent-chain":         {"release-manager-agent", "deploy-agent", "repo-mcp-delegation"},
		"auth-agent-elicitation":   {"github-agent", "mcp-github"},
	}
	wantAgent := map[string]string{
		"auth-agent-impersonation": "repo",
		"auth-agent-delegation":    "repo",
		"auth-agent-chain":         "release-manager",
		"auth-agent-elicitation":   "github",
	}
	wantMode := map[string]string{
		"auth-agent-impersonation": "standard",
		"auth-agent-delegation":    "sts-delegation",
		"auth-agent-chain":         "",
		"auth-agent-elicitation":   "",
	}

	for _, step := range auth.Steps {
		want, hasPolicy := wantViewRefNames[step.ID]
		if !hasPolicy {
			continue
		}
		if step.OpsAgent == nil {
			t.Fatalf("step %q: expected OpsAgent to be set", step.ID)
		}
		if step.OpsAgent.Agent != wantAgent[step.ID] {
			t.Fatalf("step %q: OpsAgent.Agent = %q, want %q", step.ID, step.OpsAgent.Agent, wantAgent[step.ID])
		}
		if step.OpsAgent.Mode != wantMode[step.ID] {
			t.Fatalf("step %q: OpsAgent.Mode = %q, want %q", step.ID, step.OpsAgent.Mode, wantMode[step.ID])
		}
		if step.OpsAgent.DefaultPrompt == "" {
			t.Fatalf("step %q: expected a non-empty DefaultPrompt", step.ID)
		}

		if len(step.Policies) != 1 {
			t.Fatalf("step %q: expected exactly 1 policy, got %d", step.ID, len(step.Policies))
		}
		p := step.Policies[0]
		if !p.ReadOnly {
			t.Fatalf("step %q: expected policy to be ReadOnly", step.ID)
		}
		if len(p.ViewRefs) != len(want) {
			t.Fatalf("step %q: expected %d ViewRefs, got %d", step.ID, len(want), len(p.ViewRefs))
		}
		for i, name := range want {
			if p.ViewRefs[i].Name != name {
				t.Fatalf("step %q: ViewRefs[%d].Name = %q, want %q", step.ID, i, p.ViewRefs[i].Name, name)
			}
			if p.ViewRefs[i].GVR.Resource != "httproutes" {
				t.Fatalf("step %q: ViewRefs[%d].GVR.Resource = %q, want httproutes", step.ID, i, p.ViewRefs[i].GVR.Resource)
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

func findStep(steps []Step, id string) Step {
	for _, s := range steps {
		if s.ID == id {
			return s
		}
	}
	return Step{}
}

func toStrings(docs [][]byte) []string {
	out := make([]string, len(docs))
	for i, d := range docs {
		out[i] = string(d)
	}
	return out
}
