# Banking Auth Pillar Redesign - Design

**Status:** Draft for review
**Supersedes the auth-pillar narrative in:** `2026-10-09-auth-pillar-token-exchange-rework-design.md`, `2026-10-10-auth-pillar-agents-mcp-design.md` (mechanisms stay; the git/GitHub story is retired)

## Problem

The "Who's allowed to do what" pillar teaches each token mechanism twice - once against a generic echo backend, then again with a real agent - and the agents it uses (repo / release-manager / deploy / github) tell a git-operations story that doesn't land as viscerally as it should. The mechanisms are correct and working end to end, but the demo doesn't give a presenter a crisp, consistent answer to the questions an audience actually cares about:

1. Who is being acted on behalf of?
2. Who is actually performing the action?
3. Is the token intended for this specific service?
4. Does this agent have the right to do this exact task right now?

Two capabilities that directly answer these are also absent from the current demo: MCP tool-level RBAC (the sharpest answer to question 4), and Brokered elicitation (the smoother counterpart to the Interactive elicitation we already ship).

## Goal

Re-cast the pillar around a banking/finance story where every step is a real agent calling a real MCP server, each step explicitly answers one or more of the four questions, and the sequence builds a single throughline:

**read your own money** (Impersonation) -> **move money, which requires a verified chain** (Delegation + chain of custody + exact-task RBAC) -> **reach an external processor** (Elicitation, Interactive then Brokered).

The pillar then hands off cleanly to "What gets through": auth decided *who / what / whether-allowed*; guardrails decide *what content is safe to return*, even for a call that passed every check.

## The four questions (intro step)

A new non-interactive intro step opens the pillar and frames the whole thing. It maps each question to a concrete token mechanism and claim, borrowing the subject/actor/intersection vocabulary from Block's "Who's asking?" identity-delegation framework:

| Question | Mechanism in this pillar | Token evidence |
|---|---|---|
| Who is being acted on behalf of? | real identity, propagated unchanged | `sub` |
| Who is actually performing the action? | STS token exchange / delegation | `act`, nested `act.act` for multi-hop |
| Is the token intended for this specific service? | audience-scoped exchange | `aud` |
| Does this agent have the right to this exact task right now? | MCP tool-level RBAC (CEL per tool call) | authorization = capability grant ∩ delegated consent |

The intersection framing (question 4) is the key teaching point: holding a capability is not enough, and having been delegated to is not enough; the action is allowed only where both are true, evaluated at the moment of the tool call.

## Cast: agents and MCP servers

All four existing agent wrappers are **repurposed** (thin ADK wrappers - only the instruction and MCP URL change); `repo-server` is **replaced** by a new Core Banking MCP. The git/GitHub story is fully retired.

| New role | Repurposed from | Talks to | Mechanism it showcases |
|---|---|---|---|
| **Accounts Agent** | repo-agent | Core Banking MCP (internal) | Impersonation; MCP RBAC (read-gating) |
| **Transfer Orchestrator Agent** | release-manager-agent | Settlement Agent | Delegation (hop 1) |
| **Settlement Agent** | deploy-agent | Core Banking MCP (internal) | Delegation (hop 2) + chain of custody; MCP RBAC (actor-gating) |
| **Statements Agent** | github-agent | Stripe sandbox MCP (external) | Elicitation: Interactive, then Brokered |

### Core Banking MCP (new, internal)

A new FastMCP server in `ops-agents/mcp-servers/`, same shape and build pattern as `repo-server` (fake-but-relatable data, no auth logic of its own - agentgateway fronts it). Tools, illustrative:

- `list_accounts()` - the customer's own accounts (checking, savings), balances - open to any authenticated customer.
- `get_transactions(account)` - recent transactions for an account - open to any authenticated customer.
- `get_account_detail(account)` - full account/routing numbers - **RBAC-gated** (sensitive; only a privileged claim may call it). This is the read-gating RBAC demo.
- `transfer(from_account, to_account, amount)` - move funds (write) - **RBAC-gated on the actor**: callable only when the token's `act` proves the call arrived via the Transfer Orchestrator, not a raw direct call. This is the actor-gating RBAC demo and the payload the chain-of-custody step moves.

