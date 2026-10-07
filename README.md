# agentgateway-demo-wizard

[![ci](https://github.com/day0ops/agentgateway-demo-wizard/actions/workflows/ci.yml/badge.svg)](https://github.com/day0ops/agentgateway-demo-wizard/actions/workflows/ci.yml)
[![license](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

A clickops, visual-first web app for walking a customer through agentgateway's top enterprise capabilities in a single call: model routing and automatic fallback, identity-aware access control, content-safety guardrails, cost attribution, and governed agent-to-MCP tool calls. Every step explains a capability, shows a generated diagram, lets the presenter apply and revert real agentgateway configuration against a live cluster, and drives real requests to show the raw result.

It pushes its own curated set of enterprise agentgateway CRD manifests to the cluster directly via `client-go` server-side apply.

## Project layout

```
.
├── cmd
│   └── wizard      # main entrypoint (cmd/wizard/main.go)
├── internal        # Go backend: HTTP API, k8s client, CRD manifests, demo scenarios
├── web             # React/Vite frontend, embedded into the Go binary at build time
├── Dockerfile      # container build for the wizard binary
├── go.mod          # Go module definition
├── go.sum          # Go dependency lockfile
├── Makefile        # build/run/test/lint targets
```

## Prerequisites

Provisioned ahead of the call, by `agentgateway-field-kit` - this repo provisions none of it:

- A Kubernetes cluster with **enterprise agentgateway** installed, including the shared `Gateway` resource.
- **Keycloak**, reachable at the host in `KEYCLOAK_HOST`, with the `agw-dev` realm provisioned via field-kit's `keycloak` addon. That addon seeds, among others: the public `agw-client-public` client (password grant enabled by the addon's own default, even though the profile only declares the `authorization-code` flow), the `agw-token-exchange` confidential client (secret `agw-token-exchange-secret`, Standard Token Exchange already enabled) the agent pillar's token-exchange step uses, and the demo identities the wizard authenticates as - `user1` (team_id `team-alpha`) and `user2` (team_id `team-beta`), both under the realm's shared `defaultPassword`. Set `DEMO_USER_PASSWORD` to whatever that profile's `defaultPassword` is (field-kit no longer hardcodes it either).
- Network access from wherever the wizard runs to both the gateway and Keycloak hosts, and to the Kubernetes API server.

## Environment variables

| Variable                             | Required | Purpose                                                                                                                          |
| ------------------------------------ | -------- | -------------------------------------------------------------------------------------------------------------------------------- |
| `AGW_HOST`                           | yes      | agentgateway hostname the wizard sends requests to                                                                               |
| `KEYCLOAK_HOST`                      | yes      | Keycloak hostname the wizard authenticates against                                                                               |
| `OPENAI_API_KEY`                     | yes      | used by every OpenAI-backed demo step                                                                                            |
| `ANTHROPIC_API_KEY`                  | yes      | used by Pillar 1's model-by-name routing step                                                                                    |
| `OAUTH_TOKEN_EXCHANGE_CLIENT_SECRET` | yes      | Keycloak client secret for the agent pillar's token-exchange step (`agw-token-exchange` client, Standard Token Exchange enabled) |
| `DEMO_USER_PASSWORD`                 | yes      | shared password for the `user1`/`user2` demo identities, matching field-kit's realm `defaultPassword` - not hardcoded            |
| `STOCK_SERVER_MCP_IMAGE`             | no       | overrides the stock-lookup MCP server image                                                                                      |
| `CURRENCY_SERVER_MCP_IMAGE`          | no       | overrides the currency-lookup MCP server image                                                                                   |
| `WIZARD_ADDR`                        | no       | address the wizard listens on (default `:8080`)                                                                                  |
| `KUBECONFIG`                         | no       | kubeconfig path when running outside the cluster (falls back to in-cluster config, then `~/.kube/config`)                        |

`GET /api/config` reports which of the six required variables are unset; the UI shows a setup banner rather than failing silently on first click.

`STOCK_SERVER_MCP_IMAGE`/`CURRENCY_SERVER_MCP_IMAGE` default to Solo-internal Google Artifact Registry images. If the target cluster can't pull from that private registry, override both with images it can reach.

## Kubernetes access

The wizard needs a `ServiceAccount` (in-cluster) or `KUBECONFIG` (out-of-cluster) with `get`/`list`/`watch`/`create`/`patch`/`delete` on, in the `agentgateway-system` namespace: `gateways`, `httproutes`, `enterpriseagentgatewaybackends`, `enterpriseagentgatewaypolicies`, `enterpriseagentgatewayparameters`, `enterpriseagentgatewaybudgets`, `ratelimitconfigs`, `configmaps`, `secrets`, `deployments`, `services`, `serviceaccounts`.

## Running it

```bash
# one-time
cd web && npm install && cd ..

# every run
export AGW_HOST=agentgateway.demo.example.com
export KEYCLOAK_HOST=keycloak.demo.example.com
export OPENAI_API_KEY=sk-...
export ANTHROPIC_API_KEY=sk-ant-...
export OAUTH_TOKEN_EXCHANGE_CLIENT_SECRET=agw-token-exchange-secret
export DEMO_USER_PASSWORD=Password1!

make build
make run              # serves on :8080
```

For frontend-only iteration, run the Go backend (`make dev`) in one terminal and the Vite dev server (`cd web && npm run dev`) in another - `web/vite.config.ts` proxies `/api` to `:8080`.

## Tests

```bash
make test                       # Go unit tests (internal/...)
cd web && npm test -- --run     # Vitest component tests
cd web && npm run test:e2e      # Playwright, against a mocked backend - no cluster needed
make lint                       # golangci-lint
```

## The demo script

| #   | Pillar                                        | Steps                                                                                                 | Proves                                                              |
| --- | --------------------------------------------- | ----------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------- |
| -   | Welcome (~2 min)                              | Welcome to Acme Corp                                                                                  | the persona and the story spine                                     |
| 1   | One endpoint, many models (~12 min)           | Model routing by name, Dynamic routing, Automatic fallback, Streaming responses                       | one stable endpoint, many providers, automatic resilience           |
| 2   | Who's allowed to do what (~10 min)            | JWT authentication, Team / role authZ                                                                 | identity enforced at the edge, deny-by-default authorization        |
| 3   | What gets through (~3 min)                    | Not everything should get through                                                                     | unsafe content blocked inline, sensitive data masked on the way out |
| 4   | You can't govern what you can't see (~13 min) | Cost attribution, Budgets and spend limits (plus live virtual-key create/rotate)                      | per-request pricing, hard spend ceilings                            |
| 5   | From models to agents (~15 min)               | Agent to MCP tool call, MCP authentication, Token exchange (OBO), Tool authZ / progressive disclosure | the same gateway governs agentic tool traffic                       |
| -   | Wrap-up (~1 min)                              | One gateway, five guarantees                                                                          | the through-line: one control point, five guarantees                |

Pillars 1-4 are independent - each step's resources are uniquely named and independently revertible, so they can be shown in any order a customer's interest dictates. Pillar 5's four steps are intentionally cumulative (5.2, 5.3, and 5.4 build on 5.1's MCP backend and route) and are meant to be shown in order, matching how the capability actually layers in a real deployment.

A **Reset** affordance (calling `POST /api/config/reset`) reverts every policy applied so far, returning the cluster to its pre-provisioned baseline between rehearsals or back-to-back calls.

Pillar 4's `cost-budgets` step is the one exception to "independent and revertible in any order": its policy applies `apiKeyAuthentication: Strict` to the shared `Gateway`, matching the real product's own budget-enforcement pattern. While it's applied, every other route also requires a valid virtual API key, so presenters should revert `cost-budgets` (or hit the global Reset) before demonstrating any other pillar. The same step's virtual-key panel shares that caveat: any key created or rotated live is also subject to the Gateway-wide `apiKeyAuthentication: Strict` while `cost-budgets` is applied.

## Known limitations

- `get_exchange_rate`'s `{from, to}` argument schema (`cmd/wizard/main.go`) is this wizard's best-effort guess, not a confirmed contract with that MCP server - `get_stock_price`'s `{symbol}` schema is the one directly confirmed from the field-kit's own usecase tests.
- The two MCP server container images are Solo-internal GAR images (see `STOCK_SERVER_MCP_IMAGE`/`CURRENCY_SERVER_MCP_IMAGE` above).
- The wizard assumes a single presenter/session at a time - the Kubernetes applier's apply/revert tracking is in-memory and process-global, not per-browser-session.
- `RevertPatch`'s exact field-release behavior (Task 14) is verified against a real cluster's server-side apply, not the fake dynamic client used in unit tests; its non-deletion safety property is what the unit tests assert.
