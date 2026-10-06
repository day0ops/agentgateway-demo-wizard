package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/day0ops/agentgateway-demo-wizard/internal/agent"
	"github.com/day0ops/agentgateway-demo-wizard/internal/api"
	"github.com/day0ops/agentgateway-demo-wizard/internal/config"
	"github.com/day0ops/agentgateway-demo-wizard/internal/gateway"
	"github.com/day0ops/agentgateway-demo-wizard/internal/k8s"
	"github.com/day0ops/agentgateway-demo-wizard/internal/mcp"
	"github.com/day0ops/agentgateway-demo-wizard/internal/scenarios"
	"github.com/day0ops/agentgateway-demo-wizard/web"
)

func main() {
	addr := os.Getenv("WIZARD_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	cfg := config.Load()
	if missing := cfg.Missing(); len(missing) > 0 {
		log.Printf("warning: missing environment variables: %v (the UI will show a setup banner)", missing)
	}

	dyn, mapper, err := k8s.NewClient()
	if err != nil {
		log.Fatalf("connecting to Kubernetes: %v", err)
	}
	applier := k8s.NewApplier(dyn, mapper)

	gatewayClient := gateway.NewClient("https://"+cfg.KeycloakHost, "https://"+cfg.GatewayHost)
	mcpClient := mcp.NewClient("https://"+cfg.GatewayHost, "/mcp")
	agentRunner := agent.NewRunner(gatewayClient, mcpClient, "/agent-chat")

	srv := api.NewServer(
		api.WithSPA(web.DistFS()),
		api.WithConfig(cfg),
		api.WithScenarios(scenarios.Registry()),
		api.WithApplier(applier),
		api.WithGateway(gatewayClient),
		api.WithAgent(agentRunner, agentToolSpecs()),
	)

	log.Printf("agentgateway-demo-wizard listening on %s", addr)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatal(err)
	}
}

// agentToolSpecs describes the two MCP tools (Task 15's agent-mcp-server
// step) in OpenAI function-calling schema, so the built-in agent loop can
// offer them to the model. get_stock_price's {symbol} schema is confirmed
// directly from the field-kit's own usecase tests; get_exchange_rate's
// {from, to} schema is this wizard's best-effort guess at that MCP server's
// real parameters, since no usecase test in the field-kit exercises its
// arguments.
func agentToolSpecs() []agent.ToolSpec {
	return []agent.ToolSpec{
		{
			Name:        "get_stock_price",
			Description: "Look up the current price of a stock by its ticker symbol.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"symbol": {"type": "string", "description": "Stock ticker symbol, e.g. AAPL"}
				},
				"required": ["symbol"]
			}`),
		},
		{
			Name:        "get_exchange_rate",
			Description: "Look up the current exchange rate between two currencies.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"from": {"type": "string", "description": "Three-letter source currency code, e.g. USD"},
					"to": {"type": "string", "description": "Three-letter target currency code, e.g. EUR"}
				},
				"required": ["from", "to"]
			}`),
		},
	}
}
