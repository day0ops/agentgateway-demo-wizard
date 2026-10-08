package scenarios

import "k8s.io/apimachinery/pkg/runtime/schema"

// routingBackendGVR identifies the enterpriseagentgateway.solo.io Backend CRD
// the 3 routing-pillar read-only policies display. These objects are
// pre-provisioned by the agentgateway-field-kit usecase, not by this repo -
// see docs/superpowers/specs/2026-10-08-demo-wizard-field-kit-integration-design.md
// in that sibling repo for the full naming contract.
var routingBackendGVR = schema.GroupVersionResource{
	Group:    "enterpriseagentgateway.solo.io",
	Version:  "v1alpha1",
	Resource: "enterpriseagentgatewaybackends",
}

// Registry returns the wizard's full demo script, in presentation order.
func Registry() []Pillar {
	pillars := []Pillar{
		introPillar(),
		routingPillar(),
		authPillar(),
		guardrailsPillar(),
		costPillar(),
		agentPillar(),
		wrapPillar(),
	}
	normalizeNilSlices(pillars)
	return pillars
}

// normalizeNilSlices replaces any step's nil Policies/Presets with an empty,
// non-nil slice. A step literal that never sets one of these fields leaves it
// at Go's zero value (nil), which encoding/json marshals as JSON null rather
// than []; the frontend calls .map() on both fields unconditionally, so a
// null crashes it. Running this once here, after every pillar is assembled,
// means no individual step literal has to remember to initialize them.
func normalizeNilSlices(pillars []Pillar) {
	for i := range pillars {
		steps := pillars[i].Steps
		for j := range steps {
			if steps[j].Policies == nil {
				steps[j].Policies = []Policy{}
			}
			if steps[j].Presets == nil {
				steps[j].Presets = []RequestPreset{}
			}
		}
	}
}

func introPillar() Pillar {
	return Pillar{
		ID:    "intro",
		Title: "Welcome",
		Steps: []Step{
			{
				ID:    "welcome",
				Title: "Welcome to agentgateway",
				Explanation: "agentgateway is the one control point every AI call in an organization - " +
					"chat completions, agent tool calls - can flow through, instead of every team " +
					"wiring up its own access, safety, and cost controls against each provider " +
					"directly. This wizard drives a live agentgateway instance: each step applies real " +
					"configuration to a real cluster and fires real requests through it, so you see " +
					"the gateway's policies working rather than read about them. Here's what you'll see:",
			},
		},
	}
}

