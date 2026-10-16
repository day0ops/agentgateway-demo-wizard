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

// opsAgentHTTPRouteGVR identifies the HTTPRoutes the banking usecase creates
// for the agent containers (accounts-agent, transfer-orchestrator-agent,
// settlement-agent, statements-agent) and the Core Banking / PayPal / Stripe
// MCP routes (core-banking-standard, core-banking-delegation, mcp-paypal,
// mcp-stripe-brokered). These objects are pre-provisioned by the
// agentgateway-field-kit usecase, not by this repo.
var opsAgentHTTPRouteGVR = schema.GroupVersionResource{
	Group:    "gateway.networking.k8s.io",
	Version:  "v1",
	Resource: "httproutes",
}

// opsAgentPolicyGVR identifies the enterpriseagentgateway.solo.io Policy CRD
// the auth pillar's read-only steps display alongside their HTTPRoute - MCP
// tool-level RBAC policies and PayPal/Stripe elicitation policies. These
// objects are pre-provisioned by the agentgateway-field-kit ops-agents
// usecase, not by this repo.
var opsAgentPolicyGVR = schema.GroupVersionResource{
	Group:    "enterpriseagentgateway.solo.io",
	Version:  "v1alpha1",
	Resource: "enterpriseagentgatewaypolicies",
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
				Explanation: "agentgateway is the single control point every AI call in an organization " +
					"can flow through - whether that's a chatbot answering a question or an AI agent " +
					"calling a tool - instead of every team building its own access, safety, and cost " +
					"controls against each AI provider separately.\n\n" +
					"- This wizard drives a live agentgateway: every step applies real configuration to " +
					"a real cluster and sends real requests through it.\n" +
					"- You'll watch the gateway's policies actually working, not just read about them.\n\n" +
					"Here's what you'll see:",
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
				Explanation: "Acme Corp's teams all call one agentgateway address, but each request " +
					"can ask for a different AI model by name.\n\n" +
					"- Here, that's OpenAI's gpt-4o-mini or Anthropic's Claude.\n" +
					"- The gateway reads which model the request wants and forwards it to the right " +
					"provider.\n" +
					"- One address, many models, and no routing logic in the app itself.",
				Diagram: "sequenceDiagram\n" +
					"  participant Client\n" +
					"  participant AGW as agentgateway\n" +
					"  participant OAI as OpenAI (gpt-4o-mini)\n" +
					"  participant ANT as Anthropic (Claude)\n" +
					"  Client->>AGW: POST /chat/by-name (X-Demo-Model: gpt-4o-mini)\n" +
					"  AGW->>OAI: forward\n" +
					"  OAI-->>Client: response\n" +
					"  Client->>AGW: POST /chat/by-name (X-Demo-Model: claude-sonnet-5)\n" +
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
						Headers:  map[string]string{"X-Demo-Model": "claude-sonnet-5"},
						Body:     `{"model":"claude-sonnet-5","messages":[{"role":"user","content":"What is agentgateway in one sentence?"}]}`,
					},
				},
			},
			{
				ID:    "routing-dynamic",
				Title: "Dynamic routing",
				Explanation: "The gateway doesn't have to route by model name - anything in the " +
					"request can steer it.\n\n" +
					"- Here, a simple \"tier\" label on the request picks between a fast, cheap model " +
					"and a premium one.\n" +
					"- The caller just asks for \"fast\" or \"premium\" and never needs to know which " +
					"actual model sits behind each tier.",
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
				Explanation: "Real traffic has to keep working even when a provider is having a bad " +
					"day.\n\n" +
					"- agentgateway ranks the AI providers in priority order.\n" +
					"- If the preferred one slows down or starts failing, traffic shifts to the next " +
					"one automatically, in the moment.\n" +
					"- No retry code in the app, and no one paged to flip a switch by hand.",
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
				Explanation: "agentgateway speaks OpenAI's API natively, so streaming just works: " +
					"adding \"stream\": true to the request is all it takes - there's no extra setting " +
					"to turn on.\n\n" +
					"- The same by-name route from the first step now sends the answer back piece by " +
					"piece, as the model writes it.\n" +
					"- The user starts reading right away instead of waiting for the whole reply to " +
					"finish.",
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
		ID:    "auth",
		Title: "Who's allowed to do what",
		Teaser: "Four questions every call must answer - enforced live on real banking agents " +
			"and tools.",
		Steps: []Step{
			{
				ID:    "auth-questions",
				Title: "Four questions every call must answer",
				Explanation: "Everything after this step is a real AI agent calling a real banking " +
					"tool on your behalf. Before agentgateway lets any of those calls through, it " +
					"checks the same four things every time - and logging in only settles the first " +
					"two.\n\n" +
					"- **Question 1 - who is this for?** The call has to act for the real customer, " +
					"not a stand-in (the identity in the token's `sub`).\n" +
					"- **Question 2 - who is actually doing it?** When one agent hands work to " +
					"another, every agent in the chain is named, in order (the `act` chain).\n" +
					"- **Question 3 - is this token meant for this exact service?** Each time the " +
					"gateway passes a token along, it narrows what that token is good for, so one " +
					"stolen on its way to the ledger is useless anywhere else (the `aud`).\n" +
					"- **Question 4 - is this agent allowed to do this one thing, right now?** " +
					"Decided at the moment of each tool call: the overlap between what the agent " +
					"*can* do and what you actually asked it to do.\n\n" +
					"Logging in answers only the first two. agentgateway checks all four, on every " +
					"single call.",
				Diagram: "sequenceDiagram\n" +
					"  participant You\n" +
					"  participant Agent\n" +
					"  participant AGW as agentgateway\n" +
					"  participant MCP as Core Banking MCP\n" +
					"  You->>Agent: \"Move $500 from savings to checking.\"\n" +
					"  Agent->>AGW: MCP tool call (Bearer JWT)\n" +
					"  Note over AGW: Question 1 (sub) - who is this on behalf of?\n" +
					"  Note over AGW: Question 2 (act / act.act) - who is actually acting?\n" +
					"  Note over AGW: Question 3 (aud) - is this token meant for this service?\n" +
					"  Note over AGW: Question 4 (per-tool authz) - capability ∩ delegated consent?\n" +
					"  AGW->>MCP: forward, only if all four pass\n" +
					"  MCP-->>You: result",
			},
			{
				ID:        "auth-login",
				Title:     "Real login",
				LoginDemo: true,
				Explanation: "Every other step in this wizard takes a shortcut to log you in: the " +
					"wizard itself asks Keycloak (the identity provider) for a token using a fixed demo " +
					"username and password. That's fine for showing off routing or cost, but it's the " +
					"wrong way to prove **Question 1** - who a real customer actually is.\n\n" +
					"This step does it properly, the way a real app would:\n\n" +
					"- agentgateway itself - not the wizard - sends you to Keycloak's own hosted login " +
					"page, and you sign in there.\n" +
					"- From then on, every call in this pillar carries your real identity, attached by " +
					"agentgateway as a standard login token - the wizard never handles it.\n" +
					"- agentgateway quietly renews that login in the background, so your session keeps " +
					"working without anyone managing it.\n\n" +
					"It's the welcome screen's promise made literal: agentgateway is the one point every " +
					"call flows through - and here, that includes the login itself, not just the banking " +
					"calls that follow.",
				Diagram: "sequenceDiagram\n" +
					"  participant Browser\n" +
					"  participant AGW as agentgateway (ext-auth)\n" +
					"  participant KC as Keycloak\n" +
					"  Browser->>AGW: GET /secure/login?returnTo=auth\n" +
					"  AGW->>AGW: no session yet\n" +
					"  AGW-->>Browser: 302 to Keycloak's real hosted login page\n" +
					"  Browser->>KC: log in as team-alpha or team-beta\n" +
					"  KC-->>AGW: authorization code\n" +
					"  AGW->>KC: exchange for tokens\n" +
					"  AGW->>AGW: bank session (Redis), set session cookie\n" +
					"  AGW-->>Browser: 302 back into the wizard at the right pillar\n" +
					"  Note over AGW: every later call: inject Authorization: Bearer token,<br/>refreshing transparently before it expires",
			},
			{
				ID:    "auth-accounts-impersonation",
				Title: "Accounts Agent → Core Banking (Impersonation)",
				Explanation: "The Accounts Agent reads your own balances and recent activity from the " +
					"Core Banking system. It passes your login along as-is, and agentgateway swaps it " +
					"for a token scoped to the banking system before forwarding the call - same you, " +
					"just re-issued for this one backend.\n\n" +
					"- **Question 1 - who is this for?** Still you (`sub` is unchanged).\n" +
					"- **Question 2 - who's acting?** Only the gateway, standing in for you directly. " +
					"No other agent is involved yet, so there's nothing to delegate.\n" +
					"- **Question 3 - is the token meant for this service?** Yes, and only this one: " +
					"it's scoped to the banking system (`aud`), so even if it leaked it couldn't be " +
					"used anywhere else - a token for the ledger only works on the ledger.",
				Diagram: "sequenceDiagram\n" +
					"  participant You\n" +
					"  participant Wizard\n" +
					"  participant Accounts as Accounts Agent\n" +
					"  participant AGW as agentgateway\n" +
					"  participant Bank as Core Banking MCP\n" +
					"  You->>Wizard: \"What's my balance and recent activity?\"\n" +
					"  Wizard->>Accounts: POST /accounts-agent/run (token)\n" +
					"  Accounts->>AGW: MCP call (Bearer: your token, unchanged)\n" +
					"  AGW->>AGW: token exchange (Impersonation, aud=core-banking)\n" +
					"  AGW->>Bank: forward with exchanged token\n" +
					"  Bank-->>Accounts: balances + recent activity\n" +
					"  Accounts-->>Wizard: answer\n" +
					"  Wizard-->>You: answer",
				Policies: []Policy{
					{
						ID:       "auth-accounts-impersonation",
						Title:    "Accounts Agent + Core Banking (standard route)",
						ReadOnly: true,
						ViewRefs: []ViewRef{
							{GVR: opsAgentHTTPRouteGVR, Name: "core-banking-standard"},
							{GVR: opsAgentPolicyGVR, Name: "oauth-token-exchange-core-banking-standard"},
						},
					},
				},
				OpsAgent: &OpsAgentDemo{
					Agent:         "accounts",
					DefaultPrompt: "What's my balance and recent activity?",
				},
			},
			{
				ID:    "auth-accounts-rbac",
				Title: "Accounts Agent → Core Banking (MCP tool RBAC)",
				Explanation: "Same agent, same banking system, but a sharper question: is this user " +
					"allowed to call this *specific* tool right now?\n\n" +
					"- **Try it first, before applying anything below.** Listing accounts and reading " +
					"transactions are open to any logged-in customer - and so, right now, is pulling " +
					"full account and routing numbers.\n" +
					"- **Now apply the policy and ask the exact same thing again.** A rule on the " +
					"banking backend now requires an \"org admin\" mark on your login for that one " +
					"tool. Yours doesn't have it, so that call is turned down - even though logging in " +
					"and reaching the backend still work perfectly.\n\n" +
					"That's **Question 4**: permission checked at the individual tool, in the moment of " +
					"the call - not decided once at login.",
				Diagram: "sequenceDiagram\n" +
					"  participant Accounts as Accounts Agent\n" +
					"  participant AGW as agentgateway\n" +
					"  participant Bank as Core Banking MCP\n" +
					"  Note over AGW: before Apply - no rule at all\n" +
					"  Accounts->>AGW: tools/call get_account_detail (Bearer exchanged token)\n" +
					"  AGW->>Bank: forward (nothing stops it)\n" +
					"  Bank-->>Accounts: account + routing numbers\n" +
					"  Note over AGW: after Apply\n" +
					"  Accounts->>AGW: tools/call get_account_detail (same token)\n" +
					"  AGW->>AGW: CEL: jwt.is_org_admin == true -> required\n" +
					"  AGW-->>Accounts: 403 denied - tool-level RBAC",
				Policies: []Policy{
					{
						ID:           "auth-accounts-rbac",
						Title:        "MCP tool RBAC: admin-only account detail",
						ManifestPath: "auth/accounts-rbac.yaml.tmpl",
					},
				},
				OpsAgent: &OpsAgentDemo{
					Agent:         "accounts",
					DefaultPrompt: "What's my full account and routing number?",
				},
			},
			{
				ID:    "auth-transfer-chain",
				Title: "Transfer Orchestrator → Settlement → Core Banking (Delegation)",
				Explanation: "Moving money takes more than proving who you are - the bank needs a " +
					"signed record of every agent that touched the request on its way to the ledger.\n\n" +
					"- You ask the Transfer Orchestrator to move money. It never touches the ledger " +
					"itself - it hands the job to the Settlement Agent, stamping its own name on the " +
					"request as it passes it along.\n" +
					"- The Settlement Agent does the same, adding its own name, before it finally calls " +
					"the transfer tool.\n" +
					"- The request that reaches Core Banking carries the whole signed trail: still " +
					"*your* money, moved by Settlement, acting for the Orchestrator, acting for you (a " +
					"nested `act` chain).\n\n" +
					"That covers **Question 1** (it's still you throughout), **Question 2** (the trail " +
					"names exactly which agents acted, and in what order), and **Question 3** (each " +
					"hand-off is locked to the next service only, so a token grabbed mid-chain is " +
					"useless anywhere else).",
				Diagram: "sequenceDiagram\n" +
					"  participant You\n" +
					"  participant Wizard\n" +
					"  participant Orch as Transfer Orchestrator\n" +
					"  participant STS\n" +
					"  participant Settle as Settlement Agent\n" +
					"  participant AGW as agentgateway\n" +
					"  participant Bank as Core Banking MCP\n" +
					"  You->>Wizard: \"Move $500 from savings to checking.\"\n" +
					"  Wizard->>Orch: POST /transfer-orchestrator-agent/run (token)\n" +
					"  Orch->>STS: exchange (subject=your token, actor=Orchestrator SA)\n" +
					"  STS-->>Orch: T1 (sub=you, act=transfer-orchestrator-agent)\n" +
					"  Orch->>Settle: POST /settlement-agent/run (T1)\n" +
					"  Settle->>STS: exchange (subject=T1, actor=Settlement SA)\n" +
					"  STS-->>Settle: T2 (sub=you, act=settlement-agent, act.act=transfer-orchestrator-agent)\n" +
					"  Settle->>AGW: MCP call transfer (Bearer: T2)\n" +
					"  AGW->>Bank: forward\n" +
					"  Bank-->>Settle: transfer confirmed\n" +
					"  Settle-->>Orch: result + decoded chain\n" +
					"  Orch-->>Wizard: answer + chain\n" +
					"  Wizard-->>You: answer + chain",
				Policies: []Policy{
					{
						ID:       "auth-transfer-chain",
						Title:    "Transfer Orchestrator + Settlement + Core Banking delegation route",
						ReadOnly: true,
						ViewRefs: []ViewRef{
							{GVR: opsAgentHTTPRouteGVR, Name: "transfer-orchestrator-agent"},
							{GVR: opsAgentHTTPRouteGVR, Name: "settlement-agent"},
							{GVR: opsAgentHTTPRouteGVR, Name: "core-banking-delegation"},
							{GVR: opsAgentPolicyGVR, Name: "obo-token-exchange-core-banking-delegation"},
						},
					},
				},
				OpsAgent: &OpsAgentDemo{
					Agent:         "transfer-orchestrator",
					DefaultPrompt: "Move $500 from savings to checking.",
				},
			},
			{
				ID:    "auth-settlement-rbac",
				Title: "Settlement → Core Banking (right task, right now)",
				Explanation: "That signed trail from the last step isn't just for auditing later - " +
					"it's what the bank uses to decide whether to move the money at all.\n\n" +
					"- **Try it first, before applying anything.** Run the agent: the proper chained " +
					"\"move $500\" request goes through. A direct transfer with just your own login - " +
					"same user, same bank, but no Orchestrator in the trail - also goes through for now, " +
					"because nothing is stopping it yet.\n" +
					"- **Now apply the policy.** The transfer tool now insists the request arrive " +
					"through the Transfer Orchestrator - its name has to be in the signed trail before " +
					"any money moves.\n" +
					"- **Run the agent again** and the proper chained path still works, because the " +
					"Orchestrator really did vouch for it.\n" +
					"- **Then use the \"direct transfer\" button below** to try the raw, chain-less " +
					"transfer again - now it's turned down, every time.\n\n" +
					"Reaching the banking system and being properly delegated to are *both* required, " +
					"and neither is enough on its own. That's **Question 4**: permission judged against " +
					"the whole chain of who-acted-for-whom, not just who logged in.",
				Diagram: "sequenceDiagram\n" +
					"  participant Settle as Settlement Agent (via chain)\n" +
					"  participant Direct as You (direct call, no chain)\n" +
					"  participant AGW as agentgateway\n" +
					"  participant Bank as Core Banking MCP\n" +
					"  Note over AGW: before Apply - no rule at all\n" +
					"  Direct->>AGW: tools/call transfer (your token, no act chain)\n" +
					"  AGW->>Bank: forward (nothing stops it)\n" +
					"  Bank-->>Direct: transfer confirmed\n" +
					"  Note over AGW: after Apply\n" +
					"  Settle->>AGW: tools/call transfer (act.act.sub = transfer-orchestrator-agent)\n" +
					"  AGW->>AGW: CEL: jwt.act.act.sub == orchestrator SA -> Allow\n" +
					"  AGW->>Bank: forward\n" +
					"  Bank-->>Settle: transfer confirmed\n" +
					"  Direct->>AGW: tools/call transfer (your token, no act chain)\n" +
					"  AGW->>AGW: CEL: jwt.act.act.sub missing -> no match\n" +
					"  AGW-->>Direct: denied - transfer tool hidden (no delegation chain)",
				Policies: []Policy{
					{
						ID:           "auth-settlement-rbac",
						Title:        "MCP tool RBAC: transfer requires the delegation chain",
						ManifestPath: "auth/settlement-rbac.yaml.tmpl",
					},
				},
				OpsAgent: &OpsAgentDemo{
					Agent:         "transfer-orchestrator",
					DefaultPrompt: "Move $500 from savings to checking.",
				},
				DirectCall: &DirectCallDemo{
					Title:     "Try a direct transfer (no delegation chain)",
					Path:      "/core-banking-delegation/mcp",
					Tool:      "transfer",
					Arguments: map[string]any{"from_account": "savings", "to_account": "checking", "amount": 500},
				},
			},
			{
				ID:    "auth-statements-interactive",
				Title: "Statements Agent → PayPal (Interactive elicitation)",
				Explanation: "Some things an agent needs to reach live outside the bank entirely. Here " +
					"the Statements Agent pulls your history from PayPal's own system (in sandbox mode), " +
					"on your behalf - but the gateway doesn't have your permission to use PayPal yet.\n\n" +
					"- Instead of just failing, the gateway hands back a one-time approval link. You open " +
					"it, approve access once, and from then on the gateway quietly supplies your PayPal " +
					"credential on every later call.\n" +
					"- This step uses PayPal, and the next one uses Stripe, because the two connect in " +
					"genuinely different ways - PayPal fits this \"send the user off to approve it\" " +
					"style, which is the contrast the next step builds on.\n" +
					"- Approving PayPal only proves you allowed the connection; it still doesn't decide " +
					"which Statements tools the agent may call. That's **Question 4** again, enforced " +
					"exactly like the banking tools earlier.\n\n" +
					"The agent itself never sees or stores your PayPal credential.",
				Diagram: "sequenceDiagram\n" +
					"  participant You\n" +
					"  participant Wizard\n" +
					"  participant Stmt as Statements Agent\n" +
					"  participant AGW as agentgateway\n" +
					"  participant STS\n" +
					"  participant UI as Solo UI\n" +
					"  participant PayPal\n" +
					"  You->>Wizard: \"Show my recent PayPal transactions.\"\n" +
					"  Wizard->>Stmt: POST /statements-agent/run (token)\n" +
					"  Stmt->>AGW: MCP call (Bearer: your token)\n" +
					"  AGW->>STS: check for a stored PayPal token\n" +
					"  STS-->>AGW: none yet - elicitation required\n" +
					"  AGW-->>Stmt: pending + consent URL\n" +
					"  Stmt-->>Wizard: \"open this link to connect PayPal\"\n" +
					"  You->>UI: open the link, approve\n" +
					"  UI->>PayPal: OAuth consent (sandbox)\n" +
					"  PayPal-->>STS: PayPal access token (stored)\n" +
					"  You->>Wizard: retry\n" +
					"  Wizard->>Stmt: POST /statements-agent/run (token)\n" +
					"  Stmt->>AGW: MCP call (Bearer: your token)\n" +
					"  AGW->>STS: check for a stored PayPal token\n" +
					"  STS-->>AGW: found - inject it\n" +
					"  AGW-->>Stmt: real PayPal transactions",
				Policies: []Policy{
					{
						ID:       "auth-statements-interactive",
						Title:    "Statements Agent + PayPal (Interactive elicitation)",
						ReadOnly: true,
						ViewRefs: []ViewRef{
							{GVR: opsAgentHTTPRouteGVR, Name: "mcp-paypal"},
							{GVR: opsAgentPolicyGVR, Name: "elicitation-backend-paypal-mcp-backend"},
						},
					},
				},
				OpsAgent: &OpsAgentDemo{
					Agent:         "statements",
					Mode:          "interactive",
					DefaultPrompt: "Show my recent PayPal transactions.",
				},
			},
			{
				ID:    "auth-statements-brokered",
				Title: "Statements Agent → Stripe (Brokered elicitation)",
				Explanation: "Same agent, a different outside service (Stripe), and a smoother path to " +
					"connecting it.\n\n" +
					"- You already signed in once, back at the login step, and the gateway trusts that - " +
					"so there's no need to send you off to a separate approval page again.\n" +
					"- Instead, the gateway handles the Stripe connection inline: it shows its own " +
					"approval screen, saves the resulting Stripe credential against your identity, and " +
					"retries the call in the same round trip.\n" +
					"- Your identity to the bank doesn't change here - you're still you. The only thing " +
					"being brokered is the extra Stripe credential, saved the same way the PayPal step " +
					"saved its own.\n\n" +
					"**Question 4** is unchanged - the agent's tool access is still checked per tool, " +
					"exactly as before. The contrast is the whole point: PayPal sent you to a separate " +
					"page because nothing trusted you there yet; Stripe can fold approval right into the " +
					"gateway because your session already does.",
				Diagram: "sequenceDiagram\n" +
					"  participant You\n" +
					"  participant Wizard\n" +
					"  participant Stmt as Statements Agent\n" +
					"  participant AGW as agentgateway (OAuth issuer)\n" +
					"  participant Stripe\n" +
					"  You->>Wizard: \"Show my recent Stripe payouts.\"\n" +
					"  Wizard->>Stmt: POST /statements-agent/run (token)\n" +
					"  Stmt->>AGW: MCP call (Bearer: your token)\n" +
					"  AGW->>AGW: no Stripe credential banked for you yet\n" +
					"  AGW-->>You: gateway-hosted consent screen (inline)\n" +
					"  You->>AGW: approve\n" +
					"  AGW->>Stripe: OAuth consent (sandbox)\n" +
					"  Stripe-->>AGW: Stripe access token (banked to your identity)\n" +
					"  AGW->>Stripe: forward with brokered Stripe token\n" +
					"  Stripe-->>AGW: payouts\n" +
					"  AGW-->>Stmt: real Stripe payouts\n" +
					"  Stmt-->>Wizard: answer\n" +
					"  Wizard-->>You: answer",
				Policies: []Policy{
					{
						ID:       "auth-statements-brokered",
						Title:    "Statements Agent + Stripe (Brokered elicitation)",
						ReadOnly: true,
						ViewRefs: []ViewRef{
							{GVR: opsAgentHTTPRouteGVR, Name: "mcp-stripe-brokered"},
							{GVR: opsAgentPolicyGVR, Name: "elicitation-backend-stripe-mcp-brokered-backend"},
						},
					},
				},
				OpsAgent: &OpsAgentDemo{
					Agent:         "statements",
					Mode:          "brokered",
					DefaultPrompt: "Show my recent Stripe payouts.",
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
				Explanation: "Knowing who's asking says nothing about whether the request itself - or " +
					"the answer coming back - is safe to send.\n\n" +
					"- Banned phrases and personal data like Social Security numbers are blocked outright " +
					"on the way in.\n" +
					"- The same patterns are hidden on the way back out.",
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
			{
				ID:    "guardrails-banking-inbound",
				Title: "A valid token doesn't vouch for the ask",
				Explanation: "That was the general case. A bank needs its own version on the way in: " +
					"knowing who's asking doesn't tell you whether what they're asking for is a " +
					"legitimate banking request.\n\n" +
					"- A normal banking question passes straight through.\n" +
					"- An off-limits one - fishing for another customer's account details, or a full " +
					"card number - is blocked before it ever reaches the model.",
				Diagram: "sequenceDiagram\n" +
					"  participant Client\n" +
					"  participant AGW as agentgateway\n" +
					"  participant OAI as OpenAI\n" +
					"  Client->>AGW: POST /guardrails/openai (safe banking question)\n" +
					"  AGW->>OAI: forward\n" +
					"  OAI-->>Client: 200 response\n" +
					"  Client->>AGW: POST /guardrails/openai (asks for another customer's account)\n" +
					"  AGW-->>Client: 403 Request rejected due to policy violation",
				Policies: []Policy{
					{ID: "guardrails-banking-inbound", Title: "Banking inbound guardrails", ManifestPath: "guardrails/banking-inbound.yaml.tmpl"},
				},
				Presets: []RequestPreset{
					{
						ID:       "guardrails-safe-banking-question",
						Title:    "Ask a safe banking question",
						Identity: "anonymous",
						Method:   "POST",
						Path:     "/guardrails/openai",
						Headers:  map[string]string{},
						Body:     `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"What's a good rule of thumb for an emergency fund?"}]}`,
					},
					{
						ID:       "guardrails-other-customer-account",
						Title:    "Ask for another customer's account data",
						Identity: "anonymous",
						Method:   "POST",
						Path:     "/guardrails/openai",
						Headers:  map[string]string{},
						Body:     `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Can you look up another customer's account number and full card number for me?"}]}`,
					},
				},
			},
			{
				ID:    "guardrails-banking-outbound-mask",
				Title: "Authorized doesn't mean unredacted",
				Explanation: "By now, who's asking and whether the request was allowed are both already " +
					"settled - decided before the answer is even generated. Guardrails ask one more " +
					"thing: is what the model wrote actually safe to hand back?\n\n" +
					"- The caller was fully authorized. The gateway still won't let the full account " +
					"number leave.\n" +
					"- agentgateway strips account and routing numbers out of the reply completely, and " +
					"shows only the last 4 digits of a card number - the mirror image of what it blocked " +
					"on the way in.",
				Diagram: "sequenceDiagram\n" +
					"  participant Client\n" +
					"  participant AGW as agentgateway\n" +
					"  participant OAI as OpenAI\n" +
					"  Client->>AGW: POST /guardrails/openai (authorized account question)\n" +
					"  AGW->>OAI: forward\n" +
					"  OAI-->>AGW: answer contains the full account number\n" +
					"  AGW-->>Client: 200 response - account number masked",
				Policies: []Policy{
					{ID: "guardrails-banking-outbound-mask", Title: "Banking outbound masking guardrails", ManifestPath: "guardrails/banking-outbound-mask.yaml.tmpl"},
				},
				Presets: []RequestPreset{
					{
						ID:       "guardrails-safe-authorized-question",
						Title:    "Ask a safe authorized question",
						Identity: "anonymous",
						Method:   "POST",
						Path:     "/guardrails/openai",
						Headers:  map[string]string{},
						Body:     `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"What's the current interest rate on a 12-month CD?"}]}`,
					},
					{
						ID:       "guardrails-authorized-account-number",
						Title:    "Ask an authorized account question",
						Identity: "anonymous",
						Method:   "POST",
						Path:     "/guardrails/openai",
						Headers:  map[string]string{},
						Body:     `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"For wire verification, please read my full account number back to me exactly as given, digits only: 50473618299."}]}`,
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
				Explanation: "Once every request is tied to a team, agentgateway can put a price on " +
					"each one using a built-in price list for each model - no separate billing system " +
					"needed.\n\n" +
					"- The calculated cost is written into agentgateway's own request logs, ready to feed " +
					"a spending dashboard.\n" +
					"- The raw response below still shows the model's own token usage - the figures the " +
					"price list turns into a dollar amount.",
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
				Explanation: "Seeing the spend is only half of it - agentgateway can also enforce a " +
					"hard limit.\n\n" +
					"- team-alpha and team-beta each call with their own API key and the same " +
					"deliberately tiny daily budget, set so it's used up almost immediately.\n" +
					"- team-alpha's budget is in Audit mode: going over is logged, but the request still " +
					"goes through.\n" +
					"- team-beta's budget is in Block mode: the request is turned down the instant the " +
					"budget is exceeded.",
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
		Teaser: "Governed agent-to-tool calling at the bank's Wealth & FX desk, through the same control point.",
		Steps: []Step{
			{
				ID:        "agent-mcp-server",
				Title:     "Wealth & FX desk agent",
				AgentDemo: true,
				Explanation: "Agents don't just talk to models - they use tools. Here, agentgateway " +
					"sits in front of two real tool servers behind the bank's Wealth & FX desk: one " +
					"looks up stock positions, the other looks up currency exchange rates.\n\n" +
					"Run the agent and watch:\n\n" +
					"- the model decide which tool it needs,\n" +
					"- agentgateway pass that request to the right tool server,\n" +
					"- the result flow back into the model's final answer.\n\n" +
					"All through the same gateway that handled routing, identity, and cost.",
				Diagram: "sequenceDiagram\n" +
					"  participant Agent\n" +
					"  participant AGW as agentgateway\n" +
					"  participant Model as gpt-4o-mini\n" +
					"  participant Stock as Wealth Desk MCP\n" +
					"  participant Currency as FX Desk MCP\n" +
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
				Explanation: "The same Wealth & FX desk from the last step now demands a valid login " +
					"before it will even list its tools. An agent reaching for tools gets its identity " +
					"checked exactly like any other request through the gateway - nothing is open just " +
					"because it's an agent asking.",
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
				Explanation: "Knowing who's calling is one thing - this step lets the gateway then act " +
					"as that caller against the Wealth & FX desk, without the app ever juggling a second " +
					"credential.\n\n" +
					"- agentgateway trades the caller's own login for a fresh one issued for this backend, " +
					"using a standard token-exchange handshake with the identity provider.\n" +
					"- It all happens inside the gateway, and the new token is forwarded to the tool " +
					"server.",
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
				Explanation: "Being logged in doesn't mean every caller should see every tool.\n\n" +
					"- team-alpha's login unlocks the full Wealth & FX desk - both stock and currency.\n" +
					"- team-beta's login - just as valid - unlocks only the stock tool, not currency.\n\n" +
					"Each agent is shown exactly the set of tools its identity is allowed to use, and " +
					"nothing more.",
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
				Explanation: "Every request we just ran - chatbot replies, blocked and masked content, " +
					"budget-limited calls, and agent tool calls - went through the same agentgateway " +
					"control point. Five jobs, one gateway:\n\n" +
					"- Routing to the right model\n" +
					"- Identity - who's allowed to do what\n" +
					"- Content safety\n" +
					"- Cost tracking and limits\n" +
					"- Governed agent tool access\n\n" +
					"They aren't five separate systems to buy, wire together, and maintain - they're all " +
					"just policies on one gateway.",
			},
		},
	}
}
