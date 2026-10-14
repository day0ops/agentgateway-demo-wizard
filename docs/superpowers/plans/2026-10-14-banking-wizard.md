# Banking Wizard Narrative Implementation Plan (Plan 3 of 3)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
>
> **Depends on Plan 1 (agent/MCP names, OpsAgent `/run` contract) and Plan 2 (field-kit routes, RBAC policies, Stripe elicitation backends, and the Task 0.C Interactive-on-Stripe verdict).**

**Goal:** Rewrite the "Who's allowed to do what" pillar as the banking story - an intro framing the four questions, then seven real agent/MCP steps teaching Impersonation, MCP-RBAC (read + actor), Delegation+chain, and Elicitation (Interactive + Brokered) - and extend "What gets through" with banking guardrails (inbound block + outbound mask). The wizard is a pure renderer of `internal/scenarios`; this plan is almost entirely registry + handler-rename + two guardrail manifests.

**Architecture:** `internal/scenarios/registry.go` is the single source of truth; the frontend renders it via `GET /api/scenarios`. Agent-driven steps use `OpsAgentDemo` -> `POST /api/ops-agent/run` -> `Server.opsAgentURL`. Read-only policy cards use `ViewRef`s pointing at field-kit-provisioned resources (no manifests). Guardrail steps Apply/Revert wizard-owned manifest templates (the existing prompt-guard pattern).

**Tech Stack:** Go (registry, handlers, config), Go tests; the web frontend's step-rendering tests (`web/src/**/*.test.tsx`). Mermaid diagrams as inline strings.

## Global Constraints

- **Agent names in `OpsAgentDemo.Agent` must match `Server.opsAgentURL` cases** after rename: `accounts`, `transfer-orchestrator`, `statements`. (Settlement is called agent-to-agent by the Transfer Orchestrator, never directly by the wizard - no case for it, mirroring how `deploy` had none.)
- **Every step's narration explicitly names which of the four questions it answers** - this is the pillar's organizing principle, not decoration.
- **ViewRef GVRs/names must match the field-kit-provisioned resources** from Plan 2 Task 4 (route names `core-banking-standard`, `core-banking-delegation`, `settlement-agent`, `mcp-stripe`, `mcp-stripe-brokered`; the MCP-RBAC and elicitation policy names). Confirm exact names against Plan 2 before writing ViewRefs.
- **Interactive-vs-Brokered copy branches on Plan 2 Task 0.C:** if Interactive works on Stripe, both steps say Stripe; if not, the Interactive step says GitHub and explicitly contrasts "classic OAuth app (Interactive) vs DCR provider (Brokered)."
- **Each task ends green:** `go test ./...` and the web test suite pass.

---

## File Structure

- `internal/scenarios/registry.go` - rewrite `authPillar()`, extend `guardrailsPillar()`.
- `internal/api/ops_agent_handler.go` - rename the `opsAgentURL` cases.
- `internal/config` (wherever `RepoAgentURL`/`ReleaseManagerAgentURL`/`GitHubAgentURL` and their env bindings live) - rename to `AccountsAgentURL`/`TransferOrchestratorAgentURL`/`StatementsAgentURL`.
- `internal/manifests/templates/guardrails/banking-inbound.yaml.tmpl` (new), `.../banking-outbound-mask.yaml.tmpl` (new).
- Tests: `internal/scenarios/*_test.go`, `internal/api/*_test.go`, `web/src/App.test.tsx` + `web/src/components/WelcomeFeatures.test.tsx` (update the asserted step titles).

---

### Task 1: Rename the ops-agent handler + config wiring

**Files:** `internal/api/ops_agent_handler.go`, the config struct + env binding, `internal/api/*_test.go`.