### Stripe sandbox MCP (external, hosted)

`https://mcp.stripe.com`, OAuth, test mode (fully isolated from live - no real money moves). Read tools used by the Statements Agent: `stripe_api_read` (list charges / payouts / balance transactions, retrieve balance), `get_stripe_account_info`, `get_balance_summary`.

Narrative gift: Stripe's OAuth advertises exactly **one scope, `mcp`** - there is no read-only grant. So fine-grained control over what the Statements Agent may do can *only* come from our MCP tool-level RBAC, not from Stripe's own scopes. Same lesson GitHub taught, now structurally unavoidable, and a natural callback to question 4.

## Step-by-step pillar design (8 steps)

Replaces the current 8 steps. The two echo-backend warm-ups (`auth-exchange-standard`, `auth-exchange-sts`) are dropped; their mechanisms now live in the agent steps.

1. **The four questions** (intro, no request). Frames the pillar with the table above and a diagram showing `sub` / `act` / `aud` / per-tool authz on one request.
2. **Real login** (PKCE). Unchanged in substance - establishes the real `sub` every later step carries. Lightly reframed to name question 1.
3. **Accounts Agent -> Core Banking (Impersonation).** "What's my balance?" The agent presents your token; agentgateway exchanges it for a backend-`aud`-scoped token (same identity, re-signed, no `act`) before forwarding. Answers q1 (sub propagated), q2 (actor = gateway impersonating, no act claim), and - newly explained - q3: *why* the audience scoping matters (a token minted for the ledger can't be replayed elsewhere).
4. **Accounts Agent -> Core Banking (MCP RBAC, read-gating).** Same agent, same MCP. `list_accounts` / `get_transactions` succeed for any authenticated customer; `get_account_detail` (full account numbers) is denied unless the JWT carries the privileged claim. Answers q4 as capability-at-the-tool. This is where the old team-alpha/team-beta RBAC lesson lands, now on a real tool call with real stakes.
5. **Transfer Orchestrator -> Settlement -> Core Banking (Delegation + chain of custody).** "Move $500 from savings to checking." Orchestrator exchanges your token at the STS (actor = Orchestrator SA), calls the Settlement Agent with the exchanged token, which exchanges again (actor = Settlement SA) before calling `transfer`. Resulting token carries nested `act`: `act.sub` = Settlement, `act.act.sub` = Orchestrator - a signed record of every hop. Answers q1 + q2 (the full actor chain) + q3.
6. **Settlement -> Core Banking (MCP RBAC, actor-gating).** The `transfer` tool's CEL requires the `act` chain prove the call arrived via the Orchestrator; a raw direct call to `transfer` (same user, no chain) is denied. Answers q4 as *right-task-right-now*: identity alone is insufficient, the delegation path itself is part of the authorization decision. Ties chain-of-custody and exact-task authz into one demonstration.
7. **Statements Agent -> Stripe (Interactive elicitation).** "Show my Stripe payouts." No banked Stripe credential yet -> the STS returns a one-time consent URL -> the user approves in Solo UI -> the STS banks a real Stripe token and injects it on retry. The flow already built for GitHub, re-pointed at Stripe sandbox.
8. **Statements Agent -> Stripe (Brokered elicitation).** Same agent, same goal, smoother flow: because the user already authenticated (step 2), the gateway's own OAuth issuer brokers only the upstream Stripe credential inline, showing its own gateway-hosted consent screen, and banks it keyed to the user's identity - no separate trip to Solo UI. Explicit contrast with step 7. (Not a token exchange: the downstream token identifies the user; the upstream Stripe credential is brokered separately and stored the same way Interactive stores it.)

## Guardrails tie-in: "What gets through"

Auth answered who / what / whether-allowed. Guardrails answer what content is safe to return - even for a call that passed every check. The pillar gains both directions:

- **Inbound block.** Reject prompts that try to exfiltrate or solicit prohibited financial data on the way in (extends the existing SSN / credit-card block pattern to banking-specific prohibited asks).
- **Outbound mask.** An *authorized* Accounts or Statements response still has sensitive financial fields masked on the way out - full account numbers -> last-4, routing numbers, card PANs - by agentgateway, before the data reaches the caller.

The bridge line a presenter uses to move from pillar to pillar: "The caller was fully authorized. The gateway still won't let the full account number leave."

## Cross-repo scope

This is one cohesive narrative but the implementation spans three repos with sequential dependencies (wizard narrative references agents/MCPs that ops-agents builds and field-kit deploys). Expect three implementation plans, one per repo, in dependency order.

### ops-agents (build)
- New Core Banking MCP server (`mcp-servers/core-banking`), repo-server-style.
- Repurpose the four agent wrappers: new instructions + MCP URLs (repo->Accounts, release-manager->Transfer Orchestrator, deploy->Settlement, github->Statements). Statements keeps the elicitation probe logic already built.
- New container images; retire repo-server image.

### field-kit (config + one feature extension)
- Usecase steps for the new agents/MCP/routes.
- Extend `elicitation-backend` (or add a sibling feature) to emit Brokered elicitation policy + the gateway OAuth-issuer `consent` config, alongside the existing Interactive path.
- MCP tool-level RBAC policy wiring (`backend.mcp.authorization` CEL) - likely a small new feature or an extension to the MCP-server feature.
- Stripe OAuth app registration + client secret (env-var driven, like the GitHub one).
- Env image-tag bumps.

### demo-wizard (this repo: narrative + manifests)
- Rewrite `authPillar()` in `internal/scenarios/registry.go` to the 8 steps above.
- Extend `guardrailsPillar()` with the inbound-block + outbound-mask banking steps.
- Manifest templates for the new routes/policies the read-only steps display.
- Agent-run plumbing reuses the existing `OpsAgentDemo` mechanism.

## Non-goals

- No real money: Stripe stays in sandbox/test mode throughout.
- No GitHub App / fine-grained-PAT migration: the OAuth-scope coarseness (Stripe's single `mcp` scope, like GitHub's `repo`) is deliberately left as the motivation for MCP RBAC, not worked around.
- No change to the routing, cost, or other pillars beyond the guardrails tie-in.
- No new identity provider or realm topology work; the single-realm `agw-dev` setup stands.

## Risks / to verify during implementation

- **Brokered elicitation feature support in field-kit.** The current `elicitation-backend` feature only emits the Interactive policy shape. Brokered needs the gateway OAuth-issuer config (`KGW_OAUTH_ISSUER_CONFIG` with a `consent` block) plus `entElicitation.brokered.chainedAuth.oauth` on the policy. Confirm the exact CRD/Helm shape against a live deploy before finalizing the field-kit plan.
- **Stripe MCP human-confirmation on writes.** Stripe requires human confirmation for certain `stripe_api_write` actions (refunds, outbound payments). The Statements Agent is read-only by design, so this shouldn't surface - but keep the Statements demo strictly on `stripe_api_read` to avoid an unexpected confirmation gate.
- **Actor-gating CEL expressiveness.** Confirm the MCP authorization CEL can reference the nested `act` chain (not just `jwt.sub` and `mcp.tool.name`) to express "arrived via the Orchestrator." If it can only see the outermost actor, step 6 narrows to "arrived via *a* delegating agent" rather than naming the Orchestrator specifically - still a valid lesson, but adjust the copy.
- **Core Banking MCP identity.** The internal MCP has no auth of its own; it returns the same fake data regardless of caller. The RBAC gating happens entirely at agentgateway via CEL on claims, so the demo's "privileged claim" and "actor chain" must be real claims the deployed tokens actually carry. Verify against the `agw-dev` token shape.

## Verification plan

- Each repo's plan ends with its own tests (ops-agents: pytest on the MCP + agent wrappers; field-kit: bun tests on the new/extended features; wizard: Go tests on the registry + web tests on the step rendering).
- End-to-end, live, in presentation order: run each of the 8 steps through the deployed wizard as a presenter would, confirming the four-questions narration matches observed behavior (token claims at each hop, RBAC allow/deny, both elicitation flows, inbound block + outbound mask).
- Explicitly verify the two "deny" demonstrations actually deny: `get_account_detail` without the privileged claim, and a raw `transfer` call without the Orchestrator chain.
