package scenarios

import (
	"strings"
	"testing"
)

func TestRegistryIncludesPillar4Agent(t *testing.T) {
	reg := Registry()
	if len(reg) != 7 {
		t.Fatalf("expected 7 pillars (intro, routing, auth, guardrails, cost, agent, wrap), got %d", len(reg))
	}

	agentPillarResult := reg[5]
	if agentPillarResult.ID != "agent" {
		t.Fatalf("expected pillar[5].ID 'agent', got %q", agentPillarResult.ID)
	}
	if len(agentPillarResult.Steps) != 4 {
		t.Fatalf("expected 4 agent steps, got %d", len(agentPillarResult.Steps))
	}

	wantStepIDs := []string{"agent-mcp-server", "agent-mcp-auth", "agent-token-exchange", "agent-mcp-tool-access"}
	for i, want := range wantStepIDs {
		if agentPillarResult.Steps[i].ID != want {
			t.Fatalf("expected step[%d].ID %q, got %q", i, want, agentPillarResult.Steps[i].ID)
		}
	}

	if !agentPillarResult.Steps[0].AgentDemo {
		t.Fatalf("expected agent-mcp-server.AgentDemo to be true")
	}
	if agentPillarResult.Steps[1].AgentDemo || agentPillarResult.Steps[2].AgentDemo || agentPillarResult.Steps[3].AgentDemo {
		t.Fatalf("expected only agent-mcp-server to be an agent demo step")
	}
}

func TestAgentPoliciesRenderValidManifests(t *testing.T) {
	params := ManifestParams{
		Namespace:                      "agentgateway-system",
		GatewayName:                    "agentgateway-gw",
		GatewayNS:                      "agentgateway-system",
		GatewayHost:                    "agentgateway.demo.example.com",
		KeycloakHost:                   "keycloak.demo.example.com",
		OpenAIKey:                      "sk-test",
		StockMCPImage:                  "example.com/stock:0.1.1",
		CurrencyMCPImage:               "example.com/currency:0.1.1",
		OAuthTokenExchangeClientSecret: "test-oauth-exchange-secret",
	}

	agentPillarResult := Registry()[5]

	serverDocs, err := agentPillarResult.Steps[0].Policies[0].Render(params)
	if err != nil {
		t.Fatalf("rendering agent-mcp-server: %v", err)
	}
	serverJoined := strings.Join(toStrings(serverDocs), "\n")
	for _, want := range []string{params.Namespace, params.StockMCPImage, params.CurrencyMCPImage, "agent-mcp-backend", "/agent-chat"} {
		if !strings.Contains(serverJoined, want) {
			t.Fatalf("expected agent-mcp-server manifest to contain %q, got: %s", want, serverJoined)
		}
	}
	if strings.Contains(serverJoined, "{{") {
		t.Fatalf("unresolved template variable in output: %s", serverJoined)
	}

	authDocs, err := agentPillarResult.Steps[1].Policies[0].Render(params)
	if err != nil {
		t.Fatalf("rendering agent-mcp-auth: %v", err)
	}
	authJoined := strings.Join(toStrings(authDocs), "\n")
	for _, want := range []string{"jwtAuthentication", params.Namespace, params.KeycloakHost, params.GatewayHost} {
		if !strings.Contains(authJoined, want) {
			t.Fatalf("expected agent-mcp-auth manifest to contain %q, got: %s", want, authJoined)
		}
	}
	if strings.Contains(authJoined, "{{") {
		t.Fatalf("unresolved template variable in output: %s", authJoined)
	}

	tokenExchangeDocs, err := agentPillarResult.Steps[2].Policies[0].Render(params)
	if err != nil {
		t.Fatalf("rendering agent-token-exchange: %v", err)
	}
	tokenExchangeJoined := strings.Join(toStrings(tokenExchangeDocs), "\n")
	for _, want := range []string{"oauthTokenExchange", "agw-token-exchange", params.KeycloakHost, params.OAuthTokenExchangeClientSecret} {
		if !strings.Contains(tokenExchangeJoined, want) {
			t.Fatalf("expected agent-token-exchange manifest to contain %q, got: %s", want, tokenExchangeJoined)
		}
	}
	if strings.Contains(tokenExchangeJoined, "{{") {
		t.Fatalf("unresolved template variable in output: %s", tokenExchangeJoined)
	}

	toolAccessDocs, err := agentPillarResult.Steps[3].Policies[0].Render(params)
	if err != nil {
		t.Fatalf("rendering agent-mcp-tool-access: %v", err)
	}
	toolAccessJoined := strings.Join(toStrings(toolAccessDocs), "\n")
	if !strings.Contains(toolAccessJoined, "mcp.tool.target") {
		t.Fatalf("expected agent-mcp-tool-access to filter by mcp.tool.target, got: %s", toolAccessJoined)
	}
	if strings.Contains(toolAccessJoined, "{{") {
		t.Fatalf("unresolved template variable in output: %s", toolAccessJoined)
	}
}
