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
	if len(agentPillarResult.Steps) != 8 {
		t.Fatalf("expected 8 agent steps, got %d", len(agentPillarResult.Steps))
	}

	wantStepIDs := []string{
		"agent-mcp-server", "agent-mcp-auth", "agent-token-exchange", "agent-mcp-tool-access",
		"agent-mcp-guardrails-deny", "agent-mcp-guardrails-poison", "agent-mcp-guardrails-pii",
		"tool-modes",
	}
	for i, want := range wantStepIDs {
		if agentPillarResult.Steps[i].ID != want {
			t.Fatalf("expected step[%d].ID %q, got %q", i, want, agentPillarResult.Steps[i].ID)
		}
	}

	for i, step := range agentPillarResult.Steps {
		wantAgentDemo := i == 0
		if step.AgentDemo != wantAgentDemo {
			t.Fatalf("expected only agent-mcp-server to be an agent demo step, step[%d] (%q).AgentDemo = %v", i, step.ID, step.AgentDemo)
		}
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

// TestAgentPillarMCPPresetsSetAcceptHeader is the regression test for a live
// bug: every RequestPreset that POSTs to /mcp went through the generic
// request-builder (handleRequest), which forwards Headers verbatim with no
// MCP-specific defaults - an empty Headers map meant agentgateway's
// Streamable HTTP transport rejected the request with 406 "client must
// accept both application/json and text/event-stream".
func TestAgentPillarMCPPresetsSetAcceptHeader(t *testing.T) {
	agentPillarResult := Registry()[5]

	var checked int
	for _, step := range agentPillarResult.Steps {
		for _, preset := range step.Presets {
			if preset.Path != "/mcp" {
				continue
			}
			checked++
			if got := preset.Headers["Accept"]; got != "application/json, text/event-stream" {
				t.Errorf("preset %q: expected Accept header, got %q", preset.ID, got)
			}
		}
	}
	if checked != 5 {
		t.Fatalf("expected 5 presets hitting /mcp, checked %d - update this test if presets were added/removed", checked)
	}
}

func TestMcpGuardrailsPoliciesRenderValidManifests(t *testing.T) {
	params := ManifestParams{
		Namespace:            "agentgateway-system",
		GatewayName:          "agentgateway-gw",
		GatewayNS:            "agentgateway-system",
		StockMCPImage:        "example.com/stock:0.1.1",
		ExtMCPGuardrailImage: "example.com/ext-mcp-guardrail:0.1.2",
	}

	agentPillarResult := Registry()[5]

	denyStep := agentPillarResult.Steps[4]
	if denyStep.ID != "agent-mcp-guardrails-deny" {
		t.Fatalf("expected step[4].ID 'agent-mcp-guardrails-deny', got %q", denyStep.ID)
	}
	denyDocs, err := denyStep.Policies[0].Render(params)
	if err != nil {
		t.Fatalf("rendering agent-mcp-guardrails-deny: %v", err)
	}
	denyJoined := strings.Join(toStrings(denyDocs), "\n")
	for _, want := range []string{
		params.ExtMCPGuardrailImage, "DENIED_TOOLS", "get_stock_history",
		"mcp-guardrails-deny-backend", "kubernetes.io/h2c", "FailClosed",
	} {
		if !strings.Contains(denyJoined, want) {
			t.Fatalf("expected agent-mcp-guardrails-deny manifest to contain %q, got: %s", want, denyJoined)
		}
	}
	if strings.Contains(denyJoined, "{{") {
		t.Fatalf("unresolved template variable in output: %s", denyJoined)
	}

	poisonStep := agentPillarResult.Steps[5]
	if poisonStep.ID != "agent-mcp-guardrails-poison" {
		t.Fatalf("expected step[5].ID 'agent-mcp-guardrails-poison', got %q", poisonStep.ID)
	}
	poisonDocs, err := poisonStep.Policies[0].Render(params)
	if err != nil {
		t.Fatalf("rendering agent-mcp-guardrails-poison: %v", err)
	}
	poisonJoined := strings.Join(toStrings(poisonDocs), "\n")
	for _, want := range []string{
		"entMcp", "stock-lookup", "attacker@rug-pull-demo.example", "tools/list",
		"mcp-guardrails-poison-backend", "mcp-guardrails-poison-stock-backend",
	} {
		if !strings.Contains(poisonJoined, want) {
			t.Fatalf("expected agent-mcp-guardrails-poison manifest to contain %q, got: %s", want, poisonJoined)
		}
	}
	if strings.Contains(poisonJoined, "{{") {
		t.Fatalf("unresolved template variable in output: %s", poisonJoined)
	}

	piiStep := agentPillarResult.Steps[6]
	if piiStep.ID != "agent-mcp-guardrails-pii" {
		t.Fatalf("expected step[6].ID 'agent-mcp-guardrails-pii', got %q", piiStep.ID)
	}
	piiDocs, err := piiStep.Policies[0].Render(params)
	if err != nil {
		t.Fatalf("rendering agent-mcp-guardrails-pii: %v", err)
	}
	piiJoined := strings.Join(toStrings(piiDocs), "\n")
	for _, want := range []string{
		"entMcp", "customer-lookup", "jane.doe@example.com", "tools/call",
		"mcp-guardrails-pii-backend", "mcp-guardrails-pii-stock-backend",
	} {
		if !strings.Contains(piiJoined, want) {
			t.Fatalf("expected agent-mcp-guardrails-pii manifest to contain %q, got: %s", want, piiJoined)
		}
	}
	if strings.Contains(piiJoined, "{{") {
		t.Fatalf("unresolved template variable in output: %s", piiJoined)
	}
}

// TestMcpGuardrailsPresetsHaveDistinctRoutesAndMCPSuffix guards against the
// live bug class this feature is most exposed to: handleRequest only runs the
// MCP session handshake when the request path ends in "/mcp" (see
// internal/api/request_handler.go), and three screens sharing one Gateway
// must each own a distinct route or Apply/Revert on one would affect another.
func TestMcpGuardrailsPresetsHaveDistinctRoutesAndMCPSuffix(t *testing.T) {
	agentPillarResult := Registry()[5]
	guardrailStepIDs := map[string]bool{
		"agent-mcp-guardrails-deny":   true,
		"agent-mcp-guardrails-poison": true,
		"agent-mcp-guardrails-pii":    true,
	}

	routeOwner := map[string]string{}
	for _, step := range agentPillarResult.Steps {
		if !guardrailStepIDs[step.ID] {
			continue
		}
		for _, preset := range step.Presets {
			if !strings.HasSuffix(preset.Path, "/mcp") {
				t.Errorf("step %q preset %q: path %q must end in /mcp or the MCP handshake never runs", step.ID, preset.ID, preset.Path)
			}
			if got := preset.Headers["Accept"]; got != "application/json, text/event-stream" {
				t.Errorf("step %q preset %q: expected Accept header, got %q", step.ID, preset.ID, got)
			}
			if owner, ok := routeOwner[preset.Path]; ok && owner != step.ID {
				t.Errorf("preset path %q is shared between steps %q and %q", preset.Path, owner, step.ID)
			}
			routeOwner[preset.Path] = step.ID
		}
	}
	owningSteps := map[string]bool{}
	for _, step := range routeOwner {
		owningSteps[step] = true
	}
	if len(owningSteps) != 3 {
		t.Fatalf("expected all 3 guardrail steps to own at least one preset route, got %d", len(owningSteps))
	}
}