func routingPillar() Pillar {
	return Pillar{
		ID:     "routing",
		Title:  "One endpoint, many models",
		Teaser: "Smart model routing with automatic fallback when a backend degrades.",
		Steps: []Step{
			{
				ID:    "routing-by-name",
				Title: "Model routing by name",
				Explanation: "Acme Corp's teams all call one agentgateway endpoint, but each request " +
					"can ask for a different model - here, OpenAI's gpt-4o-mini or Anthropic's Claude. " +
					"The gateway resolves the backend from a single signal in the request and forwards " +
					"to the right provider. One endpoint, many models, zero client-side routing logic.",
				Diagram: "sequenceDiagram\n" +
					"  participant Client\n" +
					"  participant AGW as agentgateway\n" +
					"  participant OAI as OpenAI (gpt-4o-mini)\n" +
					"  participant ANT as Anthropic (Claude)\n" +
					"  Client->>AGW: POST /chat/by-name (X-Demo-Model: gpt-4o-mini)\n" +
					"  AGW->>OAI: forward\n" +
					"  OAI-->>Client: response\n" +
					"  Client->>AGW: POST /chat/by-name (X-Demo-Model: claude-3-5-sonnet-20241022)\n" +
					"  AGW->>ANT: forward\n" +
					"  ANT-->>Client: response",
				Policies: []Policy{
					{
						ID:       "routing-by-name",
						ReadOnly: true,
						ViewRefs: []ViewRef{
							{GVR: routingBackendGVR, Name: "routing-by-name-openai"},
							{GVR: routingBackendGVR, Name: "routing-by-name-anthropic"},
						},
					},
				},
				Presets: []RequestPreset{
					{
						ID:       "ask-gpt-4o-mini",
						Title:    "Route to OpenAI",
						Identity: "anonymous",
						Method:   "POST",
						Path:     "/chat/by-name",
						Headers:  map[string]string{"X-Demo-Model": "gpt-4o-mini"},
						Body:     `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"What is agentgateway in one sentence?"}]}`,
					},
					{
						ID:       "ask-claude",
						Title:    "Route to Anthropic",
						Identity: "anonymous",
						Method:   "POST",
						Path:     "/chat/by-name",
						Headers:  map[string]string{"X-Demo-Model": "claude-3-5-sonnet-20241022"},
						Body:     `{"model":"claude-3-5-sonnet-20241022","messages":[{"role":"user","content":"What is agentgateway in one sentence?"}]}`,
					},
				},
			},
			{
				ID:    "routing-dynamic",
				Title: "Dynamic routing",
				Explanation: "The gateway isn't limited to routing by model name - any signal in the " +
					"request can steer traffic. Here, a tier header picks between a fast, inexpensive " +
					"model and a premium one, without the caller needing to know which concrete model " +
					"backs each tier.",
				Diagram: "flowchart LR\n" +
					"  Client -->|\"X-Model-Tier: fast\"| AGW[agentgateway]\n" +
					"  Client -->|\"X-Model-Tier: premium\"| AGW\n" +
					"  AGW -->|fast| Fast[gpt-4o-mini]\n" +
					"  AGW -->|premium| Premium[gpt-4o]",
				Policies: []Policy{
					{
						ID:       "routing-dynamic",
						Title:    "Dynamic tier routing",
						ReadOnly: true,
						ViewRefs: []ViewRef{
							{GVR: routingBackendGVR, Name: "routing-dynamic-fast"},
							{GVR: routingBackendGVR, Name: "routing-dynamic-premium"},
						},
					},
				},
				Presets: []RequestPreset{
					{
						ID:       "ask-fast-tier",
						Title:    "Ask as fast tier",
						Identity: "anonymous",
						Method:   "POST",
						Path:     "/chat/dynamic",
						Headers:  map[string]string{"X-Model-Tier": "fast"},
						Body:     `{"messages":[{"role":"user","content":"Summarize agentgateway in one sentence."}]}`,
					},
					{
						ID:       "ask-premium-tier",
						Title:    "Ask as premium tier",
						Identity: "anonymous",
						Method:   "POST",
						Path:     "/chat/dynamic",
						Headers:  map[string]string{"X-Model-Tier": "premium"},
						Body:     `{"messages":[{"role":"user","content":"Summarize agentgateway in one sentence."}]}`,
					},
				},
			},
			{
				ID:    "routing-fallback",
				Title: "Automatic fallback",
				Explanation: "Production traffic doesn't get to choose a good day. agentgateway groups " +
					"backends into priority tiers and automatically shifts traffic to the next tier the " +
					"moment the current one degrades - no client retry logic, no manual failover runbook.",
				Diagram: "sequenceDiagram\n" +
					"  participant Client\n" +
					"  participant AGW as agentgateway\n" +
					"  participant Cheap as gpt-4o-mini (priority 100)\n" +
					"  participant Premium as gpt-4o (priority 50)\n" +
					"  Client->>AGW: request\n" +
					"  AGW->>Cheap: try priority 100 first\n" +
					"  Cheap-->>AGW: unhealthy / unavailable\n" +
					"  AGW->>Premium: fail over automatically\n" +
					"  Premium-->>Client: response",
				Policies: []Policy{
					{
						ID:       "routing-fallback",
						Title:    "Priority-tiered failover",
						ReadOnly: true,
						ViewRefs: []ViewRef{
							{GVR: routingBackendGVR, Name: "routing-fallback"},
						},
					},
				},
				Presets: []RequestPreset{
					{
						ID:       "ask-with-fallback",
						Title:    "Send a request",
						Identity: "anonymous",
						Method:   "POST",
						Path:     "/chat/fallback",
						Headers:  map[string]string{},
						Body:     `{"messages":[{"role":"user","content":"What is Kubernetes in one sentence?"}]}`,
					},
				},
			},
			{
				ID:    "routing-streaming",
				Title: "Streaming responses",
				Explanation: "OpenAI is natively OpenAI-compatible, so \"stream\": true in the " +
					"request body is all agentgateway needs - no separate streaming policy. The " +
					"same by-name route from the first step now returns the response as it's " +
					"generated, token by token, instead of making the caller wait for the whole " +
					"completion.",
				Diagram: "sequenceDiagram\n" +
					"  participant Client\n" +
					"  participant AGW as agentgateway\n" +
					"  participant OAI as OpenAI\n" +
					"  Client->>AGW: POST /chat/by-name (stream: true)\n" +
					"  AGW->>OAI: forward as-is\n" +
					"  OAI-->>AGW: SSE chunk 1\n" +
					"  AGW-->>Client: data: {...}\n" +
					"  OAI-->>AGW: SSE chunk 2\n" +
					"  AGW-->>Client: data: {...}\n" +
					"  OAI-->>AGW: data: [DONE]\n" +
					"  AGW-->>Client: data: [DONE]",
				Presets: []RequestPreset{
					{
						ID:       "ask-streaming",
						Title:    "Ask with streaming",
						Identity: "anonymous",
						Method:   "POST",
						Path:     "/chat/by-name",
						Headers:  map[string]string{"X-Demo-Model": "gpt-4o-mini"},
						Body:     `{"model":"gpt-4o-mini","stream":true,"messages":[{"role":"user","content":"Count from 1 to 5."}]}`,
						Stream:   true,
					},
				},
			},
		},
	}
}