**Interfaces:**
- Produces: `opsAgentURL` cases `accounts`, `transfer-orchestrator`, `statements`; config fields `AccountsAgentURL`, `TransferOrchestratorAgentURL`, `StatementsAgentURL` bound to env (rename the existing `*_AGENT_URL` env vars to match; update deployment env in field-kit's wizard feature if it sets them - cross-check `features/agentgateway-demo-wizard/index.js`).

- [ ] **Step 1: Find the config fields + env bindings** - `grep -rn "RepoAgentURL\|ReleaseManagerAgentURL\|GitHubAgentURL" internal/`.
- [ ] **Step 2: Write failing test** - a handler test asserting `opsAgentURL("accounts")` / `("transfer-orchestrator")` / `("statements")` resolve, and an unknown name errors.
- [ ] **Step 3: Rename** the three config fields, their env var names, and the three `switch` cases.
- [ ] **Step 4: Check the wizard deployment feature** - if `features/agentgateway-demo-wizard/index.js` (field-kit) injects `*_AGENT_URL` env, note the rename for a field-kit follow-up (cross-repo; record it, don't fix here).
- [ ] **Step 5: Tests pass. Commit** - `refactor(wizard): rename ops-agent handler + config to banking agents`

---

### Task 2: Rewrite `authPillar()` - intro + login + 6 mechanism steps

**Files:** `internal/scenarios/registry.go` (`authPillar`), `internal/scenarios/*_test.go`.

**Interfaces:**
- Consumes: `opsAgentHTTPRouteGVR` (exists); add a GVR const for `EnterpriseAgentgatewayPolicy` if any step ViewRefs an RBAC/elicitation policy.
- Produces: an 8-step `authPillar` with IDs `auth-questions`, `auth-login`, `auth-accounts-impersonation`, `auth-accounts-rbac`, `auth-transfer-chain`, `auth-settlement-rbac`, `auth-statements-interactive`, `auth-statements-brokered`.

- [ ] **Step 1: Write failing tests** - assert `authPillar()` has those 8 step IDs in order; the intro step has no `Presets`/`OpsAgent`; the four agent steps have `OpsAgent.Agent` in {`accounts`,`transfer-orchestrator`,`statements`}; `auth-login` keeps `LoginDemo: true`.
- [ ] **Step 2: Run, expect fail.**
- [ ] **Step 3: Write the intro step** (`auth-questions`): no request; `Explanation` lays out the four questions and maps each to `sub`/`act`/`aud`/per-tool-authz; `Diagram` a single-request flow annotating all four. Teaser-level framing of subject vs actor vs audience vs authorization-as-intersection.
- [ ] **Step 4: Keep `auth-login`** (PKCE) nearly verbatim; reframe its explanation to name question 1 (establishing the real `sub`).
- [ ] **Step 5: `auth-accounts-impersonation`** - `OpsAgent{Agent:"accounts", DefaultPrompt:"What's my balance and recent activity?"}`, ReadOnly ViewRefs to the `core-banking-standard` route; explanation names q1+q2 (gateway impersonates, no `act`) + q3 (audience-scoped token can't be replayed elsewhere); sequence diagram You->Wizard->Accounts->AGW(impersonation exchange)->Core Banking.
- [ ] **Step 6: `auth-accounts-rbac`** - same `accounts` agent, two presets/prompts: one that lists accounts (allowed) and one that asks for full account numbers (`get_account_detail`, denied for a non-privileged identity). Explanation names q4 as capability-at-the-tool; diagram shows the CEL allow/deny at the MCP backend.
- [ ] **Step 7: `auth-transfer-chain`** - `OpsAgent{Agent:"transfer-orchestrator", DefaultPrompt:"Move $500 from savings to checking."}`; ViewRefs to `transfer-orchestrator`, `settlement-agent`, `core-banking-delegation` routes; explanation names q1+q2 (nested `act.act`) + q3; two-hop sequence diagram with STS exchanges and the nested-act result.
- [ ] **Step 8: `auth-settlement-rbac`** - explanation names q4 as right-task-right-now: the `transfer` tool's CEL requires the Orchestrator in the `act` chain; a raw direct `transfer` (same user, no chain) is denied. Diagram contrasts chained-allowed vs direct-denied. (Drives the same transfer-orchestrator prompt for the allow path; the deny path is explained, or driven via a direct-call preset if one exists.)
- [ ] **Step 9: `auth-statements-interactive`** - `OpsAgent{Agent:"statements", DefaultPrompt:"Show my recent Stripe payouts."}`; ViewRefs to `mcp-stripe` route + its elicitation policy; explanation = Interactive elicitation (pending consent URL -> approve in Solo UI -> retry). Copy branches on Plan 2 Task 0.C (Stripe vs GitHub).
- [ ] **Step 10: `auth-statements-brokered`** - same `statements` agent, Brokered route `mcp-stripe-brokered`; explanation contrasts with the previous step: user already authenticated, so the gateway's own OAuth issuer brokers only the upstream Stripe credential inline via its gateway-hosted consent screen - not a token exchange. Diagram shows the issuer-coordinated flow.
- [ ] **Step 11: Run tests, expect pass.** **Commit** - `feat(wizard): banking auth pillar narrative (8 steps)`

---

### Task 3: Guardrails - inbound block (banking prohibited asks)

**Files:** `internal/manifests/templates/guardrails/banking-inbound.yaml.tmpl` (new), `internal/scenarios/registry.go` (`guardrailsPillar`), tests.

**Interfaces:**
- Produces: a new guardrails step applying a wizard-owned `EnterpriseAgentgatewayPolicy` with `entPromptguard.request` rejecting banking-prohibited content (reuse the PII pattern approach from `prompt-guard.yaml.tmpl`, add banking-specific custom phrases). Targets the guardrails demo route (same `/guardrails/openai` the existing step uses).

- [ ] **Step 1: Write the template** - model on `prompt-guard.yaml.tmpl`; `entPromptguard.request` with built-in PII patterns + custom regexes/phrases for prohibited financial asks; action reject.
- [ ] **Step 2: Write failing test** - `guardrailsPillar()` includes the new step with the right `Policy.ManifestPath`; template renders to valid YAML via `manifests.Render`.
- [ ] **Step 3: Add the step** to `guardrailsPillar()` with explanation + presets (a safe ask; a prohibited ask that's rejected inbound).
- [ ] **Step 4: Tests pass. Commit** - `feat(wizard): banking inbound guardrail step`

---

### Task 4: Guardrails - outbound mask (financial fields)

**Files:** `internal/manifests/templates/guardrails/banking-outbound-mask.yaml.tmpl` (new), `registry.go`, tests.

**Interfaces:**
- Produces: a guardrails step applying a policy with `entPromptguard.response` masking account-number / routing-number / card-PAN patterns on the response path. Target the route the agent's LLM answer flows through (confirm the exact route during implementation - the agent's final natural-language answer is where account numbers would appear; mask there).

- [ ] **Step 1: Write the template** - `entPromptguard.response` with mask patterns for 9+-digit account numbers, 9-digit routing numbers, and card PANs (last-4 reveal). Confirm the response-masking field shape against the EntPromptguardResponse CRD type in agentgateway-enterprise.
- [ ] **Step 2: Write failing test** - step present with the template path; renders valid YAML.
- [ ] **Step 3: Add the step** - explanation carries the pillar bridge line: "The caller was fully authorized. The gateway still won't let the full account number leave." Preset drives an authorized Accounts/Statements query whose answer would contain a full number, showing it masked.
- [ ] **Step 4: Tests pass. Commit** - `feat(wizard): banking outbound masking guardrail step`

---

### Task 5: Update frontend step-title tests + full suite

**Files:** `web/src/App.test.tsx`, `web/src/components/WelcomeFeatures.test.tsx`, any snapshot/registry web tests referencing old step titles.

- [ ] **Step 1: Update** the asserted titles/teasers to the new pillar (the pillar Title "Who's allowed to do what" can stay or be resharpened - if changed, update everywhere).
- [ ] **Step 2: Run** `go test ./...` and the web suite; both green.
- [ ] **Step 3: Commit** - `test(wizard): update step assertions for the banking pillar`

---

### Task 6: End-to-end render + drive

- [ ] Build+run the wizard against the deployed banking usecase; walk all 8 auth steps and both guardrail steps as a presenter; confirm each agent step drives the right container, each ViewRef resolves, both deny paths deny, both elicitation flows work, and the outbound mask actually masks a real account number in an authorized answer.
- [ ] Commit any copy/diagram/ViewRef corrections.

## Self-Review notes

- The wizard is a renderer: the bulk is `registry.go` strings + two manifests + a handler rename. No new agent-loop or API surface.
- Cross-repo coupling points are called out (ViewRef names from Plan 2; `*_AGENT_URL` env from field-kit's wizard feature) so they're verified, not assumed.
- Guardrails close the loop to the next pillar with the explicit "authorized, still masked" bridge - the stated goal of the tie-in.
- Outbound-mask target route and the exact `entPromptguard.response` shape are the two things to confirm live during Task 4 (flagged, with the CRD type named as the source of truth).
