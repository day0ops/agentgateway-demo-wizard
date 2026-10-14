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
// settlement-agent, statements-agent) and the Core Banking / Stripe MCP routes
// (core-banking-standard, core-banking-delegation, mcp-stripe,
// mcp-stripe-brokered). These objects are pre-provisioned by the
// agentgateway-field-kit usecase, not by this repo.
var opsAgentHTTPRouteGVR = schema.GroupVersionResource{
	Group:    "gateway.networking.k8s.io",
	Version:  "v1",
	Resource: "httproutes",
}

// opsAgentPolicyGVR identifies the enterpriseagentgateway.solo.io Policy CRD
// the auth pillar's read-only steps display alongside their HTTPRoute - MCP
// tool-level RBAC policies and Stripe elicitation policies. These objects
// are pre-provisioned by the agentgateway-field-kit ops-agents usecase, not
// by this repo.
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
		ID:    "auth",
		Title: "Who's allowed to do what",
		Teaser: "Four questions every call must answer - enforced live on real banking agents " +
			"and tools.",
		Steps: []Step{
			{
				ID:    "auth-questions",
				Title: "Four questions every call must answer",
				Explanation: "Everything else in this pillar is a real agent calling a real banking " +
					"tool, and every one of those calls has to answer the same four questions before " +
					"agentgateway lets it through. Question 1: who is this being acted on behalf of - " +
					"carried in the token's sub claim, and it should be the real customer, not a " +
					"stand-in. Question 2: who is actually performing the action - carried in act, and " +
					"in a nested act.act once more than one agent is in the chain. Question 3: is this " +
					"token intended for this specific service - carried in aud, scoped down every time " +
					"the gateway exchanges a token, so a token minted for one backend can't be replayed " +
					"against another. Question 4: does this agent have the right to this exact task right " +
					"now - decided per MCP tool call, as the intersection of what the agent is capable of " +
					"and what the user actually delegated to it. Authentication only ever answers the " +
					"first two; the gateway enforces all four, on every call, not just at login.",
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
				Explanation: "Every other step in this pillar - and every other pillar in this wizard - " +
					"uses a quick password grant: the wizard itself asks Keycloak for a token using a " +
					"fixed demo username and password. That's fine for driving routing or cost demos, but " +
					"it's the wrong way to establish Question 1 (who this is being acted on behalf of) for " +
					"a real customer, since it means handing a password straight to a client. This step " +
					"replaces it with a real Authorization Code + PKCE login: the wizard redirects your " +
					"browser to Keycloak's own hosted login page, Keycloak redirects back with an " +
					"authorization code, and the wizard exchanges that code for a real access token " +
					"server-side. Every step after this one carries that real sub, not a password-grant " +
					"placeholder.",
				Diagram: "sequenceDiagram\n" +
					"  participant Browser\n" +
					"  participant Wizard\n" +
					"  participant KC as Keycloak\n" +
					"  Browser->>Wizard: GET /auth/login?identity=team-alpha\n" +
					"  Wizard->>Wizard: generate PKCE verifier/challenge + state\n" +
					"  Wizard-->>Browser: 302 to Keycloak (code_challenge, state)\n" +
					"  Browser->>KC: real hosted login page\n" +
					"  KC-->>Browser: 302 to /auth/callback (code, state)\n" +
					"  Browser->>Wizard: GET /auth/callback\n" +
					"  Wizard->>KC: POST /token (code + code_verifier)\n" +
					"  KC-->>Wizard: real access token\n" +
					"  Wizard-->>Browser: session cookie + redirect back into the wizard",
			},
			{
				ID:    "auth-accounts-impersonation",
				Title: "Accounts Agent → Core Banking (Impersonation)",
				Explanation: "The Accounts Agent takes your real session token as-is and presents it " +
					"straight to the Core Banking MCP server's gateway route; agentgateway performs a " +
					"Standard/Impersonation token exchange itself before forwarding - the same identity, " +
					"re-signed for this backend, with no act claim. That answers Question 1 (sub is still " +
					"you) and Question 2 (the only actor here is the gateway itself, impersonating - " +
					"nothing was delegated). It also answers Question 3: the exchanged token's aud is " +
					"scoped to the core banking backend, so even if it leaked, it couldn't be replayed " +
					"against any other service - a token minted for the ledger only works on the ledger.",
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
				Explanation: "Same agent, same MCP server, a different question: does this token carry " +
					"the right to call this exact tool right now? list_accounts and get_transactions are " +
					"open to any authenticated customer, but get_account_detail - which returns full " +
					"account and routing numbers - is gated by a CEL rule on the MCP backend requiring an " +
					"is_org_admin claim. Your session token doesn't carry it, so that one tool call is " +
					"denied even though the Impersonation exchange above succeeds cleanly every time. This " +
					"is Question 4: capability-at-the-tool, evaluated at the moment of the call, not once " +
					"at login. Ask the agent to list your accounts, then edit the prompt above and ask for " +
					"your full account number to see the same identity denied one tool over.",
				Diagram: "sequenceDiagram\n" +
					"  participant Accounts as Accounts Agent\n" +
					"  participant AGW as agentgateway\n" +
					"  participant Bank as Core Banking MCP\n" +
					"  Accounts->>AGW: tools/call list_accounts (Bearer exchanged token)\n" +
					"  AGW->>Bank: forward\n" +
					"  Bank-->>Accounts: accounts + balances\n" +
					"  Accounts->>AGW: tools/call get_account_detail (same token)\n" +
					"  AGW->>AGW: CEL: jwt.is_org_admin == true -> required\n" +
					"  AGW-->>Accounts: 403 denied - tool-level RBAC",
				Policies: []Policy{
					{
						ID:       "auth-accounts-rbac",
						Title:    "Core Banking standard route + tool RBAC",
						ReadOnly: true,
						ViewRefs: []ViewRef{
							{GVR: opsAgentHTTPRouteGVR, Name: "core-banking-standard"},
							{GVR: opsAgentPolicyGVR, Name: "core-banking-standard-rbac"},
						},
					},
				},
				OpsAgent: &OpsAgentDemo{
					Agent:         "accounts",
					DefaultPrompt: "List my accounts and recent transactions.",
				},
			},
			{
				ID:    "auth-transfer-chain",
				Title: "Transfer Orchestrator → Settlement → Core Banking (Delegation)",
				Explanation: "Moving money needs more than your own identity re-signed - it needs a " +
					"verified record of which agents acted along the way. The Transfer Orchestrator " +
					"exchanges your token at the STS, presenting its own Kubernetes ServiceAccount as " +
					"actor; the resulting token carries act=transfer-orchestrator-agent. It calls the " +
					"Settlement Agent with that token, not your raw one; Settlement exchanges it again, " +
					"presenting its own SA as actor. The token that finally calls Core Banking's transfer " +
					"tool carries a nested act: act.sub is Settlement, act.act.sub is the Orchestrator - a " +
					"signed chain of custody for every hop. That answers Question 1 (sub is still you " +
					"throughout) and Question 2 (act / act.act name exactly which agents acted, in order), " +
					"and Question 3: each exchange is scoped to the next hop's audience, so neither " +
					"intermediate token works anywhere but its intended destination.",
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
				Explanation: "The chain of custody from the previous step isn't just an audit trail - " +
					"it's the authorization decision. The transfer tool's CEL rule on the delegation route " +
					"requires jwt.act.act.sub to equal the Transfer Orchestrator's ServiceAccount " +
					"(system:serviceaccount:agentgateway-system:transfer-orchestrator-agent) before it " +
					"will move money; a direct call to transfer with your own token - same user, same " +
					"bank, no Orchestrator anywhere in the chain - is denied, even though your identity is " +
					"exactly as valid as it was a moment ago. This is Question 4 at its sharpest: holding a " +
					"capability (your token can reach this backend) and having been delegated to (the " +
					"Orchestrator vouches for you) are each necessary and neither is sufficient alone - the " +
					"call is allowed only where both are true, checked at the moment of the call. Run the " +
					"same \"move $500\" prompt again to see the chained path allowed; a raw direct " +
					"transfer that bypasses the Orchestrator entirely is denied the same way every time.",
				Diagram: "sequenceDiagram\n" +
					"  participant Settle as Settlement Agent (via chain)\n" +
					"  participant Direct as You (direct call, no chain)\n" +
					"  participant AGW as agentgateway\n" +
					"  participant Bank as Core Banking MCP\n" +
					"  Settle->>AGW: tools/call transfer (act.act.sub = transfer-orchestrator-agent)\n" +
					"  AGW->>AGW: CEL: jwt.act.act.sub == orchestrator SA -> Allow\n" +
					"  AGW->>Bank: forward\n" +
					"  Bank-->>Settle: transfer confirmed\n" +
					"  Direct->>AGW: tools/call transfer (your token, no act chain)\n" +
					"  AGW->>AGW: CEL: jwt.act.act.sub missing -> no match\n" +
					"  AGW-->>Direct: 403 denied - wrong task, wrong path",
				Policies: []Policy{
					{
						ID:       "auth-settlement-rbac",
						Title:    "Core Banking delegation route + actor-gated transfer RBAC",
						ReadOnly: true,
						ViewRefs: []ViewRef{
							{GVR: opsAgentHTTPRouteGVR, Name: "core-banking-delegation"},
							{GVR: opsAgentPolicyGVR, Name: "core-banking-delegation-rbac"},
						},
					},
				},
				OpsAgent: &OpsAgentDemo{
					Agent:         "transfer-orchestrator",
					DefaultPrompt: "Move $500 from savings to checking.",
				},
			},
			{
				ID:    "auth-statements-interactive",
				Title: "Statements Agent → Stripe (Interactive elicitation)",
				Explanation: "Not every credential an agent needs lives in Keycloak. The Statements " +
					"Agent calls Stripe's own real MCP server, in sandbox/test mode, on your behalf - but " +
					"the gateway has no Stripe token for you yet. Instead of failing, the STS returns a " +
					"one-time consent URL; after you approve it in the Solo UI, the STS stores a real " +
					"Stripe OAuth token and injects it into every later call automatically. Stripe's own " +
					"OAuth app advertises exactly one scope, mcp - there's no read-only grant to fall back " +
					"on, so this consent only ever proves you authorized some Stripe access, not which " +
					"Statements tools the agent may actually call; that's still Question 4's job, enforced " +
					"the same way as the banking tools above. The agent never sees or stores the Stripe " +
					"token itself.",
				Diagram: "sequenceDiagram\n" +
					"  participant You\n" +
					"  participant Wizard\n" +
					"  participant Stmt as Statements Agent\n" +
					"  participant AGW as agentgateway\n" +
					"  participant STS\n" +
					"  participant UI as Solo UI\n" +
					"  participant Stripe\n" +
					"  You->>Wizard: \"Show my recent Stripe payouts.\"\n" +
					"  Wizard->>Stmt: POST /statements-agent/run (token)\n" +
					"  Stmt->>AGW: MCP call (Bearer: your token)\n" +
					"  AGW->>STS: check for a stored Stripe token\n" +
					"  STS-->>AGW: none yet - elicitation required\n" +
					"  AGW-->>Stmt: pending + consent URL\n" +
					"  Stmt-->>Wizard: \"open this link to connect Stripe\"\n" +
					"  You->>UI: open the link, approve\n" +
					"  UI->>Stripe: OAuth consent (sandbox)\n" +
					"  Stripe-->>STS: Stripe access token (stored)\n" +
					"  You->>Wizard: retry\n" +
					"  Wizard->>Stmt: POST /statements-agent/run (token)\n" +
					"  Stmt->>AGW: MCP call (Bearer: your token)\n" +
					"  AGW->>STS: check for a stored Stripe token\n" +
					"  STS-->>AGW: found - inject it\n" +
					"  AGW-->>Stmt: real Stripe payouts",
				Policies: []Policy{
					{
						ID:       "auth-statements-interactive",
						Title:    "Statements Agent + Stripe (Interactive elicitation)",
						ReadOnly: true,
						ViewRefs: []ViewRef{
							{GVR: opsAgentHTTPRouteGVR, Name: "mcp-stripe"},
							{GVR: opsAgentPolicyGVR, Name: "stripe-mcp-elicitation"},
						},
					},
				},
				OpsAgent: &OpsAgentDemo{
					Agent:         "statements",
					DefaultPrompt: "Show my recent Stripe payouts.",
				},
			},
			{
				ID:    "auth-statements-brokered",
				Title: "Statements Agent → Stripe (Brokered elicitation)",
				Explanation: "Same agent, same goal, a smoother path. You already authenticated once, " +
					"back in the login step - there's no need to send you out to the Solo UI again just to " +
					"connect Stripe. This route's gateway-hosted OAuth issuer brokers only the missing " +
					"upstream credential inline: it shows its own consent screen for Stripe, banks the " +
					"resulting Stripe token keyed to your identity, and retries the call in the same round " +
					"trip. This is not a token exchange - the downstream token presented to the Statements " +
					"MCP route still identifies you directly; only the upstream Stripe credential is " +
					"brokered, and it's stored exactly the way the Interactive flow above stores it. What's " +
					"unchanged is Question 4 - the agent's tool access is still gated by MCP-level " +
					"authorization, exactly as it is in the interactive route. The contrast is the point: " +
					"Interactive sends you to a separate consent surface because nothing else trusts it yet; " +
					"Brokered folds that consent into the gateway's own issuer because your session already does.",
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
							{GVR: opsAgentPolicyGVR, Name: "stripe-mcp-brokered"},
						},
					},
				},
				OpsAgent: &OpsAgentDemo{
					Agent:         "statements",
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
			{
				ID:    "guardrails-banking-inbound",
				Title: "A valid token doesn't vouch for the ask",
				Explanation: "That's the general case. For a bank, the inbound half of the story needs " +
					"its own policy: a valid token says who's asking, not whether what they're asking for " +
					"is a legitimate banking request. agentgateway rejects prohibited asks - soliciting " +
					"another customer's account data, a full card number - on the way in, before the " +
					"request ever reaches the model.",
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
				Explanation: "Auth already decided who's asking and whether the ask itself is allowed - that " +
					"question is answered before the request ever reaches this step. Guardrails ask a " +
					"different one: once the model answers, is the content itself safe to send back? " +
					"The caller was fully authorized. The gateway still won't let the full account number " +
					"leave. agentgateway masks account and routing numbers out of the response entirely, " +
					"and reveals only the last 4 digits of a card number, the same way it rejected " +
					"prohibited asks on the way in.",
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