func authPillar() Pillar {
	return Pillar{
		ID:     "auth",
		Title:  "Who's allowed to do what",
		Teaser: "Identity-aware access control - every call is authenticated and authorized.",
		Steps: []Step{
			{
				ID:    "auth-jwt",
				Title: "JWT authentication",
				Explanation: "Identity is enforced at the edge, not in every backend service. " +
					"agentgateway validates the caller's JWT against Keycloak before a request is " +
					"ever forwarded - no token, no access, and the backend never has to implement " +
					"auth itself.",
				Diagram: "sequenceDiagram\n" +
					"  participant Client\n" +
					"  participant AGW as agentgateway\n" +
					"  participant KC as Keycloak\n" +
					"  participant Echo as Echo backend\n" +
					"  Client->>AGW: GET /secure/jwt (no token)\n" +
					"  AGW-->>Client: 401\n" +
					"  Client->>KC: password grant\n" +
					"  KC-->>Client: JWT (team=team-alpha)\n" +
					"  Client->>AGW: GET /secure/jwt (Bearer JWT)\n" +
					"  AGW->>KC: validate JWT (JWKS)\n" +
					"  AGW->>Echo: forward + x-gw-team header\n" +
					"  Echo-->>Client: 200",
				Policies: []Policy{
					{ID: "auth-jwt", Title: "JWT authentication", ManifestPath: "auth/jwt.yaml.tmpl"},
				},
				Presets: []RequestPreset{
					{
						ID:       "no-token",
						Title:    "No token",
						Identity: "anonymous",
						Method:   "GET",
						Path:     "/secure/jwt",
						Headers:  map[string]string{},
						Body:     "",
					},
					{
						ID:       "valid-token-team-alpha",
						Title:    "Valid token (team-alpha)",
						Identity: "team-alpha",
						Method:   "GET",
						Path:     "/secure/jwt",
						Headers:  map[string]string{},
						Body:     "",
					},
				},
			},
			{
				ID:    "auth-rbac",
				Title: "Team / role authZ",
				Explanation: "Authentication answers \"who are you\"; authorization answers \"what are " +
					"you allowed to do.\" Both teams present a perfectly valid JWT here - team-alpha is " +
					"explicitly allowed, team-beta is not, and is denied by default rather than by an " +
					"explicit rule naming them.",
				Diagram: "sequenceDiagram\n" +
					"  participant Alpha as team-alpha\n" +
					"  participant Beta as team-beta\n" +
					"  participant AGW as agentgateway\n" +
					"  Alpha->>AGW: GET /secure/rbac (Bearer JWT, team=team-alpha)\n" +
					"  AGW->>AGW: CEL: jwt.team == 'team-alpha' -> Allow\n" +
					"  AGW-->>Alpha: 200\n" +
					"  Beta->>AGW: GET /secure/rbac (Bearer JWT, team=team-beta)\n" +
					"  AGW->>AGW: CEL: no matching Allow rule\n" +
					"  AGW-->>Beta: 403",
				Policies: []Policy{
					{ID: "auth-rbac", Title: "Team RBAC", ManifestPath: "auth/rbac.yaml.tmpl"},
				},
				Presets: []RequestPreset{
					{
						ID:       "team-alpha-allowed",
						Title:    "team-alpha (allowed)",
						Identity: "team-alpha",
						Method:   "GET",
						Path:     "/secure/rbac",
						Headers:  map[string]string{},
						Body:     "",
					},
					{
						ID:       "team-beta-denied",
						Title:    "team-beta (denied)",
						Identity: "team-beta",
						Method:   "GET",
						Path:     "/secure/rbac",
						Headers:  map[string]string{},
						Body:     "",
					},
				},
			},
		},
	}
}

