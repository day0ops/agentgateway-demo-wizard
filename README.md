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
| `OAUTH_TOKEN_EXCHANGE_CLIENT_SECRET` | yes      | Keycloak client secret for the agent pillar's token-exchange step (`agw-token-exchange` client, Standard Token Exchange enabled) |
| `DEMO_USER_PASSWORD`                 | yes      | shared password for the `user1`/`user2` demo identities, matching field-kit's realm `defaultPassword` - not hardcoded            |
| `STOCK_SERVER_MCP_IMAGE`             | no       | overrides the stock-lookup MCP server image                                                                                      |
| `CURRENCY_SERVER_MCP_IMAGE`          | no       | overrides the currency-lookup MCP server image                                                                                   |
| `WIZARD_ADDR`                        | no       | address the wizard listens on (default `:8080`)                                                                                  |
| `KUBECONFIG`                         | no       | kubeconfig path when running outside the cluster (falls back to in-cluster config, then `~/.kube/config`)                        |

`GET /api/config` reports which of the five required variables are unset; the UI shows a setup banner rather than failing silently on first click.

`STOCK_SERVER_MCP_IMAGE`/`CURRENCY_SERVER_MCP_IMAGE` default to Solo-internal Google Artifact Registry images. If the target cluster can't pull from that private registry, override both with images it can reach.

The routing pillar's 3 "provider" policies (model-by-name, dynamic-tier, fallback) are pre-provisioned by the agentgateway-field-kit usecase that deploys this wizard, not applied by the wizard itself - the Configure pane shows their live config read-only. See agentgateway-field-kit's docs/superpowers/specs/2026-10-08-demo-wizard-field-kit-integration-design.md for the resource-naming contract.

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