package main

import "testing"

// TestAgentToolSpecsUseTheMultiplexedTargetPrefix is the regression test for
// a live bug: the agent-mcp-server step's EnterpriseAgentgatewayBackend
// aggregates two MCP servers behind one endpoint under targets named "stock"
// and "currency", so agentgateway routes tools/call by a "<target>_" prefix
// on the tool name (e.g. "stock_get_stock_price") - a bare "get_stock_price"
// gets misparsed as target "get", which doesn't exist, and 500s.
func TestAgentToolSpecsUseTheMultiplexedTargetPrefix(t *testing.T) {
	tools := agentToolSpecs()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if tools[0].Name != "stock_get_stock_price" {
		t.Fatalf("expected the stock target's prefix on the tool name, got %q", tools[0].Name)
	}
	if tools[1].Name != "currency_get_exchange_rate" {
		t.Fatalf("expected the currency target's prefix on the tool name, got %q", tools[1].Name)
	}
}