func guardrailsPillar() Pillar {
	return Pillar{
		ID:     "guardrails",
		Title:  "What gets through",
		Teaser: "Content guardrails that block or mask what shouldn't get through.",
		Steps: []Step{
			{
				ID:    "guardrails-prompt-guard",
				Title: "Not everything should get through",
				Explanation: "A valid token only proves who's asking - it says nothing about " +
					"whether what they're asking for, or what comes back, is safe to send. " +
					"agentgateway inspects both the request and the response: custom phrases and " +
					"built-in PII patterns are rejected outright on the way in, and the same " +
					"patterns are masked on the way out.",
				Diagram: "sequenceDiagram\n" +
					"  participant Client\n" +
					"  participant AGW as agentgateway\n" +
					"  participant OAI as OpenAI\n" +
					"  Client->>AGW: POST /guardrails/openai (safe prompt)\n" +
					"  AGW->>OAI: forward\n" +
					"  OAI-->>Client: 200 response\n" +
					"  Client->>AGW: POST /guardrails/openai (contains SSN)\n" +
					"  AGW-->>Client: 403 Request rejected due to policy violation",
				Policies: []Policy{
					{ID: "guardrails-prompt-guard", Title: "Prompt guardrails", ManifestPath: "guardrails/prompt-guard.yaml.tmpl"},
				},
				Presets: []RequestPreset{
					{
						ID:       "guardrails-safe-question",
						Title:    "Ask a safe question",
						Identity: "anonymous",
						Method:   "POST",
						Path:     "/guardrails/openai",
						Headers:  map[string]string{},
						Body:     `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"What is agentgateway in one sentence?"}]}`,
					},
					{
						ID:       "guardrails-mention-ssn",
						Title:    "Mention an SSN",
						Identity: "anonymous",
						Method:   "POST",
						Path:     "/guardrails/openai",
						Headers:  map[string]string{},
						Body:     `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"My SSN is 123-45-6789, can you help me?"}]}`,
					},
					{
						ID:       "guardrails-credit-card",
						Title:    "Ask for a credit card number",
						Identity: "anonymous",
						Method:   "POST",
						Path:     "/guardrails/openai",
						Headers:  map[string]string{},
						Body:     `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Can you help me with my credit card information?"}]}`,
					},
				},
			},
		},
	}
}

func costPillar() Pillar {
	return Pillar{
		ID:     "cost",
		Title:  "You can't govern what you can't see",
		Teaser: "Full cost attribution, with budgets and virtual keys per team.",
		Steps: []Step{
			{
				ID:    "cost-attribution",
				Title: "Cost attribution",
				Explanation: "Once requests are authenticated per team, agentgateway can price every " +
					"one of them using a model cost catalog attached to the gateway - no custom " +
					"billing pipeline required. The computed spend (llm.cost, agw.ai.usage.cost.total) " +
					"lands in agentgateway's own request logs, ready to feed a cost dashboard; the raw " +
					"response below still shows OpenAI's own token usage, the numbers the catalog prices.",
				Diagram: "flowchart LR\n" +
					"  Client -->|POST /cost/openai| AGW[agentgateway]\n" +
					"  AGW -->|spec.infrastructure.parametersRef| Params[EnterpriseAgentgatewayParameters]\n" +
					"  Params --> Catalog[(Model cost catalog)]\n" +
					"  AGW --> OpenAI[OpenAI gpt-4o-mini]\n" +
					"  AGW -.->|llm.cost, agw.ai.usage.cost.total| Logs[(Request logs)]",
				Policies: []Policy{
					{
						ID:                      "cost-attribution",
						Title:                   "Model cost catalog",
						ManifestPath:            "cost/attribution.yaml.tmpl",
						PatchManifestPath:       "cost/attribution-attach.yaml.tmpl",
						PatchRevertManifestPath: "cost/attribution-detach.yaml.tmpl",
					},
				},
				Presets: []RequestPreset{
					{
						ID:       "ask-priced-route",
						Title:    "Ask through the priced route",
						Identity: "anonymous",
						Method:   "POST",
						Path:     "/cost/openai",
						Headers:  map[string]string{},
						Body:     `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"What is agentgateway in one sentence?"}]}`,
					},
				},
			},
			{
				ID:          "cost-budgets",
				Title:       "Budgets and spend limits",
				VirtualKeys: true,
				Explanation: "Visibility is only half the story - agentgateway can also enforce a " +
					"hard ceiling. team-alpha and team-beta each call with a scoped API key and an " +
					"identical one-token daily budget; team-alpha's budget is in Audit mode (the " +
					"overage is logged, the request still succeeds), team-beta's is in Block mode " +
					"(the request is rejected the moment the budget is exceeded).",
				Diagram: "sequenceDiagram\n" +
					"  participant Alpha as team-alpha (API key)\n" +
					"  participant Beta as team-beta (API key)\n" +
					"  participant AGW as agentgateway\n" +
					"  participant OAI as OpenAI\n" +
					"  Alpha->>AGW: POST /cost/budgets (Bearer sk-team-alpha-...)\n" +
					"  AGW->>AGW: budget 'team-alpha-daily-tokens' exceeded (Audit) - log only\n" +
					"  AGW->>OAI: forward\n" +
					"  OAI-->>Alpha: 200\n" +
					"  Beta->>AGW: POST /cost/budgets (Bearer sk-team-beta-...)\n" +
					"  AGW->>AGW: budget 'team-beta-daily-tokens' exceeded (Block)\n" +
					"  AGW-->>Beta: 429",
				Policies: []Policy{
					{ID: "cost-budgets", Title: "Budgets and virtual keys", ManifestPath: "cost/budgets.yaml.tmpl"},
				},
				Presets: []RequestPreset{
					{
						ID:       "team-alpha-audit",
						Title:    "team-alpha asks (audit)",
						Identity: "anonymous",
						Method:   "POST",
						Path:     "/cost/budgets",
						Headers:  map[string]string{"Authorization": "Bearer sk-team-alpha-demo-budget"},
						Body:     `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello"}]}`,
					},
					{
						ID:       "team-beta-blocked",
						Title:    "team-beta asks (blocked)",
						Identity: "anonymous",
						Method:   "POST",
						Path:     "/cost/budgets",
						Headers:  map[string]string{"Authorization": "Bearer sk-team-beta-demo-budget"},
						Body:     `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello"}]}`,
					},
				},
			},
		},
	}
}

func agentPillar() Pillar {
	return Pillar{
		ID:     "agent",
		Title:  "From models to agents",
		Teaser: "Governed agent-to-tool calling, through the same control point.",
		Steps: []Step{
			{
				ID:        "agent-mcp-server",
				Title:     "Agent to MCP tool call",
				AgentDemo: true,
				Explanation: "Agents don't just call models - they call tools. Here, agentgateway " +
					"fronts two real MCP servers (stock and currency lookups). Run the agent and watch " +
					"the model decide which tool to call, agentgateway forward that tool call to the " +
					"right MCP server, and the result flow back into the model's final answer - all " +
					"through the same gateway that handled routing, auth, and cost.",
				Diagram: "sequenceDiagram\n" +
					"  participant Agent\n" +
					"  participant AGW as agentgateway\n" +
					"  participant Model as gpt-4o-mini\n" +
					"  participant Stock as Stock MCP\n" +
					"  participant Currency as Currency MCP\n" +
					"  Agent->>AGW: POST /agent-chat (prompt)\n" +
					"  AGW->>Model: forward\n" +
					"  Model-->>AGW: tool_call: get_stock_price\n" +
					"  AGW->>Stock: forward tool call\n" +
					"  Stock-->>AGW: result\n" +
					"  AGW->>Model: forward result\n" +
					"  Model-->>Agent: final answer",
				Policies: []Policy{
					{ID: "agent-mcp-server", Title: "MCP servers + agent chat provider", ManifestPath: "mcp/server.yaml.tmpl"},
				},
			},
			{
				ID:    "agent-mcp-auth",
				Title: "MCP authentication",
				Explanation: "The same MCP backend from the last step now requires a valid token " +
					"before it will even list its tools - agentic tool access gets the exact same " +
					"identity enforcement as any other route through the gateway.",
				Diagram: "sequenceDiagram\n" +
					"  participant Client as MCP Client\n" +
					"  participant AGW as agentgateway\n" +
					"  participant KC as Keycloak\n" +
					"  Client->>AGW: tools/list (no token)\n" +
					"  AGW-->>Client: 401\n" +
					"  Client->>KC: password grant\n" +
					"  KC-->>Client: JWT\n" +
					"  Client->>AGW: tools/list (Bearer JWT)\n" +
					"  AGW->>KC: validate JWT\n" +
					"  AGW-->>Client: 200 (tools list)",
				Policies: []Policy{
					{ID: "agent-mcp-auth", Title: "MCP authentication", ManifestPath: "mcp/auth.yaml.tmpl"},
				},
				Presets: []RequestPreset{
					{
						ID:       "list-tools-no-token",
						Title:    "List tools (no token)",
						Identity: "anonymous",
						Method:   "POST",
						Path:     "/mcp",
						Headers:  map[string]string{},
						Body:     `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
					},
					{
						ID:       "list-tools-team-alpha",
						Title:    "List tools (team-alpha)",
						Identity: "team-alpha",
						Method:   "POST",
						Path:     "/mcp",
						Headers:  map[string]string{},
						Body:     `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
					},
				},
			},
			{
				ID:    "agent-token-exchange",
				Title: "Token exchange (OBO)",
				Explanation: "Authentication confirms who's calling - token exchange lets the " +
					"gateway act as that caller against a downstream service, without the client " +
					"ever handling a second credential. agentgateway exchanges the caller's own " +
					"Keycloak token for a new one at Keycloak's token endpoint (RFC 8693), " +
					"natively in the data plane, and forwards the exchanged token to the MCP " +
					"backend.",
				Diagram: "sequenceDiagram\n" +
					"  participant Client\n" +
					"  participant AGW as agentgateway\n" +
					"  participant KC as Keycloak\n" +
					"  participant MCP as MCP backend\n" +
					"  Client->>AGW: tools/list (Bearer JWT)\n" +
					"  AGW->>KC: validate via JWKS\n" +
					"  AGW->>KC: exchange (RFC 8693 subject_token)\n" +
					"  KC-->>AGW: exchanged token\n" +
					"  AGW->>MCP: forward with exchanged token",
				Policies: []Policy{
					{ID: "agent-token-exchange", Title: "OAuth token exchange", ManifestPath: "agent/token-exchange.yaml.tmpl"},
				},
				Presets: []RequestPreset{
					{
						ID:       "tools-list-token-exchange",
						Title:    "Call MCP as team-alpha",
						Identity: "team-alpha",
						Method:   "POST",
						Path:     "/mcp",
						Headers:  map[string]string{},
						Body:     `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
						Stream:   false,
					},
				},
			},
			{
				ID:    "agent-mcp-tool-access",
				Title: "Tool authZ and progressive disclosure",
				Explanation: "Authentication alone doesn't mean every caller should see every tool. " +
					"team-alpha's token unlocks both the stock and currency tools, while team-beta's " +
					"token - equally valid - only unlocks stock. This is progressive disclosure: agents " +
					"are shown exactly the tool surface their identity is allowed to use, nothing more.",
				Diagram: "sequenceDiagram\n" +
					"  participant Alpha as team-alpha\n" +
					"  participant Beta as team-beta\n" +
					"  participant AGW as agentgateway\n" +
					"  Alpha->>AGW: tools/list (Bearer JWT, team=team-alpha)\n" +
					"  AGW-->>Alpha: [get_stock_price, get_exchange_rate]\n" +
					"  Beta->>AGW: tools/list (Bearer JWT, team=team-beta)\n" +
					"  AGW-->>Beta: [get_stock_price]",
				Policies: []Policy{
					{ID: "agent-mcp-tool-access", Title: "Per-team tool access", ManifestPath: "mcp/tool-access.yaml.tmpl"},
				},
				Presets: []RequestPreset{
					{
						ID:       "tools-list-team-alpha",
						Title:    "team-alpha tools/list",
						Identity: "team-alpha",
						Method:   "POST",
						Path:     "/mcp",
						Headers:  map[string]string{},
						Body:     `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
					},
					{
						ID:       "tools-list-team-beta",
						Title:    "team-beta tools/list",
						Identity: "team-beta",
						Method:   "POST",
						Path:     "/mcp",
						Headers:  map[string]string{},
						Body:     `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
					},
				},
			},
		},
	}
}

func wrapPillar() Pillar {
	return Pillar{
		ID:    "wrap",
		Title: "Wrap-up",
		Steps: []Step{
			{
				ID:    "recap",
				Title: "One gateway, five guarantees",
				Explanation: "Every request we just ran - chat completions, blocked and masked " +
					"content, budget-limited calls, and agent tool calls - went through the same " +
					"agentgateway control point. That's the point: routing, identity, content safety, " +
					"cost, and agentic tool access aren't five separate systems to integrate and " +
					"maintain, they're one gateway's policies.",
			},
		},
	}
}
