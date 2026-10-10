# Auth pillar: agents and MCP servers - Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add three agent-driven demo moments to the wizard's auth pillar - a single-hop agent (Standard vs. STS Delegation), a two-hop STS re-exchange chain (nested `act`, chain-of-custody), and an Elicitation flow against real GitHub - backed by real deployed agent + MCP-server containers.

**Architecture:** Five new container images live in a new monorepo `github.com/day0ops/ops-agents` (one MCP server + four ADK agents, Python). `agentgateway-field-kit` stands up the STS/Postgres infra, the gateway routes/policies, the agent + MCP deployments, and the Keycloak clients. This repo (`agentgateway-demo-wizard`) adds the new registry steps and the UI to trigger agents; its Go backend proxies each agent run, forwarding the user's captured session token (reusing the existing `oauthlogin.Store` + session-token request plumbing) to the agent's `POST /run`.

**Tech Stack:** Python 3.10+ (Google ADK `google-adk` 2.11.0, official `mcp` SDK 1.30.0 (NOT 2.x - FastMCP lives in the 1.x line; 2.x renamed it to MCPServer and changed the API shape entirely, live-verified), FastAPI, uvicorn), Docker, `agentgateway-field-kit` (Node/Helm/YAML usecase features), Go (stdlib `net/http`), React/TypeScript/Vitest.

## Global Constraints

- **Repos & registry:** all five images build from `github.com/day0ops/ops-agents` and push to `australia-southeast1-docker.pkg.dev/field-engineering-apac/kasunt/<name>:<tag>`, named after their directory (`repo-server`, `repo-agent`, `release-manager-agent`, `deploy-agent`, `github-agent`). CI/CD (`.github/workflows/ci.yml` + `release.yml`, mirroring `agentgateway-demo-wizard`'s pattern) pushes git-tag-based versions - tags are always `v`-prefixed (e.g. `v0.1.0`), never a bare `0.1.0`.
- **Exchange mechanisms (locked):** Standard/Impersonation = agentgateway proxy-native `backend.auth.oauthTokenExchange` (no STS). All delegation = Solo's controller-side STS (`issuer = enterprise-agentgateway.agentgateway-system.svc.cluster.local:7777`, `/oauth2/token`). Keycloak-native token exchange is NOT used.
- **STS exchange request shape (verified live):** POST form to the STS `/oauth2/token` with `grant_type=urn:ietf:params:oauth:grant-type:token-exchange`, `subject_token`, `subject_token_type=urn:ietf:params:oauth:token-type:jwt`, `actor_token`, `actor_token_type=urn:ietf:params:oauth:token-type:jwt`, `audience=agentgateway`, AND an `Authorization: Bearer <subject_token>` header. The STS accepts both Keycloak-issued and its own self-issued tokens as `subject_token` (confirmed by the design spike - this is what makes the two-hop re-exchange work).
- **ADK API (verified against 2.11.0 docs):** custom LLM via `from google.adk.models.lite_llm import LiteLlm` → `LiteLlm(model="openai/gpt-4o-mini", api_base=<.../openai>)`; MCP via `McpToolset(connection_params=StreamableHTTPConnectionParams(url=...), header_provider=<async(ReadonlyContext)->dict[str,str]>)`; serve via `InMemoryRunner` wrapped in FastAPI, `runner.run_async(...)` returns `AsyncGenerator[Event]` (iterate, pick `event.is_final_response()`); `new_message=types.Content(role="user", parts=[types.Part(text=...)])` from `google.genai`; per-request token placed in session state (`InMemorySessionService.create_session`, async) and read by `header_provider` from `ctx.state`.
- **MCP server API:** official `mcp` SDK `from mcp.server import FastMCP`; `@mcp.tool()` (schema inferred from type hints + docstring); `FastMCP(name=..., host="0.0.0.0", port=8000)` (host/port are constructor-only kwargs in 1.30.0, not run() kwargs), then `mcp.run(transport="streamable-http")` (default path `/mcp`).
- **Agent deployment contract (from field-kit):** each agent container exposes `POST /run` (`{"query": "...", ...}`) and `GET /health`; routed through agentgateway at `POST /<agent>/run`. LLM reached at the `/openai` route, MCP at the configured MCP route.
- **No AI attribution in commits/PRs.** Conventional Commits. Draft PRs assigned to `day0ops`. Every task's code must be complete, not placeholder.
- **GitHub OAuth app is a trailing placeholder:** Elicitation code/config is wired to `GITHUB_ISSUER_CLIENT_ID`/`GITHUB_ISSUER_CLIENT_SECRET` env with no fallback; real registration happens outside this plan before the Elicitation flow is demoed live.

## File / Phase Structure

Three phases, one per repo, each independently testable in its own toolchain. Within a phase, tasks are ordered so each ends with a testable deliverable.

**Phase A - `ops-agents` monorepo (Python/Docker), builds + unit-tests standalone, mocking STS/LLM/MCP:**
- A1 Scaffold monorepo (layout, shared pyproject/tooling, CI, README)
- A2 `mcp-servers/repo-server` - FastMCP server, 3 tools, Dockerfile, tests
- A3 `agents/_shared` - ADK agent factory + STS-exchange helper + FastAPI harness, tests
- A4 `agents/repo-agent` - single-hop, two modes (standard / sts-delegation), tests
- A5 `agents/release-manager-agent` - hop-1 exchange + call downstream agent, tests
- A6 `agents/deploy-agent` - hop-2 re-exchange + call MCP, surfaces act chain, tests
- A7 `agents/github-agent` - elicitation (GitHub MCP), surfaces elicitation URL, tests
- A8 Build + push all five images

**Phase B - `agentgateway-field-kit` (Node/Helm/YAML), testable by deploying + hitting endpoints:**
- B1 Fix a missing `preserveToken: true` bug in the shared `oauth-token-exchange` feature (B3 depends on it)
- B2 Enable the STS (Postgres + token-exchange) for the ops-agents usecase
- B3 Internal repo MCP server deployment + two gateway routes (standard / delegation) + policies
- B4 Agent deployments (four agents) via the `workload-agent` feature
- B5 GitHub remote-MCP backend + `entElicitation` policy (+ OAuth env placeholders)
- B6 Image references in `config/environments/aws-dev.yaml`

No separate "Keycloak clients" task: these agents authenticate as their own Kubernetes ServiceAccount (validated via the STS's `k8s` actorValidator, not a Keycloak client_credentials grant), and `workload-agent` already creates the ServiceAccount for each agent as part of B4.

**Phase C - `agentgateway-demo-wizard` (Go/React, this repo), Go + vitest tests + live:**
- C1 Config: agent endpoint URLs
- C2 Backend: agent-run proxy handler (forwards captured session token to agent `/run`)
- C3 Registry: four new auth-pillar steps
- C4 Frontend: agent-run UI for the new steps (+ elicitation link handling)
- C5 Full-repo verification pass

**Tracked follow-up (NOT in this plan):** reworking field-kit's `fd-loan-rbac-jwt-propagation` is deliberately excluded - it uses the propagate pattern for an intentional per-caller-cap lesson; changing it to re-exchange is a separate, deliberate field-kit change (rename + RBAC rework) with its own PR.

---

## Phase A - `ops-agents` monorepo

> Phase A is developed in the separate `github.com/day0ops/ops-agents` checkout, not this repo. Clone it first: `git clone https://github.com/day0ops/ops-agents && cd ops-agents`. All Phase A paths are relative to that repo root.

### Task A1: Scaffold the monorepo

**Files:**
- Create: `pyproject.toml`, `.gitignore`, `README.md`, `.github/workflows/ci.yml`, `Makefile`

**Interfaces:**
- Produces: the repo layout and shared tooling (ruff + pytest) every later task relies on; a `build-and-push` make target parameterized by service directory.

- [ ] **Step 1: Write the repo layout and shared config**

Create `pyproject.toml` at the repo root (shared dev tooling; each service pins its own runtime deps in its own `requirements.txt`):

```toml
[project]
name = "ops-agents"
version = "0.1.0"
description = "Operational agents and MCP servers for the agentgateway auth-pillar demo"
requires-python = ">=3.10"

[tool.pytest.ini_options]
pythonpath = ["agents", "mcp-servers"]
testpaths = ["agents", "mcp-servers"]
# Every agent's tests/ dir has no __init__.py and they all share the basename
# test_main.py - pytest's default "prepend" import mode can only ever resolve
# one bare "test_main" module across a whole collection run, so collecting
# from the repo root (what CI does) errors on the 2nd+ one with "import file
# mismatch". importlib mode resolves each by its real path instead. (Only
# surfaces once more than one agent exists - live-verified via CI after all
# four agents landed.)
addopts = "--import-mode=importlib"

[tool.ruff]
line-length = 100
target-version = "py310"
```

Create `.gitignore`:

```
__pycache__/
*.pyc
.venv/
.pytest_cache/
.ruff_cache/
*.egg-info/
```

Create `README.md` documenting the layout (the five services, how to build/test one, the image registry/naming convention from Global Constraints).

- [ ] **Step 2: Write the Makefile**

The build context is always the monorepo root (`.`), not the service directory - `-f $(SVC)/Dockerfile` just selects which Dockerfile to use. This is required because the four agent images need to `COPY agents/_shared` into the image, and Docker refuses `COPY`/`ADD` of any path outside the build context (live-verified). Every Dockerfile's `COPY` sources are therefore root-relative (e.g. `COPY mcp-servers/repo-server/requirements.txt .`), not relative to the Dockerfile's own location.

```makefile
# Usage: make build SVC=mcp-servers/repo-server IMAGE=repo-server TAG=v0.1.0
# TAG should match the latest git release tag (see .github/workflows/release.yml,
# Task A8) - CI always pushes v-prefixed tags, so a manual build should match them.
REGISTRY ?= australia-southeast1-docker.pkg.dev/field-engineering-apac/kasunt
TAG ?= v0.1.0

.PHONY: test lint build push
test:
	pytest -q
lint:
	ruff check .
build:
	docker build -t $(REGISTRY)/$(IMAGE):$(TAG) -f $(SVC)/Dockerfile .
push:
	docker push $(REGISTRY)/$(IMAGE):$(TAG)
```

- [ ] **Step 3: Write CI**

Create `.github/workflows/ci.yml` running `ruff check .` and `pytest -q` on push/PR (Python 3.10, install each service's `requirements.txt` + `requirements-dev.txt`). Mirror the demo-wizard repo's existing CI conventions (lint + test gate).

- [ ] **Step 4: Commit**

```bash
git add pyproject.toml .gitignore README.md Makefile .github/workflows/ci.yml
git commit -m "chore: scaffold ops-agents monorepo"
```

---

### Task A2: `repo-server` MCP server

**Files:**
- Create: `mcp-servers/repo-server/server.py`, `requirements.txt`, `requirements-dev.txt`, `Dockerfile`, `tests/test_server.py`

**Interfaces:**
- Produces: a StreamableHTTP MCP server on `:8000` path `/mcp` exposing `list_repos`, `get_pull_request_status`, `trigger_deploy`. Backend for the single-hop and two-hop flows. No auth logic of its own (agentgateway fronts it).

- [ ] **Step 1: Write failing tests**

Create `mcp-servers/repo-server/tests/test_server.py`. The tool logic lives in plain helper functions so it's unit-testable without a running server; thin `@mcp.tool()` wrappers (Step 3) just call them.

```python
from repo_server.server import _list_repos, _get_pull_request_status, _trigger_deploy

def test_list_repos_returns_seeded_repos():
    repos = _list_repos()
    names = [r["name"] for r in repos]
    assert "payments-service" in names
    assert all("name" in r and "default_branch" in r for r in repos)

def test_pr_status_shape():
    s = _get_pull_request_status("payments-service", 42)
    assert s["repo"] == "payments-service"
    assert s["number"] == 42
    assert s["state"] in {"open", "merged", "closed"}

def test_trigger_deploy_echoes_target():
    r = _trigger_deploy("payments-service", "staging")
    assert r["repo"] == "payments-service"
    assert r["environment"] == "staging"
    assert r["status"] == "queued"
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd mcp-servers/repo-server && python -m pytest -q`
Expected: FAIL - `repo_server` module doesn't exist yet.

- [ ] **Step 3: Write the server**

Create `mcp-servers/repo-server/repo_server/__init__.py` (empty) and `mcp-servers/repo-server/repo_server/server.py`:

```python
"""Internal "repo" MCP server - fake-but-relatable repo/deploy data for the
auth-pillar demo. Auth is handled by agentgateway in front of this server;
this process has no auth logic of its own."""
from mcp.server import FastMCP

mcp = FastMCP(name="repo-server", host="0.0.0.0", port=8000)

_REPOS = [
    {"name": "payments-service", "default_branch": "main", "open_prs": 3},
    {"name": "checkout-web", "default_branch": "main", "open_prs": 1},
    {"name": "fraud-rules", "default_branch": "main", "open_prs": 0},
]


def _list_repos() -> list[dict]:
    """Return the organization's repositories."""
    return _REPOS


def _get_pull_request_status(repo: str, number: int) -> dict:
    """Return the status of a pull request in a repo."""
    return {"repo": repo, "number": number, "state": "open",
            "checks": "passing", "reviewers": ["alice", "bob"]}


def _trigger_deploy(repo: str, environment: str) -> dict:
    """Trigger a deployment of a repo to an environment."""
    return {"repo": repo, "environment": environment, "status": "queued",
            "deploy_id": f"dep-{repo}-{environment}-001"}


@mcp.tool()
def list_repos() -> list[dict]:
    """List the organization's repositories."""
    return _list_repos()


@mcp.tool()
def get_pull_request_status(repo: str, number: int) -> dict:
    """Get the status of a pull request by repo name and PR number."""
    return _get_pull_request_status(repo, number)


@mcp.tool()
def trigger_deploy(repo: str, environment: str) -> dict:
    """Trigger a deployment of a repo to a named environment (write action)."""
    return _trigger_deploy(repo, environment)


if __name__ == "__main__":
    mcp.run(transport="streamable-http")
```

Create `mcp-servers/repo-server/requirements.txt`:

```
mcp==1.30.0
```

Create `mcp-servers/repo-server/requirements-dev.txt`:

```
-r requirements.txt
pytest==8.3.4
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd mcp-servers/repo-server && pip install -r requirements-dev.txt && python -m pytest -q`
Expected: PASS (3 tests).

- [ ] **Step 5: Smoke-test the running server (verifies the SDK transport, not just the tool logic)**

Run: `cd mcp-servers/repo-server && python -m repo_server.server &` then
`curl -s -i -X POST http://localhost:8000/mcp -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'`
Expected: a 200 (or MCP session/initialize response) listing the three tools. If the SDK's current `tools/list` handshake needs an `initialize` first, adjust the smoke invocation accordingly - the goal is to confirm the server speaks MCP over StreamableHTTP on `/mcp`. Kill the background server afterward.

- [ ] **Step 6: Write the Dockerfile**

Create `mcp-servers/repo-server/Dockerfile`:

```dockerfile
FROM python:3.12-slim
WORKDIR /app
COPY mcp-servers/repo-server/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY mcp-servers/repo-server/repo_server ./repo_server
EXPOSE 8000
CMD ["python", "-m", "repo_server.server"]
```

- [ ] **Step 7: Commit**

```bash
git add mcp-servers/repo-server
git commit -m "feat(repo-server): MCP server with list_repos, pr-status, trigger_deploy"
```

---

### Task A3: `agents/_shared` - ADK agent harness + STS helper

**Files:**
- Create: `agents/_shared/ops_agent/__init__.py`, `agents/_shared/ops_agent/sts.py`, `agents/_shared/ops_agent/harness.py`, `agents/_shared/requirements.txt`, `agents/_shared/requirements-dev.txt`, `agents/_shared/tests/test_sts.py`, `agents/_shared/tests/test_harness.py`

**Interfaces:**
- Produces:
  - `ops_agent.sts.exchange(sts_base, subject_token, actor_token, audience="agentgateway") -> str` - performs the verified STS token-exchange request shape and returns the exchanged access token.
  - `ops_agent.sts.read_sa_token(path="/var/run/secrets/tokens/sa-token") -> str` - reads the agent's own projected ServiceAccount token.
  - `ops_agent.sts.decode_claims(jwt) -> dict` - decodes a JWT payload (no signature check; for surfacing the `act` chain).
  - `ops_agent.harness.build_app(agent_factory) -> FastAPI` - a FastAPI app exposing `POST /run` and `GET /health`, where `agent_factory(token: str, mode: str) -> (LlmAgent, bearer: str, extra: dict)` builds the per-request agent; `bearer` (which may differ from `token` - e.g. an STS-exchanged token) is threaded into session state for the MCP `header_provider`, and `extra` is merged into the JSON response.
  - `ops_agent.harness.bearer_header_provider(ctx) -> dict` - reads `bearer_token` from ADK session state and returns the `Authorization` header dict, for use as an `McpToolset`'s `header_provider`.
- Consumes: `google-adk` 2.11.0, `mcp` SDK, FastAPI/uvicorn, `google-genai` (for `types.Content`).

- [ ] **Step 1: Write failing tests for the STS helper**

Create `agents/_shared/tests/test_sts.py`:

```python
import json, base64
from ops_agent import sts

def _jwt(payload: dict) -> str:
    def seg(d): return base64.urlsafe_b64encode(json.dumps(d).encode()).rstrip(b"=").decode()
    return f"{seg({'alg':'none'})}.{seg(payload)}.sig"

def test_decode_claims_reads_nested_act():
    tok = _jwt({"sub": "user", "act": {"sub": "deploy", "act": {"sub": "release-manager"}}})
    c = sts.decode_claims(tok)
    assert c["sub"] == "user"
    assert c["act"]["sub"] == "deploy"
    assert c["act"]["act"]["sub"] == "release-manager"

def test_exchange_posts_verified_shape(monkeypatch):
    captured = {}
    class FakeResp:
        status_code = 200
        def json(self): return {"access_token": "exchanged-token"}
        def raise_for_status(self): pass
    def fake_post(url, data=None, headers=None, timeout=None):
        captured["url"] = url; captured["data"] = data; captured["headers"] = headers
        return FakeResp()
    monkeypatch.setattr(sts.requests, "post", fake_post)
    out = sts.exchange("http://sts:7777", "subject-tok", "actor-tok")
    assert out == "exchanged-token"
    assert captured["url"] == "http://sts:7777/oauth2/token"
    assert captured["data"]["grant_type"] == "urn:ietf:params:oauth:grant-type:token-exchange"
    assert captured["data"]["subject_token"] == "subject-tok"
    assert captured["data"]["subject_token_type"] == "urn:ietf:params:oauth:token-type:jwt"
    assert captured["data"]["actor_token"] == "actor-tok"
    assert captured["data"]["actor_token_type"] == "urn:ietf:params:oauth:token-type:jwt"
    assert captured["data"]["audience"] == "agentgateway"
    assert captured["headers"]["Authorization"] == "Bearer subject-tok"
```

- [ ] **Step 2: Run to verify fail**

Run: `cd agents/_shared && python -m pytest -q`
Expected: FAIL - `ops_agent` not importable.

- [ ] **Step 3: Write the STS helper**

Create `agents/_shared/ops_agent/__init__.py` (empty) and `agents/_shared/ops_agent/sts.py`:

```python
"""Solo STS token-exchange helper. Request shape verified live against the
running STS: form POST to /oauth2/token with jwt token types, audience, and
the subject token ALSO repeated as the Authorization header. The STS accepts
both Keycloak-issued and its own self-issued tokens as subject_token, which
is what makes the two-hop re-exchange chain work."""
import base64
import json
import requests

GRANT = "urn:ietf:params:oauth:grant-type:token-exchange"
JWT_TYPE = "urn:ietf:params:oauth:token-type:jwt"
DEFAULT_SA_TOKEN_PATH = "/var/run/secrets/tokens/sa-token"


def exchange(sts_base: str, subject_token: str, actor_token: str,
             audience: str = "agentgateway", timeout: int = 15) -> str:
    """Exchange subject_token (+ actor_token) at the STS, returning the
    exchanged access token. sts_base is scheme+host[:port] with no path."""
    resp = requests.post(
        sts_base.rstrip("/") + "/oauth2/token",
        data={
            "grant_type": GRANT,
            "subject_token": subject_token,
            "subject_token_type": JWT_TYPE,
            "actor_token": actor_token,
            "actor_token_type": JWT_TYPE,
            "audience": audience,
        },
        headers={
            "Content-Type": "application/x-www-form-urlencoded",
            "Authorization": "Bearer " + subject_token,
        },
        timeout=timeout,
    )
    resp.raise_for_status()
    return resp.json()["access_token"]


def read_sa_token(path: str = DEFAULT_SA_TOKEN_PATH) -> str:
    """Read this agent's own projected Kubernetes ServiceAccount token."""
    with open(path) as f:
        return f.read().strip()


def decode_claims(token: str) -> dict:
    """Decode a JWT payload without verifying the signature - used only to
    surface the sub/act chain in responses, never for authorization."""
    payload = token.split(".")[1]
    payload += "=" * (-len(payload) % 4)
    return json.loads(base64.urlsafe_b64decode(payload))
```

Create `agents/_shared/requirements.txt`:

```
google-adk[extensions]==2.11.0
mcp==1.30.0
fastapi==0.115.6
uvicorn==0.34.0
requests==2.32.3
```

Create `agents/_shared/requirements-dev.txt`:

```
-r requirements.txt
pytest==8.3.4
httpx==0.28.1
```

- [ ] **Step 4: Run to verify the STS helper passes**

Run: `cd agents/_shared && pip install -r requirements-dev.txt && python -m pytest tests/test_sts.py -q`
Expected: PASS (2 tests).

- [ ] **Step 5: Write the FastAPI harness + a verify-the-ADK-API smoke test**

The harness wraps the verified ADK 2.11.0 pattern (see Global Constraints). Because ADK 2.11.0 is post-training-cutoff, the harness is validated by a smoke test that actually constructs a runner and drives one turn against a stub LLM/MCP - if any ADK signature differs from the documented API, this test fails loudly and is the one place to adjust, before any of the four agents build on it.

Create `agents/_shared/ops_agent/harness.py`:

```python
"""FastAPI harness serving an ADK agent as POST /run + GET /health, threading
a per-request bearer token into session state so an McpToolset header_provider
can attach it to each MCP call. ADK 2.11.0 API per the official docs.

agent_factory(token, mode) returns a 3-tuple:
  - the LlmAgent to run
  - bearer: the token to place in session state for header_provider to use.
    This is NOT always `token` verbatim - an agent performing its own STS
    exchange (e.g. Delegation mode, or the two-hop chain's re-exchange)
    computes the exchanged token BEFORE building the agent/session, and
    returns that exchanged value here, so the MCP call carries it.
  - extra: a dict merged into the JSON response (e.g. a decoded delegation
    chain for the chain-of-custody demo) - {} if unused.
"""
import uuid
from typing import Callable

from fastapi import FastAPI
from pydantic import BaseModel
from google.adk.runners import InMemoryRunner
from google.genai import types


class RunRequest(BaseModel):
    query: str
    token: str = ""
    mode: str = ""


async def bearer_header_provider(ctx) -> dict:
    """Reusable McpToolset header_provider: attaches the bearer token placed
    in session state by build_app (see agent_factory contract above) as the
    Authorization header on every MCP tool call for this session."""
    token = ctx.state.get("bearer_token", "")
    return {"Authorization": f"Bearer {token}"} if token else {}


def build_app(agent_factory: Callable) -> FastAPI:
    app = FastAPI()

    @app.get("/health")
    async def health():
        return {"status": "ok"}

    @app.post("/run")
    async def run(req: RunRequest):
        agent, bearer, extra = agent_factory(req.token, req.mode)
        runner = InMemoryRunner(agent=agent, app_name="ops-agent")
        user_id = "demo"
        session_id = str(uuid.uuid4())
        await runner.session_service.create_session(
            app_name="ops-agent", user_id=user_id, session_id=session_id,
            state={"bearer_token": bearer, "mode": req.mode},
        )
        message = types.Content(role="user", parts=[types.Part(text=req.query)])
        final = ""
        async for event in runner.run_async(
            user_id=user_id, session_id=session_id, new_message=message,
        ):
            if event.is_final_response() and event.content and event.content.parts:
                final = "".join(p.text for p in event.content.parts if getattr(p, "text", None))
        return {"answer": final, **extra}

    return app
```

Create `agents/_shared/tests/test_harness.py`:

```python
from fastapi.testclient import TestClient
from ops_agent.harness import build_app

def test_health():
    # A trivial agent factory - health must not require a real LLM.
    app = build_app(lambda token, mode: (None, token, {}))
    client = TestClient(app)
    assert client.get("/health").json() == {"status": "ok"}
```

> Note for the implementer: `test_harness.py` only covers `/health` here - the `/run` path exercises real ADK classes (`InMemoryRunner`, `types.Content`, `run_async`) and can't be driven by a unit test without a real LLM. It's validated live in Task A4's smoke test (the first agent built on this harness). If any ADK signature in `harness.py` differs from the documented API when A4 runs it live, fix it here in A3 and re-run - this harness is the single shared point for ADK-API corrections. Do not add a placeholder `/run` test that asserts nothing; the live smoke test is the real coverage for that path.

- [ ] **Step 6: Run harness test**

Run: `cd agents/_shared && python -m pytest tests/test_harness.py -q`
Expected: PASS (health test).

- [ ] **Step 7: Commit**

```bash
git add agents/_shared
git commit -m "feat(agents): shared STS helper and ADK FastAPI harness"
```

---

### Task A4: `agents/repo-agent` - single-hop, two modes

**Files:**
- Create: `agents/repo-agent/repo_agent/__init__.py`, `agents/repo-agent/repo_agent/main.py`, `agents/repo-agent/requirements.txt`, `agents/repo-agent/Dockerfile`, `agents/repo-agent/tests/test_main.py`

**Interfaces:**
- Produces: the `repo-agent` container. `POST /run {"query", "token", "mode": "standard"|"sts-delegation"}`.
  - `mode=standard`: the agent presents the user's own token as-is to the MCP tool call; agentgateway performs the Impersonation exchange itself (no STS call from the agent).
  - `mode=sts-delegation`: the agent exchanges the user's token at the STS (subject=user token, actor=this agent's own SA token) BEFORE calling the MCP tool, and presents the delegated token.
- Consumes: `ops_agent.sts.exchange`, `ops_agent.sts.read_sa_token`, `ops_agent.harness.build_app`, `ops_agent.harness.bearer_header_provider`.
- Env: `LLM_BASE_URL`, `MCP_STANDARD_URL`, `MCP_DELEGATION_URL`, `STS_URL`, `SA_TOKEN_PATH` (default from `ops_agent.sts.DEFAULT_SA_TOKEN_PATH`).

- [ ] **Step 1: Write failing tests**

Create `agents/repo-agent/tests/test_main.py` (tests the mode-selection/exchange logic directly, not the full ADK run - that's the live smoke test in Step 5):

```python
from unittest.mock import patch
from repo_agent.main import build_agent, _mcp_url_for_mode

def test_mcp_url_selects_route_by_mode(monkeypatch):
    monkeypatch.setenv("MCP_STANDARD_URL", "http://gw/repo-standard/mcp")
    monkeypatch.setenv("MCP_DELEGATION_URL", "http://gw/repo-delegation/mcp")
    assert _mcp_url_for_mode("standard") == "http://gw/repo-standard/mcp"
    assert _mcp_url_for_mode("sts-delegation") == "http://gw/repo-delegation/mcp"

def test_standard_mode_bearer_is_the_raw_user_token(monkeypatch):
    monkeypatch.setenv("MCP_STANDARD_URL", "http://gw/repo-standard/mcp")
    monkeypatch.setenv("MCP_DELEGATION_URL", "http://gw/repo-delegation/mcp")
    monkeypatch.setenv("LLM_BASE_URL", "http://gw/openai")
    agent, bearer, extra = build_agent("user-token-123", "standard")
    assert bearer == "user-token-123"
    assert extra == {}

def test_sts_delegation_mode_exchanges_before_building_agent(monkeypatch, tmp_path):
    monkeypatch.setenv("MCP_STANDARD_URL", "http://gw/repo-standard/mcp")
    monkeypatch.setenv("MCP_DELEGATION_URL", "http://gw/repo-delegation/mcp")
    monkeypatch.setenv("LLM_BASE_URL", "http://gw/openai")
    monkeypatch.setenv("STS_URL", "http://sts:7777")
    sa_path = tmp_path / "sa-token"
    sa_path.write_text("agent-sa-token")
    monkeypatch.setenv("SA_TOKEN_PATH", str(sa_path))
    with patch("repo_agent.main.sts.exchange", return_value="delegated-token") as m:
        agent, bearer, extra = build_agent("user-token-123", "sts-delegation")
    m.assert_called_once_with("http://sts:7777", "user-token-123", "agent-sa-token")
    assert bearer == "delegated-token"
```

- [ ] **Step 2: Run to verify fail**

Run: `cd agents/repo-agent && python -m pytest -q`
Expected: FAIL - `repo_agent` not importable.

- [ ] **Step 3: Write the agent**

Create `agents/repo-agent/repo_agent/__init__.py` (empty) and `agents/repo-agent/repo_agent/main.py`:

```python
"""Repo Agent - single-hop agent demonstrating Standard (Impersonation,
proxy-native) vs STS Delegation exchange against the internal repo MCP
server. Mode switches which gateway route / exchange path is used - see
Flow 1 in docs/superpowers/specs/2026-10-10-auth-pillar-agents-mcp-design.md."""
import os

from google.adk.agents import LlmAgent
from google.adk.models.lite_llm import LiteLlm
from google.adk.tools.mcp_tool import McpToolset, StreamableHTTPConnectionParams

from ops_agent import sts
from ops_agent.harness import bearer_header_provider, build_app

INSTRUCTION = (
    "You are the Repo Agent. You can list repositories and check pull "
    "request status using your tools. Answer concisely based on tool results."
)


def _mcp_url_for_mode(mode: str) -> str:
    if mode == "sts-delegation":
        return os.environ["MCP_DELEGATION_URL"]
    return os.environ["MCP_STANDARD_URL"]


def build_agent(token: str, mode: str):
    """Returns (LlmAgent, bearer_for_mcp, extra) per the harness contract."""
    if mode == "sts-delegation":
        sa_token = sts.read_sa_token(os.environ.get("SA_TOKEN_PATH", sts.DEFAULT_SA_TOKEN_PATH))
        bearer = sts.exchange(os.environ["STS_URL"], token, sa_token)
    else:
        bearer = token

    toolset = McpToolset(
        connection_params=StreamableHTTPConnectionParams(url=_mcp_url_for_mode(mode)),
        header_provider=bearer_header_provider,
    )
    agent = LlmAgent(
        model=LiteLlm(model="openai/gpt-4o-mini", api_base=os.environ["LLM_BASE_URL"]),
        name="repo_agent",
        instruction=INSTRUCTION,
        tools=[toolset],
    )
    return agent, bearer, {}


app = build_app(build_agent)
```

Create `agents/repo-agent/requirements.txt`:

```
-r ../_shared/requirements.txt
```

- [ ] **Step 4: Run to verify pass**

Run: `cd agents/repo-agent && pip install -r requirements.txt -r ../_shared/requirements-dev.txt && PYTHONPATH=../_shared python -m pytest -q`
Expected: PASS (3 tests).

- [ ] **Step 5: Live smoke test (validates the full ADK + MCP + FastAPI stack end to end for the first time)**

This is the task that proves out `harness.py`'s ADK usage for real, per A3's note. Run against the real `repo-server` (Task A2) started locally and a stub OpenAI-compatible endpoint, or - more realistically - defer this exact smoke test to Phase C's live verification once both are deployed behind agentgateway (Task C5), since `LLM_BASE_URL`/`MCP_*_URL` need a real OpenAI-compatible chat endpoint and a real MCP server to drive a genuine tool-call turn. Record here what to check when that live pass happens:
  - `POST /run {"query": "list our repos", "token": "<any>", "mode": "standard"}` returns `{"answer": "..."}` mentioning repo names from `repo-server`'s `list_repos`.
  - If `runner.run_async`/`types.Content`/`event.is_final_response()` differ from the documented 2.11.0 API, the error surfaces here first - fix `harness.py` (Task A3) and re-run both A3's and A4's tests.

- [ ] **Step 6: Write the Dockerfile**

Create `agents/repo-agent/Dockerfile`:

```dockerfile
FROM python:3.12-slim
WORKDIR /app
COPY agents/_shared /app/_shared
RUN pip install --no-cache-dir -e /app/_shared -r /app/_shared/requirements.txt
COPY agents/repo-agent/repo_agent ./repo_agent
EXPOSE 8080
CMD ["uvicorn", "repo_agent.main:app", "--host", "0.0.0.0", "--port", "8080"]
```

> Notes: (1) `_shared` is installed as an editable local package so `repo_agent.main`'s `from ops_agent import ...` resolves in the container - needs a minimal `agents/_shared/pyproject.toml` `[build-system]` section for `pip install -e` to work (confirm during Step 4's local install, since that already exercises the same import path). (2) This agent's own `requirements.txt` (`-r ../_shared/requirements.txt`) is intentionally NOT copied into the image - that relative path means something different inside the container (where `_shared` is a subdirectory of `/app`, not a sibling of the agent's own directory) than it does for local dev, so the Dockerfile installs `_shared`'s requirements.txt directly by its known in-container path instead of relying on that file. The local `requirements.txt` file still exists and is still correct for local dev (`cd agents/repo-agent && pip install -r requirements.txt`) - it's just not part of the image build. Build context is the monorepo root, not this directory - see Task A1's Makefile note.

- [ ] **Step 7: Commit**

```bash
git add agents/repo-agent
git commit -m "feat(repo-agent): single-hop agent, standard vs sts-delegation modes"
```

---

### Task A5: `agents/release-manager-agent` - hop 1 of the chain

**Files:**
- Create: `agents/release-manager-agent/release_manager_agent/__init__.py`, `agents/release-manager-agent/release_manager_agent/main.py`, `agents/release-manager-agent/requirements.txt`, `agents/release-manager-agent/Dockerfile`, `agents/release-manager-agent/tests/test_main.py`

**Interfaces:**
- Produces: the `release-manager-agent` container. `POST /run {"query", "token"}` (no `mode` - this agent only ever does Delegation). Exchanges the user's token at the STS, then calls Deploy Agent's `/run` as a tool, passing the exchanged token.
- Consumes: `ops_agent.sts.exchange`, `ops_agent.sts.read_sa_token`, `ops_agent.harness.build_app`.
- Env: `LLM_BASE_URL`, `STS_URL`, `SA_TOKEN_PATH`, `DEPLOY_AGENT_URL`.

This agent does NOT call the MCP server itself - its one tool is "ask the Deploy Agent," implemented as a plain Python function tool (an ADK `FunctionTool`, inferred automatically from a plain function passed in `tools=[...]`), not an `McpToolset`.

- [ ] **Step 1: Write failing tests**

Create `agents/release-manager-agent/tests/test_main.py`:

```python
from unittest.mock import patch
from release_manager_agent.main import build_agent, ask_deploy_agent

def test_build_agent_exchanges_user_token_for_delegated_token(monkeypatch, tmp_path):
    monkeypatch.setenv("LLM_BASE_URL", "http://gw/openai")
    monkeypatch.setenv("STS_URL", "http://sts:7777")
    monkeypatch.setenv("DEPLOY_AGENT_URL", "http://gw/deploy-agent/run")
    sa_path = tmp_path / "sa-token"
    sa_path.write_text("release-manager-sa-token")
    monkeypatch.setenv("SA_TOKEN_PATH", str(sa_path))
    with patch("release_manager_agent.main.sts.exchange", return_value="T1") as m:
        agent, bearer, extra = build_agent("user-token-123", "")
    m.assert_called_once_with("http://sts:7777", "user-token-123", "release-manager-sa-token")
    assert bearer == "T1"
    assert extra == {}

def test_ask_deploy_agent_forwards_the_exchanged_token(monkeypatch):
    monkeypatch.setenv("DEPLOY_AGENT_URL", "http://gw/deploy-agent/run")
    captured = {}
    class FakeResp:
        def json(self): return {"answer": "deployed", "chain": "user -> release-manager -> deploy-agent"}
        def raise_for_status(self): pass
    def fake_post(url, json=None, timeout=None):
        captured["url"] = url; captured["json"] = json
        return FakeResp()
    with patch("release_manager_agent.main.requests.post", fake_post):
        result = ask_deploy_agent("deploy payments-service to staging", "T1")
    assert captured["url"] == "http://gw/deploy-agent/run"
    assert captured["json"] == {"query": "deploy payments-service to staging", "token": "T1"}
    assert "chain" in result
```

- [ ] **Step 2: Run to verify fail**

Run: `cd agents/release-manager-agent && python -m pytest -q`
Expected: FAIL - module not importable.

- [ ] **Step 3: Write the agent**

Create `agents/release-manager-agent/release_manager_agent/__init__.py` (empty) and `agents/release-manager-agent/release_manager_agent/main.py`:

```python
"""Release Manager Agent - front-line of the two-hop Delegation chain
(Flow 2). Exchanges the user's token at the STS, then delegates to Deploy
Agent, passing the exchanged (not raw) token - Deploy Agent re-exchanges it
in turn, producing a nested act chain (chain-of-custody)."""
import os

import requests
from google.adk.agents import LlmAgent
from google.adk.models.lite_llm import LiteLlm

from ops_agent import sts
from ops_agent.harness import build_app

INSTRUCTION = (
    "You are the Release Manager Agent. When asked about a deploy, use the "
    "ask_deploy_agent tool to delegate to the Deploy Agent and relay its answer, "
    "including the delegation chain it reports."
)


def ask_deploy_agent(query: str, token: str) -> dict:
    """Delegate a deploy-related request to the Deploy Agent, passing along
    the caller's exchanged authorization token."""
    resp = requests.post(
        os.environ["DEPLOY_AGENT_URL"], json={"query": query, "token": token}, timeout=30,
    )
    resp.raise_for_status()
    return resp.json()


def build_agent(token: str, mode: str):
    sa_token = sts.read_sa_token(os.environ.get("SA_TOKEN_PATH", sts.DEFAULT_SA_TOKEN_PATH))
    delegated_token = sts.exchange(os.environ["STS_URL"], token, sa_token)

    def ask_deploy_agent_tool(query: str) -> dict:
        """Ask the Deploy Agent to check or perform a deploy-related action."""
        return ask_deploy_agent(query, delegated_token)

    agent = LlmAgent(
        model=LiteLlm(model="openai/gpt-4o-mini", api_base=os.environ["LLM_BASE_URL"]),
        name="release_manager_agent",
        instruction=INSTRUCTION,
        tools=[ask_deploy_agent_tool],
    )
    return agent, delegated_token, {}


app = build_app(build_agent)
```

Create `agents/release-manager-agent/requirements.txt`:

```
-r ../_shared/requirements.txt
```

- [ ] **Step 4: Run to verify pass**

Run: `cd agents/release-manager-agent && PYTHONPATH=../_shared python -m pytest -q`
Expected: PASS (2 tests).

- [ ] **Step 5: Write the Dockerfile**

Create `agents/release-manager-agent/Dockerfile` (same pattern as Task A4's `agents/repo-agent/Dockerfile` - build context is the monorepo root, `_shared`'s requirements installed directly by their in-container path, not via this agent's own `requirements.txt`):

```dockerfile
FROM python:3.12-slim
WORKDIR /app
COPY agents/_shared /app/_shared
RUN pip install --no-cache-dir -e /app/_shared -r /app/_shared/requirements.txt
COPY agents/release-manager-agent/release_manager_agent ./release_manager_agent
EXPOSE 8080
CMD ["uvicorn", "release_manager_agent.main:app", "--host", "0.0.0.0", "--port", "8080"]
```

- [ ] **Step 6: Commit**

```bash
git add agents/release-manager-agent
git commit -m "feat(release-manager-agent): hop 1 of the two-hop delegation chain"
```

---

### Task A6: `agents/deploy-agent` - hop 2, re-exchange + chain surfacing

**Files:**
- Create: `agents/deploy-agent/deploy_agent/__init__.py`, `agents/deploy-agent/deploy_agent/main.py`, `agents/deploy-agent/requirements.txt`, `agents/deploy-agent/Dockerfile`, `agents/deploy-agent/tests/test_main.py`

**Interfaces:**
- Produces: the `deploy-agent` container. `POST /run {"query", "token"}` where `token` is Release Manager's already-exchanged token (T1). Re-exchanges T1 at the STS using its own SA token as actor, producing T2 (nested `act`), calls the MCP tool with T2, and returns the decoded chain alongside the answer (`extra = {"chain": {...}}`) for the demo to display.
- Consumes: `ops_agent.sts.exchange`, `ops_agent.sts.read_sa_token`, `ops_agent.sts.decode_claims`, `ops_agent.harness.build_app`, `ops_agent.harness.bearer_header_provider`.
- Env: `LLM_BASE_URL`, `MCP_URL`, `STS_URL`, `SA_TOKEN_PATH`.

- [ ] **Step 1: Write failing tests**

Create `agents/deploy-agent/tests/test_main.py`:

```python
import json, base64
from unittest.mock import patch
from deploy_agent.main import build_agent

def _jwt(payload: dict) -> str:
    def seg(d): return base64.urlsafe_b64encode(json.dumps(d).encode()).rstrip(b"=").decode()
    return f"{seg({'alg':'none'})}.{seg(payload)}.sig"

def test_build_agent_reexchanges_and_surfaces_the_chain(monkeypatch, tmp_path):
    monkeypatch.setenv("LLM_BASE_URL", "http://gw/openai")
    monkeypatch.setenv("MCP_URL", "http://gw/repo-chain/mcp")
    monkeypatch.setenv("STS_URL", "http://sts:7777")
    sa_path = tmp_path / "sa-token"
    sa_path.write_text("deploy-agent-sa-token")
    monkeypatch.setenv("SA_TOKEN_PATH", str(sa_path))

    t1 = "release-manager-token"
    t2 = _jwt({"sub": "user-123", "act": {"sub": "deploy-agent", "act": {"sub": "release-manager"}}})

    with patch("deploy_agent.main.sts.exchange", return_value=t2) as m:
        agent, bearer, extra = build_agent(t1, "")

    m.assert_called_once_with("http://sts:7777", t1, "deploy-agent-sa-token")
    assert bearer == t2
    assert extra["chain"]["sub"] == "user-123"
    assert extra["chain"]["act"] == "deploy-agent"
    assert extra["chain"]["act_act"] == "release-manager"
```

- [ ] **Step 2: Run to verify fail**

Run: `cd agents/deploy-agent && python -m pytest -q`
Expected: FAIL - module not importable.

- [ ] **Step 3: Write the agent**

Create `agents/deploy-agent/deploy_agent/__init__.py` (empty) and `agents/deploy-agent/deploy_agent/main.py`:

```python
"""Deploy Agent - specialist, hop 2 of the two-hop Delegation chain (Flow 2).
Re-exchanges the token it receives (already an STS-issued delegated token
from Release Manager) at the STS, producing a token with a nested act chain
- confirmed live that Solo's STS accepts its own tokens as exchange
subjects (see the design spec's spike finding). Surfaces the decoded chain
in its response for the chain-of-custody demo."""
import os

from google.adk.agents import LlmAgent
from google.adk.models.lite_llm import LiteLlm
from google.adk.tools.mcp_tool import McpToolset, StreamableHTTPConnectionParams

from ops_agent import sts
from ops_agent.harness import bearer_header_provider, build_app

INSTRUCTION = (
    "You are the Deploy Agent. Use your tools to check pull request status or "
    "trigger deploys. Answer concisely based on tool results."
)


def build_agent(token: str, mode: str):
    """token here is Release Manager's already-exchanged (STS-issued) token,
    not a raw Keycloak token - this is the re-exchange hop."""
    sa_token = sts.read_sa_token(os.environ.get("SA_TOKEN_PATH", sts.DEFAULT_SA_TOKEN_PATH))
    t2 = sts.exchange(os.environ["STS_URL"], token, sa_token)
    claims = sts.decode_claims(t2)
    act = claims.get("act", {})
    chain = {
        "sub": claims.get("sub"),
        "act": act.get("sub"),
        "act_act": act.get("act", {}).get("sub"),
    }

    toolset = McpToolset(
        connection_params=StreamableHTTPConnectionParams(url=os.environ["MCP_URL"]),
        header_provider=bearer_header_provider,
    )
    agent = LlmAgent(
        model=LiteLlm(model="openai/gpt-4o-mini", api_base=os.environ["LLM_BASE_URL"]),
        name="deploy_agent",
        instruction=INSTRUCTION,
        tools=[toolset],
    )
    return agent, t2, {"chain": chain}


app = build_app(build_agent)
```

Create `agents/deploy-agent/requirements.txt`:

```
-r ../_shared/requirements.txt
```

- [ ] **Step 4: Run to verify pass**

Run: `cd agents/deploy-agent && PYTHONPATH=../_shared python -m pytest -q`
Expected: PASS (1 test).

- [ ] **Step 5: Write the Dockerfile**

Create `agents/deploy-agent/Dockerfile` (same pattern as Task A4's):

```dockerfile
FROM python:3.12-slim
WORKDIR /app
COPY agents/_shared /app/_shared
RUN pip install --no-cache-dir -e /app/_shared -r /app/_shared/requirements.txt
COPY agents/deploy-agent/deploy_agent ./deploy_agent
EXPOSE 8080
CMD ["uvicorn", "deploy_agent.main:app", "--host", "0.0.0.0", "--port", "8080"]
```

- [ ] **Step 6: Commit**

```bash
git add agents/deploy-agent
git commit -m "feat(deploy-agent): hop 2 re-exchange, surfaces the nested act chain"
```

---

### Task A7: `agents/github-agent` - Elicitation

**Files:**
- Create: `agents/github-agent/github_agent/__init__.py`, `agents/github-agent/github_agent/main.py`, `agents/github-agent/requirements.txt`, `agents/github-agent/Dockerfile`, `agents/github-agent/tests/test_main.py`

**Interfaces:**
- Produces: the `github-agent` container. `POST /run {"query", "token"}` where `token` is the user's raw Keycloak token. No STS call from this agent - the gateway's `entElicitation` backend auth does the work. If the gateway returns a pending/elicitation response, the agent surfaces the elicitation URL directly rather than treating it as a tool failure.
- Consumes: `ops_agent.harness.build_app`, `ops_agent.harness.bearer_header_provider`.
- Env: `LLM_BASE_URL`, `MCP_GITHUB_URL`.

Because the first call to the GitHub MCP route can return a "pending" response (no stored GitHub token yet - see Flow 3) rather than a tool result, this agent's MCP tool call needs to detect that case and return the elicitation URL as the tool's own result (so the LLM relays it to the user) instead of raising. The exact shape of agentgateway's "pending" response (status code, body field names for the elicitation URL) is **not fully specified in this plan** - Solo's docs describe the behavior (see the design spec's Flow 3) but not the literal wire format an MCP tool call sees it in. This task's Step 5 verifies that against the real deployment once Phase B's Elicitation policy is up, and the code here is written to be adjusted at that point.

- [ ] **Step 1: Write failing tests**

Create `agents/github-agent/tests/test_main.py`:

```python
from github_agent.main import build_agent

def test_build_agent_uses_raw_user_token_no_exchange(monkeypatch):
    monkeypatch.setenv("LLM_BASE_URL", "http://gw/openai")
    monkeypatch.setenv("MCP_GITHUB_URL", "http://gw/mcp-github")
    agent, bearer, extra = build_agent("user-keycloak-token", "")
    assert bearer == "user-keycloak-token"
    assert extra == {}
```

- [ ] **Step 2: Run to verify fail**

Run: `cd agents/github-agent && python -m pytest -q`
Expected: FAIL - module not importable.

- [ ] **Step 3: Write the agent**

Create `agents/github-agent/github_agent/__init__.py` (empty) and `agents/github-agent/github_agent/main.py`:

```python
"""GitHub Agent - Elicitation demo (Flow 3). No STS call from this agent at
all: the user's own Keycloak token is presented to the GitHub MCP route, and
agentgateway's entElicitation backend auth either injects a stored real
GitHub token or returns a pending response with a consent URL - see
docs/superpowers/specs/2026-10-10-auth-pillar-agents-mcp-design.md Flow 3.

TODO(Phase B live verification): confirm the exact wire shape of the
"pending" response an MCP tool call receives on the first attempt (status
code / body field for the elicitation URL) against the real deployment, and
adjust the instruction/tool wrapper below to surface it faithfully - this
can only be confirmed once Task B5's entElicitation policy is live."""
import os

from google.adk.agents import LlmAgent
from google.adk.models.lite_llm import LiteLlm
from google.adk.tools.mcp_tool import McpToolset, StreamableHTTPConnectionParams

from ops_agent.harness import bearer_header_provider, build_app

INSTRUCTION = (
    "You are the GitHub Agent. Use your tools to answer questions about the "
    "user's real GitHub repositories. If a tool result indicates that "
    "authorization is required (a pending status with a URL), tell the user "
    "exactly that URL and ask them to open it to connect their GitHub "
    "account, then retry."
)


def build_agent(token: str, mode: str):
    toolset = McpToolset(
        connection_params=StreamableHTTPConnectionParams(url=os.environ["MCP_GITHUB_URL"]),
        header_provider=bearer_header_provider,
    )
    agent = LlmAgent(
        model=LiteLlm(model="openai/gpt-4o-mini", api_base=os.environ["LLM_BASE_URL"]),
        name="github_agent",
        instruction=INSTRUCTION,
        tools=[toolset],
    )
    return agent, token, {}


app = build_app(build_agent)
```

- [ ] **Step 4: Run to verify pass**

Run: `cd agents/github-agent && PYTHONPATH=../_shared python -m pytest -q`
Expected: PASS (1 test).

- [ ] **Step 5: Live verification (deferred to Phase C5, requires Task B5)**

Once the real `entElicitation` policy is deployed (Task B5), drive this agent live for both states: (a) no stored GitHub token - confirm the agent's final answer surfaces a real, clickable elicitation URL; (b) after completing consent in the Solo UI - confirm a retry returns real GitHub data (e.g. repo names) via `list_my_repos`-equivalent tool(s) the real GitHub MCP server exposes. Adjust `INSTRUCTION`/the pending-detection logic in `main.py` based on what's actually observed, per the TODO above.

- [ ] **Step 6: Write the Dockerfile**

Create `agents/github-agent/Dockerfile` (same pattern as Task A4's):

```dockerfile
FROM python:3.12-slim
WORKDIR /app
COPY agents/_shared /app/_shared
RUN pip install --no-cache-dir -e /app/_shared -r /app/_shared/requirements.txt
COPY agents/github-agent/github_agent ./github_agent
EXPOSE 8080
CMD ["uvicorn", "github_agent.main:app", "--host", "0.0.0.0", "--port", "8080"]
```

- [ ] **Step 7: Commit**

```bash
git add agents/github-agent
git commit -m "feat(github-agent): elicitation agent for the real GitHub MCP"
```

---

### Task A8: CI/CD pipeline - build and push all five images on every change, cut tagged releases

Rather than a one-off manual build, this repo gets the same tag-based release pipeline as `agentgateway-demo-wizard`: `ci.yml` builds and pushes every image on every push to `main` (tagged `sha-<short>`) or version tag (tagged with the version), and a manually-triggered `release.yml` computes the next semver tag, pushes it, creates a GitHub Release, and builds+pushes all five images against it. Both use GCP Workload Identity Federation - requires `WIF_PROVIDER`/`WIF_SERVICE_ACCOUNT` repo secrets (same values as `agentgateway-demo-wizard`'s, since they authenticate to the same GCP project/registry).

**Files:**
- Modify: `.github/workflows/ci.yml` (add `build-push` matrix job + `create-release` job)
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Extend `ci.yml`**

Add `tags: ["v*"]` to the `push` trigger. Add a `build-push` job (`needs: [lint-test]`, skipped on `pull_request`) using a matrix over the five services:

```yaml
  build-push:
    name: Build and push (${{ matrix.image }})
    needs: [lint-test]
    if: github.event_name != 'pull_request'
    runs-on: ubuntu-latest
    strategy:
      matrix:
        include:
          - dir: mcp-servers/repo-server
            image: repo-server
          - dir: agents/repo-agent
            image: repo-agent
          - dir: agents/release-manager-agent
            image: release-manager-agent
          - dir: agents/deploy-agent
            image: deploy-agent
          - dir: agents/github-agent
            image: github-agent
    permissions:
      contents: read
      id-token: write # required for GCP Workload Identity Federation
    steps:
      - uses: actions/checkout@v4
      - id: auth
        uses: google-github-actions/auth@v2
        with:
          workload_identity_provider: ${{ secrets.WIF_PROVIDER }}
          service_account: ${{ secrets.WIF_SERVICE_ACCOUNT }}
      - uses: google-github-actions/setup-gcloud@v2
      - run: gcloud auth configure-docker ${{ env.REGISTRY }} --quiet
      - name: Compute tag
        id: tag
        run: |
          if [[ "${GITHUB_REF}" == refs/tags/v* ]]; then
            echo "tag=${GITHUB_REF_NAME}" >> "$GITHUB_OUTPUT"
          else
            echo "tag=sha-${GITHUB_SHA::12}" >> "$GITHUB_OUTPUT"
          fi
      - name: Build and push
        run: |
          set -euo pipefail
          IMAGE="${REGISTRY}/${REPO_PATH}/${{ matrix.image }}:${{ steps.tag.outputs.tag }}"
          docker build -f "${{ matrix.dir }}/Dockerfile" -t "$IMAGE" .
          docker push "$IMAGE"
```

with `env: { REGISTRY: australia-southeast1-docker.pkg.dev, REPO_PATH: field-engineering-apac/kasunt }` at the workflow level. Add a `create-release` job (`needs: [build-push]`, only on `v*` tag push, `permissions: { contents: write }`) that lists all five image references in the GitHub Release notes - see the sibling `create-release` job in `agentgateway-demo-wizard/.github/workflows/ci.yml` for the exact append-vs-create pattern (a release may already exist from `release.yml`, so append image notes rather than clobbering).

- [ ] **Step 2: Write `release.yml`**

Mirror `agentgateway-demo-wizard/.github/workflows/release.yml` exactly: `workflow_dispatch` with a `bump_type` (patch/minor/major) choice input, a `validate` job (ruff + pytest), a `tag-and-release` job that computes the next semver tag from the latest `git describe --tags`, pushes it, and creates the GitHub Release. Unlike the wizard's single-image repo, this one's `build-push` job (`needs: [tag-and-release]`) uses the same five-way matrix as `ci.yml`'s, and a final `create-release` job (`needs: [tag-and-release, build-push]`) appends all five image locations to the release notes.

**Important:** a tag pushed with the default `GITHUB_TOKEN` (as `tag-and-release`'s "Push tag" step does) does NOT trigger other workflow runs - this is why `release.yml` needs its own `build-push`/`create-release` jobs rather than relying on `ci.yml`'s `tags: ["v*"]` trigger to fire. `ci.yml`'s tag trigger still matters for the case where someone pushes a `v*` tag manually (not through `release.yml`), using their own credentials.

- [ ] **Step 3: Verify live**

Push to `main` and confirm `ci.yml` runs lint-test then builds+pushes all five images tagged `sha-<short>`. Then run `gh workflow run release.yml -f bump_type=minor` (minor, not patch, so the first tag lands on `v0.1.0` from a `v0.0.0` baseline, matching the `v0.1.0` image references Task B6 hardcodes) and confirm: the tag is pushed, a GitHub Release is created, all five images are pushed tagged `v0.1.0`, and the release notes list all five image locations. Verify the images actually exist in the registry (e.g. `gcloud artifacts docker images list ... --include-tags`).

- [ ] **Step 4: Record the image tags**

The five image references for Phase B (Task B3/B4/B5/B6) are `australia-southeast1-docker.pkg.dev/field-engineering-apac/kasunt/{repo-server,repo-agent,release-manager-agent,deploy-agent,github-agent}:v0.1.0`.

---

## Phase B: field-kit infrastructure (agentgateway-field-kit repo)

All paths in this phase are relative to the `agentgateway-field-kit` repo root, a sibling checkout - not this wizard repo and not `ops-agents`. Every usecase YAML file referenced here is new except where a task explicitly says "modify".

### Task B1: Fix missing `preserveToken` in the `oauth-token-exchange` feature

This bug was found and fixed as a one-off in this wizard's own hand-written manifest template earlier (see `internal/manifests/templates/auth/token-exchange-standard.yaml.tmpl`), but the shared field-kit **feature class** other usecases reuse never got the same fix. Task B3 below depends on this feature for the Repo MCP server's Standard-exchange route, so fix it first - without it, every request through that route 400s with "invalid request" (the gateway strips the JWT after validating it, so the exchange call finds no subject_token).

**Files:**
- Modify: `features/oauth-token-exchange/index.js`
- Modify: `test/features/oauth-token-exchange.test.js`

**Interfaces:** none - this is a field-kit-internal fix, no new config surface.

- [ ] **Step 1: Add a failing assertion**

In `test/features/oauth-token-exchange.test.js`, add this assertion to the existing `'combines traffic.jwtAuthentication and backend.auth.oauthTokenExchange in one policy...'` test, right after the `expect(policy.spec.targetRefs).toEqual(targetRefs);` line:

```js
    // Required alongside backend.auth.oauthTokenExchange: by default the gateway strips
    // the JWT after validating it, so the exchange finds no subject_token and every
    // request fails with 400 "invalid request" (live-verified against this wizard's own
    // auth pillar).
    expect(policy.spec.traffic.jwtAuthentication.preserveToken).toBe(true);
```

- [ ] **Step 2: Run to verify it fails**

Run: `bun test test/features/oauth-token-exchange.test.js`
Expected: FAIL - `expect(received).toBe(expected)` with `received: undefined`.

- [ ] **Step 3: Fix the feature**

In `features/oauth-token-exchange/index.js`'s `deployPolicy()`, change:

```js
        traffic: {
          jwtAuthentication: {
            mode: this.jwtAuth.mode || 'Strict',
            providers: [
```

to:

```js
        traffic: {
          jwtAuthentication: {
            mode: this.jwtAuth.mode || 'Strict',
            // Required alongside backend.auth.oauthTokenExchange below: by default the
            // gateway strips the JWT after validating it, so the exchange finds no
            // subject_token and every request fails with 400 "invalid request".
            preserveToken: true,
            providers: [
```

- [ ] **Step 4: Run to verify it passes**

Run: `bun test test/features/oauth-token-exchange.test.js`
Expected: PASS (all tests, including the new assertion).

- [ ] **Step 5: Commit**

```bash
git add features/oauth-token-exchange/index.js test/features/oauth-token-exchange.test.js
git commit -m "fix(oauth-token-exchange): always preserve the JWT for the exchange call"
```

---

### Task B2: Enable the STS (Postgres + token-exchange) for the ops-agents usecase

**Files:**
- Create: `config/usecases/enterprise/security/ops-agents.yaml`

**Interfaces:**
- Produces: a running STS at `enterprise-agentgateway.agentgateway-system.svc.cluster.local:7777`, backed by its own Postgres (`ops-agents-postgres`), with `skipMayActClaimValidation: true` (matching the live-verified cluster config this plan's spike ran against) so Task B4's agents don't need Keycloak admin API calls to add `may_act` claim mappers.
- Consumes: the `postgres` and `token-exchange` features (field-kit), unmodified.

- [ ] **Step 1: Create the usecase file**

Create `config/usecases/enterprise/security/ops-agents.yaml`:

```yaml
apiVersion: agentgateway.demo/v1
kind: UseCase
metadata:
  name: ops-agents
  description: |
    Real ADK agents and MCP servers demonstrating every agent-facing auth pattern
    agentgateway supports: Standard/Impersonation exchange, STS/Delegation exchange,
    a two-hop re-exchange chain (chain-of-custody via a nested act claim), and
    Elicitation against a real external API (GitHub).
  docs: https://docs.solo.io/agentgateway/latest/mcp/token-exchange/

spec:
  diagram: |
    sequenceDiagram
      participant You
      participant RM as Release Manager Agent
      participant STS
      participant DA as Deploy Agent
      participant AGW as agentgateway
      participant MCP as Repo MCP server
      You->>RM: "deploy payments-service to staging"
      RM->>STS: exchange (subject=your token, actor=RM's SA token)
      STS-->>RM: T1 (sub=you, act=release-manager-agent)
      RM->>DA: forward T1
      DA->>STS: exchange (subject=T1, actor=DA's SA token)
      STS-->>DA: T2 (sub=you, act=deploy-agent, act.act=release-manager-agent)
      DA->>AGW: MCP call (Bearer T2)
      AGW->>MCP: forward
      MCP-->>DA: tool result
      DA-->>RM: answer + decoded chain
      RM-->>You: answer + chain

  steps:
    - title: Deploy Postgres for the ops-agents token exchange service
      description: |
        A dedicated Postgres instance for this usecase's own STS database - required
        because token-exchange's Helm upgrades never clear a previous usecase's
        database.postgres.url, only ever set it.
      features:
        - name: postgres
          config:
            name: ops-agents-postgres

    - title: Enable the Security Token Service (STS)
      description: |
        Enables the agentgateway control plane's STS at port 7777. skipMayActClaimValidation
        is set because this usecase's agents authenticate as their own Kubernetes
        ServiceAccount (validated via the k8s actorValidator) rather than a Keycloak
        client with a registered may_act claim.
      features:
        - name: token-exchange
          config:
            keycloak:
              realm: agw-dev
              serviceName: keycloak
              serviceNamespace: keycloak
            tokenExpiration: 1h
            skipMayActClaimValidation: true
            callbackUrl: 'https://{{env.domains.core.soloUi}}/age/elicitations'
            database:
              postgres:
                url: postgres://tokenexchange:tokenexchange@ops-agents-postgres.agentgateway-system.svc.cluster.local:5432/tokenexchange?sslmode=disable

  tests: []
```

- [ ] **Step 2: Dry-run to verify it renders**

Run: `node bin/agentgateway-demo.js usecase deploy ops-agents --dry-run` (adjust to this repo's actual CLI entrypoint if different - check `package.json`'s `bin` field)
Expected: YAML output includes a `tokenExchange.enabled: true` Helm values comment and a Postgres Secret/Service/PVC/Deployment, no errors.

- [ ] **Step 3: Commit**

```bash
git add config/usecases/enterprise/security/ops-agents.yaml
git commit -m "feat(ops-agents): enable the STS and its Postgres database"
```

---

### Task B3: Deploy the repo MCP server with two gateway routes (Standard + Delegation)

**Files:**
- Modify: `config/usecases/enterprise/security/ops-agents.yaml`

**Interfaces:**
- Produces: `repo-server-mcp` workload; HTTPRoute `repo-mcp-standard` (Standard/Impersonation exchange, proxy-native, no STS call) and HTTPRoute `repo-mcp-delegation` (validates STS-issued tokens only) both forwarding to the same pod.
- Consumes: Task A2's `repo-server` image, Task B1's fixed `oauth-token-exchange` feature, `obo-token-exchange` feature (unmodified - this is the feature that, despite its name, just applies a JWT-validation policy; see its own doc comment).

- [ ] **Step 1: Append the MCP server and its two routes**

Append to `config/usecases/enterprise/security/ops-agents.yaml`'s `spec.steps`:

```yaml
    - title: Deploy the repo MCP server
      description: |
        Deploys repo-server (Task A2) and exposes it at /repo-mcp-standard. This route
        will carry the Standard/Impersonation exchange.
      features:
        - name: mcp-server
          config:
            deployServer: true
            image: '{{env.images.opsAgents.repo-server-mcp}}'
            serverName: repo-server-mcp
            serverPort: 8000
            servicePort: 80
            protocol: StreamableHTTP
            mcpPath: /mcp
            pathPrefix: /repo-mcp-standard
            backendName: repo-mcp-standard-backend
            targetName: repo-mcp-standard-target
            routeName: repo-mcp-standard

    - title: Expose a second route to the same repo MCP server for the Delegation path
      description: |
        Same pod, a different gateway route and backend - this one carries only
        STS-issued (Delegation) tokens, never a raw Keycloak token.
      features:
        - name: mcp-server
          config:
            deployServer: false
            backendName: repo-mcp-delegation-backend
            targetName: repo-mcp-delegation-target
            routeName: repo-mcp-delegation
            pathPrefix: /repo-mcp-delegation
            targets:
              - name: repo-mcp-delegation-target
                host: repo-server-mcp.agentgateway-system.svc.cluster.local
                port: 80
                protocol: StreamableHTTP
                path: /mcp

    - title: Protect the standard route with the proxy-native Impersonation exchange
      description: |
        Reuses the same agw-token-exchange Keycloak client and OAUTH_TOKEN_EXCHANGE_CLIENT_SECRET
        already provisioned for this wizard's own auth pillar (see
        internal/manifests/templates/auth/token-exchange-standard.yaml.tmpl in the wizard repo).
      features:
        - name: oauth-token-exchange
          config:
            name: repo-mcp-standard-exchange
            targetRefs:
              - group: gateway.networking.k8s.io
                kind: HTTPRoute
                name: repo-mcp-standard
            jwtAuthentication:
              issuer: 'https://{{env.domains.core.keycloak}}/realms/agw-dev'
              audiences: [agentgateway]
              jwks:
                host: '{{env.domains.core.keycloak}}'
                path: realms/agw-dev/protocol/openid-connect/certs
            tokenEndpoint:
              host: '{{env.domains.core.keycloak}}'
              path: /realms/agw-dev/protocol/openid-connect/token
            clientAuth:
              clientId: agw-token-exchange

    - title: Protect the delegation route with STS-issued-token validation
      description: |
        No exchange happens at this boundary - the caller (an agent) already exchanged
        at the STS before arriving. This policy only validates that the token was
        actually issued by the STS.
      features:
        - name: obo-token-exchange
          config:
            policyName: repo-mcp-delegation-jwt-policy
            issuer: '{{env.internal.stsIssuer}}'
            jwksPath: /.well-known/jwks.json
            jwksBackend:
              name: enterprise-agentgateway
              namespace: agentgateway-system
              port: 7777
            audiences: [agentgateway]
            jwtMode: Strict
            targetRefs:
              - group: gateway.networking.k8s.io
                kind: HTTPRoute
                name: repo-mcp-delegation
```

- [ ] **Step 2: Dry-run to verify it renders**

Run: `node bin/agentgateway-demo.js usecase deploy ops-agents --dry-run`
Expected: both `EnterpriseAgentgatewayPolicy/repo-mcp-standard-exchange` (with `preserveToken: true`, confirming Task B1's fix took effect) and `EnterpriseAgentgatewayPolicy/repo-mcp-delegation-jwt-policy` appear in the output.

- [ ] **Step 3: Commit**

```bash
git add config/usecases/enterprise/security/ops-agents.yaml
git commit -m "feat(ops-agents): deploy repo-server MCP with standard and delegation routes"
```

---

### Task B4: Deploy the four agent containers

**Files:**
- Modify: `config/usecases/enterprise/security/ops-agents.yaml`

**Interfaces:**
- Produces: four `workload-agent` deployments, each reachable at `/<agentName>/run` through agentgateway, matching the field-kit agent deployment contract (`POST /run`, `GET /health` on port 8080).
- Consumes: Task A4-A7's four images; `workload-agent` feature (unmodified). `CLIENT_ID`/`CLIENT_SECRET`/`KEYCLOAK_URL`/`AUDIENCE`/`STOCK_AGENT_URL`/`USE_KEYCLOAK_EXCHANGE` env vars the feature always sets are unused by these containers (they don't do the Keycloak-native SA exchange this feature was originally built for) - harmless, left as-is rather than forking the feature for one less env var.

- [ ] **Step 1: Append the four agent deployments**

Append to `config/usecases/enterprise/security/ops-agents.yaml`'s `spec.steps`:

```yaml
    - title: Deploy the Repo Agent (single-hop, Standard + Delegation modes)
      features:
        - name: workload-agent
          config:
            agentName: repo-agent
            image: '{{env.images.opsAgents.repo-agent}}'
            pathPrefix: /repo-agent
            routeName: repo-agent
            useTokenExchange: true
            saTokenAudience: agentgateway
            llmBaseUrl: '{{env.internal.agentgateway}}/openai'
            additionalEnv:
              - name: STS_URL
                value: '{{env.internal.sts}}'
              - name: MCP_STANDARD_URL
                value: '{{env.internal.agentgateway}}/repo-mcp-standard/mcp'
              - name: MCP_DELEGATION_URL
                value: '{{env.internal.agentgateway}}/repo-mcp-delegation/mcp'

    - title: Deploy the Release Manager Agent (hop 1 of the delegation chain)
      features:
        - name: workload-agent
          config:
            agentName: release-manager-agent
            image: '{{env.images.opsAgents.release-manager-agent}}'
            pathPrefix: /release-manager-agent
            routeName: release-manager-agent
            useTokenExchange: true
            saTokenAudience: agentgateway
            llmBaseUrl: '{{env.internal.agentgateway}}/openai'
            additionalEnv:
              - name: STS_URL
                value: '{{env.internal.sts}}'
              - name: DEPLOY_AGENT_URL
                value: '{{env.internal.agentgateway}}/deploy-agent/run'

    - title: Deploy the Deploy Agent (hop 2, re-exchanges and surfaces the chain)
      features:
        - name: workload-agent
          config:
            agentName: deploy-agent
            image: '{{env.images.opsAgents.deploy-agent}}'
            pathPrefix: /deploy-agent
            routeName: deploy-agent
            useTokenExchange: true
            saTokenAudience: agentgateway
            llmBaseUrl: '{{env.internal.agentgateway}}/openai'
            additionalEnv:
              - name: STS_URL
                value: '{{env.internal.sts}}'
              - name: MCP_URL
                value: '{{env.internal.agentgateway}}/repo-mcp-delegation/mcp'

    - title: Deploy the GitHub Agent (Elicitation)
      features:
        - name: workload-agent
          config:
            agentName: github-agent
            image: '{{env.images.opsAgents.github-agent}}'
            pathPrefix: /github-agent
            routeName: github-agent
            useTokenExchange: false
            llmBaseUrl: '{{env.internal.agentgateway}}/openai'
            additionalEnv:
              - name: MCP_GITHUB_URL
                value: '{{env.internal.agentgateway}}/mcp-github'
```

- [ ] **Step 2: Dry-run to verify it renders**

Run: `node bin/agentgateway-demo.js usecase deploy ops-agents --dry-run`
Expected: four `Deployment`/`Service`/`ServiceAccount`/`HTTPRoute` sets, one per agent, each with the right `additionalEnv` entries.

- [ ] **Step 3: Commit**

```bash
git add config/usecases/enterprise/security/ops-agents.yaml
git commit -m "feat(ops-agents): deploy the four agent containers"
```

---

### Task B5: GitHub remote MCP backend with Elicitation

**Files:**
- Modify: `config/usecases/enterprise/security/ops-agents.yaml`

**Interfaces:**
- Produces: HTTPRoute `mcp-github` at `/mcp-github`, backed by GitHub's real remote MCP server (`api.githubcopilot.com`) behind an `entElicitation` policy.
- Consumes: `elicitation-secret` and `elicitation-backend` features (unmodified). Requires `GITHUB_CLIENT_ID`/`GITHUB_CLIENT_SECRET` in the environment at deploy time - **placeholder values until a real GitHub OAuth App is registered**, per this plan's locked scope (see the design spec).

**Known risk requiring live verification (Task C5):** `elicitation-backend`'s HTTPRoute always rewrites the matched path down to `/` (`URLRewrite ReplacePrefixMatch: '/'`), so a request to `/mcp-github/mcp` arrives at GitHub as `/mcp` (no trailing slash). `remote-mcp-over-https.yaml` demonstrates GitHub's real MCP server listening at `/mcp/` (trailing slash) via an explicit static-target `path` override - a feature `elicitation-backend`'s backend shape doesn't expose (its backend is always a bare `static: {host, port}`, no `path` field). If GitHub's server 404s or redirects without the trailing slash, Task A7's `github-agent` may need `MCP_GITHUB_URL` to end in a trailing slash (`/mcp-github/`) instead - confirm against the real deployment and adjust Task B4's `MCP_GITHUB_URL` value if so.

- [ ] **Step 1: Append the elicitation secret and backend**

Append to `config/usecases/enterprise/security/ops-agents.yaml`'s `spec.steps`:

```yaml
    - title: Create the GitHub OAuth client secret
      description: |
        Placeholder until a real GitHub OAuth App is registered - set GITHUB_CLIENT_ID
        and GITHUB_CLIENT_SECRET in the environment before deploying for real.
      features:
        - name: elicitation-secret
          config:
            secretName: ops-agents-github-oauth
            provider: github
            clientSecretEnvVar: GITHUB_CLIENT_SECRET

    - title: Configure the GitHub MCP backend with Elicitation
      description: |
        Points agentgateway at GitHub's own real remote MCP server. The first call from a
        user with no stored GitHub token returns a pending consent URL; after the user
        completes OAuth in the Solo UI, the STS injects a real GitHub token into every
        subsequent call automatically.
      features:
        - name: elicitation-backend
          config:
            backendName: github-mcp-backend
            policyName: github-mcp-elicitation
            routeName: mcp-github
            pathPrefix: /mcp-github
            secretName: ops-agents-github-oauth
            provider: github
            clientIdEnvVar: GITHUB_CLIENT_ID
            scopes: ['read:user', 'repo']
            redirectUri: 'https://{{env.domains.core.soloUi}}/age/elicitations'
            upstream:
              host: api.githubcopilot.com
              port: 443
              tls: true
```

- [ ] **Step 2: Dry-run to verify it renders**

Run: `GITHUB_CLIENT_ID=placeholder GITHUB_CLIENT_SECRET=placeholder node bin/agentgateway-demo.js usecase deploy ops-agents --dry-run`
Expected: `Secret/ops-agents-github-oauth`, `EnterpriseAgentgatewayBackend/github-mcp-backend`, `EnterpriseAgentgatewayPolicy/github-mcp-elicitation` (containing `entElicitation` and `tokenExchange.mode: ElicitationOnly`), `HTTPRoute/mcp-github`.

- [ ] **Step 3: Commit**

```bash
git add config/usecases/enterprise/security/ops-agents.yaml
git commit -m "feat(ops-agents): add the GitHub elicitation MCP backend"
```

---

### Task B6: Wire the five image references into aws-dev.yaml

**Files:**
- Modify: `config/environments/aws-dev.yaml`

**Interfaces:**
- Produces: `{{env.images.opsAgents.*}}` template values Tasks B3/B4 reference.

- [ ] **Step 1: Add the image group**

In `config/environments/aws-dev.yaml`, under `spec.images`, add (matching the existing `fdLoan`/`agentChaining` groups' style):

```yaml
    opsAgents:
      repo-server-mcp: australia-southeast1-docker.pkg.dev/field-engineering-apac/kasunt/repo-server:v0.1.0
      repo-agent: australia-southeast1-docker.pkg.dev/field-engineering-apac/kasunt/repo-agent:v0.1.0
      release-manager-agent: australia-southeast1-docker.pkg.dev/field-engineering-apac/kasunt/release-manager-agent:v0.1.0
      deploy-agent: australia-southeast1-docker.pkg.dev/field-engineering-apac/kasunt/deploy-agent:v0.1.0
      github-agent: australia-southeast1-docker.pkg.dev/field-engineering-apac/kasunt/github-agent:v0.1.0
```

- [ ] **Step 2: Commit**

```bash
git add config/environments/aws-dev.yaml
git commit -m "chore(aws-dev): add ops-agents image references"
```

---

## Phase C: wizard integration (this repo)

### Task C1: Config - ops-agent endpoint URLs

Only three of the four agents are called directly by the wizard - `deploy-agent` is only ever called by `release-manager-agent` itself (Task A5/A6), never by the wizard. All three default to the in-cluster gateway DNS name, the same convention `defaultSTSHost` already uses - the wizard pod runs in the same cluster/namespace as agentgateway, so these are never "missing", matching `STSHost`/`STSPort`'s existing treatment.

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Interfaces:**
- Produces: `Config.RepoAgentURL`, `Config.ReleaseManagerAgentURL`, `Config.GitHubAgentURL` (all `string`), consumed by Task C2's handler.

- [ ] **Step 1: Write failing tests**

Add to `internal/config/config_test.go`:

```go
func TestLoadDefaultsOpsAgentURLsWhenUnset(t *testing.T) {
	t.Setenv("REPO_AGENT_URL", "")
	t.Setenv("RELEASE_MANAGER_AGENT_URL", "")
	t.Setenv("GITHUB_AGENT_URL", "")

	cfg := Load()

	if cfg.RepoAgentURL != defaultAgentGatewayInternalURL+"/repo-agent/run" {
		t.Fatalf("unexpected default RepoAgentURL: %q", cfg.RepoAgentURL)
	}
	if cfg.ReleaseManagerAgentURL != defaultAgentGatewayInternalURL+"/release-manager-agent/run" {
		t.Fatalf("unexpected default ReleaseManagerAgentURL: %q", cfg.ReleaseManagerAgentURL)
	}
	if cfg.GitHubAgentURL != defaultAgentGatewayInternalURL+"/github-agent/run" {
		t.Fatalf("unexpected default GitHubAgentURL: %q", cfg.GitHubAgentURL)
	}
}

func TestLoadRespectsOpsAgentURLOverrides(t *testing.T) {
	t.Setenv("REPO_AGENT_URL", "http://repo-agent.other-namespace.svc.cluster.local:8080/run")

	cfg := Load()

	if cfg.RepoAgentURL != "http://repo-agent.other-namespace.svc.cluster.local:8080/run" {
		t.Fatalf("expected the overridden RepoAgentURL, got %q", cfg.RepoAgentURL)
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/config/...`
Expected: FAIL - `cfg.RepoAgentURL` undefined (compile error).

- [ ] **Step 3: Add the fields**

In `internal/config/config.go`, add a new const alongside `defaultSTSHost`/`defaultSTSPort`:

```go
	// defaultAgentGatewayInternalURL is agentgateway's own in-cluster Service
	// DNS name, matching field-kit's config/environments/aws-dev.yaml
	// (spec.internal.agentgateway) - the wizard pod runs in the same
	// cluster/namespace, so it reaches the ops-agents containers through the
	// same gateway a browser would use externally.
	defaultAgentGatewayInternalURL = "http://agentgateway-gw.agentgateway-system.svc.cluster.local:8080"
```

Add fields to the `Config` struct:

```go
	RepoAgentURL           string
	ReleaseManagerAgentURL string
	GitHubAgentURL         string
```

Add to `Load()`'s returned struct literal:

```go
		RepoAgentURL:           envOrDefault("REPO_AGENT_URL", defaultAgentGatewayInternalURL+"/repo-agent/run"),
		ReleaseManagerAgentURL: envOrDefault("RELEASE_MANAGER_AGENT_URL", defaultAgentGatewayInternalURL+"/release-manager-agent/run"),
		GitHubAgentURL:         envOrDefault("GITHUB_AGENT_URL", defaultAgentGatewayInternalURL+"/github-agent/run"),
```

Do not add these to `Missing()` - they're never required, same as `STSHost`/`STSPort`.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/config/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add ops-agent endpoint URLs"
```

---

### Task C2: Go backend - ops-agent proxy handler

This is a new, separate handler from the existing `internal/api/agent_handler.go` - that one backs the unrelated, pre-existing "agent chaining" pillar's in-process `agent.Runner` loop (streamed SSE hops, no external container, no captured-token forwarding). This handler proxies to the real external ops-agents containers over plain HTTP, forwarding the session token captured by the auth pillar's login step, and returns a single JSON response (the Python agents' `/run` contract is not streaming).

**Files:**
- Create: `internal/api/ops_agent_handler.go`
- Create: `internal/api/ops_agent_handler_test.go`
- Modify: `internal/api/server.go`
- Modify: `cmd/wizard/main.go`

**Interfaces:**
- Produces: `POST /api/ops-agent/run`, request `{"identity", "agent", "mode", "query"}`, response `{"answer", "extra"}` (passthrough of the agent's own `/run` response shape from Task A3's harness).
- Consumes: `s.cfg.RepoAgentURL`/`ReleaseManagerAgentURL`/`GitHubAgentURL` (Task C1), `s.oauthStore.GetToken` / `s.sessionIDFromCookie` (existing, from `internal/oauthlogin`).

- [ ] **Step 1: Write failing tests**

Create `internal/api/ops_agent_handler_test.go`:

```go
package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/day0ops/agentgateway-demo-wizard/internal/config"
	"github.com/day0ops/agentgateway-demo-wizard/internal/oauthlogin"
)

func TestOpsAgentRunRequiresLogin(t *testing.T) {
	s := NewServer(WithOAuthLogin("https://keycloak.demo.example.com", "/dev/null"), WithConfig(&config.Config{}))

	reqBody := `{"identity":"team-alpha","agent":"repo","mode":"standard","query":"list repos"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ops-agent/run", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when no real session token has been captured, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOpsAgentRunForwardsTokenAndModeToTheTargetAgent(t *testing.T) {
	var gotBody string
	agentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"answer":"3 repos found","extra":{"chain":{"sub":"team-alpha"}}}`))
	}))
	defer agentServer.Close()

	s := NewServer(
		WithOAuthLogin("https://keycloak.demo.example.com", "/dev/null"),
		WithConfig(&config.Config{RepoAgentURL: agentServer.URL + "/run"}),
	)
	s.oauthStore.PutToken("session-1", "team-alpha", oauthlogin.Token{AccessToken: "real-token", ExpiresAt: time.Now().Add(time.Hour)})

	reqBody := `{"identity":"team-alpha","agent":"repo","mode":"sts-delegation","query":"list repos"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ops-agent/run", strings.NewReader(reqBody))
	req.AddCookie(&http.Cookie{Name: oauthlogin.SessionCookieName, Value: "session-1"})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(gotBody, `"token":"real-token"`) || !strings.Contains(gotBody, `"mode":"sts-delegation"`) {
		t.Fatalf("expected the captured token and mode to be forwarded, got: %s", gotBody)
	}
	if !strings.Contains(rec.Body.String(), "3 repos found") || !strings.Contains(rec.Body.String(), "chain") {
		t.Fatalf("expected the agent's answer and extra fields to pass through, got: %s", rec.Body.String())
	}
}

func TestOpsAgentRunRejectsUnknownAgent(t *testing.T) {
	s := NewServer(WithOAuthLogin("https://keycloak.demo.example.com", "/dev/null"), WithConfig(&config.Config{}))
	s.oauthStore.PutToken("session-1", "team-alpha", oauthlogin.Token{AccessToken: "real-token", ExpiresAt: time.Now().Add(time.Hour)})

	reqBody := `{"identity":"team-alpha","agent":"nonexistent","query":"hi"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ops-agent/run", strings.NewReader(reqBody))
	req.AddCookie(&http.Cookie{Name: oauthlogin.SessionCookieName, Value: "session-1"})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unknown agent, got %d: %s", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/api/...`
Expected: FAIL - route `/api/ops-agent/run` not registered (404s instead of the expected status codes).

- [ ] **Step 3: Write the handler**

Create `internal/api/ops_agent_handler.go`:

```go
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type opsAgentRunRequest struct {
	Identity string `json:"identity"`
	Agent    string `json:"agent"`
	Mode     string `json:"mode"`
	Query    string `json:"query"`
}

type opsAgentRunResponse struct {
	Answer string         `json:"answer"`
	Extra  map[string]any `json:"extra,omitempty"`
}

var opsAgentHTTPClient = &http.Client{Timeout: 45 * time.Second}

func (s *Server) opsAgentURL(name string) (string, error) {
	switch name {
	case "repo":
		return s.cfg.RepoAgentURL, nil
	case "release-manager":
		return s.cfg.ReleaseManagerAgentURL, nil
	case "github":
		return s.cfg.GitHubAgentURL, nil
	default:
		return "", fmt.Errorf("unknown agent %q", name)
	}
}

// handleOpsAgentRun proxies to one of the ops-agents usecase's real external
// agent containers, forwarding the real session token captured by the auth
// pillar's login step. Unlike handleAgentRun (this repo's own in-process
// agent loop), the target is a separate HTTP service and the response is a
// single JSON object, not a stream.
func (s *Server) handleOpsAgentRun(w http.ResponseWriter, r *http.Request) {
	var req opsAgentRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	url, err := s.opsAgentURL(req.Agent)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}

	sessionID := s.sessionIDFromCookie(r)
	tok, ok := s.oauthStore.GetToken(sessionID, req.Identity)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, fmt.Errorf("log in as %q first - see the login step above", req.Identity))
		return
	}

	body, err := json.Marshal(map[string]string{
		"query": req.Query,
		"token": tok.AccessToken,
		"mode":  req.Mode,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	agentReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	agentReq.Header.Set("Content-Type", "application/json")

	resp, err := opsAgentHTTPClient.Do(agentReq)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err)
		return
	}
	defer resp.Body.Close()

	var parsed opsAgentRunResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		writeJSONError(w, http.StatusBadGateway, fmt.Errorf("decoding agent response: %w", err))
		return
	}

	writeJSON(w, http.StatusOK, parsed)
}
```

In `internal/api/server.go`'s `NewServer`, add the route next to the existing `/api/agent/run` line:

```go
	s.mux.HandleFunc("POST /api/ops-agent/run", s.handleOpsAgentRun)
```

In `cmd/wizard/main.go`, no new `Option` is needed - the handler only reads `s.cfg` (already wired via `api.WithConfig(cfg)`) and `s.oauthStore` (already wired via `api.WithOAuthLogin(...)`), both already present in the existing `api.NewServer(...)` call.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/api/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/ops_agent_handler.go internal/api/ops_agent_handler_test.go internal/api/server.go
git commit -m "feat(api): add the ops-agent proxy handler"
```

---

### Task C3: Registry - four new auth-pillar steps

**Files:**
- Modify: `internal/scenarios/types.go`
- Modify: `internal/scenarios/registry.go`
- Modify: `internal/scenarios/auth_test.go`
- Modify: `internal/scenarios/routing_test.go` (read-only, no change expected - listed for the implementer to re-run, since it shares `opsAgentHTTPRouteGVR`'s sibling pattern)

**Interfaces:**
- Produces: `scenarios.OpsAgentDemo` struct, `Step.OpsAgent *OpsAgentDemo` field, four new steps under the `auth` pillar (`auth-agent-impersonation`, `auth-agent-delegation`, `auth-agent-chain`, `auth-agent-elicitation`).
- Consumes: Task C2's `/api/ops-agent/run` contract (`agent`/`mode` values must match `opsAgentURL`'s switch cases exactly: `"repo"`, `"release-manager"`, `"github"`).

- [ ] **Step 1: Write failing tests**

Update `internal/scenarios/auth_test.go`'s `TestRegistryIncludesPillar2Auth`:

```go
	if len(auth.Steps) != 8 {
		t.Fatalf("expected 8 auth steps, got %d", len(auth.Steps))
	}

	wantStepIDs := []string{
		"auth-login", "auth-rbac", "auth-exchange-standard", "auth-exchange-sts",
		"auth-agent-impersonation", "auth-agent-delegation", "auth-agent-chain", "auth-agent-elicitation",
	}
```

Update `TestAuthPoliciesRenderValidManifests`'s skip condition (it currently only skips `LoginDemo` steps; the four new steps are `ReadOnly`/`ViewRefs`-only, like the routing pillar's, so `Policy.Render` returns `nil, nil` for them - which would fail this test's `len(docs) == 0` check otherwise):

```go
	for _, step := range auth.Steps {
		if step.LoginDemo || step.OpsAgent != nil {
			continue // no templated manifest to render - see TestAuthOpsAgentStepsAreReadOnlyAndNeverApply
		}
```

Add a new test, mirroring `routing_test.go`'s `TestRoutingPoliciesAreReadOnlyAndNeverApply`:

```go
func TestAuthOpsAgentStepsAreReadOnlyAndNeverApply(t *testing.T) {
	auth := Registry()[2]

	wantViewRefNames := map[string][]string{
		"auth-agent-impersonation": {"repo-agent", "repo-mcp-standard"},
		"auth-agent-delegation":    {"repo-agent", "repo-mcp-delegation"},
		"auth-agent-chain":         {"release-manager-agent", "deploy-agent", "repo-mcp-delegation"},
		"auth-agent-elicitation":   {"github-agent", "mcp-github"},
	}
	wantAgent := map[string]string{
		"auth-agent-impersonation": "repo",
		"auth-agent-delegation":    "repo",
		"auth-agent-chain":         "release-manager",
		"auth-agent-elicitation":   "github",
	}
	wantMode := map[string]string{
		"auth-agent-impersonation": "standard",
		"auth-agent-delegation":    "sts-delegation",
		"auth-agent-chain":         "",
		"auth-agent-elicitation":   "",
	}

	for _, step := range auth.Steps {
		want, hasPolicy := wantViewRefNames[step.ID]
		if !hasPolicy {
			continue
		}
		if step.OpsAgent == nil {
			t.Fatalf("step %q: expected OpsAgent to be set", step.ID)
		}
		if step.OpsAgent.Agent != wantAgent[step.ID] {
			t.Fatalf("step %q: OpsAgent.Agent = %q, want %q", step.ID, step.OpsAgent.Agent, wantAgent[step.ID])
		}
		if step.OpsAgent.Mode != wantMode[step.ID] {
			t.Fatalf("step %q: OpsAgent.Mode = %q, want %q", step.ID, step.OpsAgent.Mode, wantMode[step.ID])
		}
		if step.OpsAgent.DefaultPrompt == "" {
			t.Fatalf("step %q: expected a non-empty DefaultPrompt", step.ID)
		}

		if len(step.Policies) != 1 {
			t.Fatalf("step %q: expected exactly 1 policy, got %d", step.ID, len(step.Policies))
		}
		p := step.Policies[0]
		if !p.ReadOnly {
			t.Fatalf("step %q: expected policy to be ReadOnly", step.ID)
		}
		if len(p.ViewRefs) != len(want) {
			t.Fatalf("step %q: expected %d ViewRefs, got %d", step.ID, len(want), len(p.ViewRefs))
		}
		for i, name := range want {
			if p.ViewRefs[i].Name != name {
				t.Fatalf("step %q: ViewRefs[%d].Name = %q, want %q", step.ID, i, p.ViewRefs[i].Name, name)
			}
			if p.ViewRefs[i].GVR.Resource != "httproutes" {
				t.Fatalf("step %q: ViewRefs[%d].GVR.Resource = %q, want httproutes", step.ID, i, p.ViewRefs[i].GVR.Resource)
			}
		}

		docs, err := p.Render(ManifestParams{Namespace: "agentgateway-system"})
		if err != nil {
			t.Fatalf("step %q: Render errored: %v", step.ID, err)
		}
		if docs != nil {
			t.Fatalf("step %q: expected a read-only policy to render nothing, got %v", step.ID, docs)
		}
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/scenarios/...`
Expected: FAIL - `step.OpsAgent` undefined (compile error), step count mismatch.

- [ ] **Step 3: Add the `OpsAgentDemo` type**

In `internal/scenarios/types.go`, add after the `Step` struct's `LoginDemo` field:

```go
	// OpsAgent marks a step that drives one of the ops-agents usecase's real
	// external agent containers (see internal/api/ops_agent_handler.go)
	// instead of a Configure+DriveRequest pane pair or this repo's own
	// built-in agent loop (AgentDemo).
	OpsAgent *OpsAgentDemo `json:"opsAgent,omitempty"`
```

Add the type itself, after the `Step` struct:

```go
// OpsAgentDemo configures the pane for a step that calls a real external
// agent container through POST /api/ops-agent/run.
type OpsAgentDemo struct {
	// Agent selects which container's /run endpoint this step calls - must
	// match one of the cases in Server.opsAgentURL: "repo",
	// "release-manager", or "github".
	Agent string `json:"agent"`
	// Mode is forwarded to the agent as-is; empty for agents that don't
	// branch on mode (release-manager, github).
	Mode          string `json:"mode"`
	DefaultPrompt string `json:"defaultPrompt"`
}
```

- [ ] **Step 4: Add the GVR and the four steps**

In `internal/scenarios/registry.go`, add a second GVR next to `routingBackendGVR`:

```go
// opsAgentHTTPRouteGVR identifies the HTTPRoutes the ops-agents usecase
// creates for the four agent containers and the two repo-MCP routes. These
// objects are pre-provisioned by the agentgateway-field-kit ops-agents
// usecase, not by this repo.
var opsAgentHTTPRouteGVR = schema.GroupVersionResource{
	Group:    "gateway.networking.k8s.io",
	Version:  "v1",
	Resource: "httproutes",
}
```

Append four steps to `authPillar()`'s `Steps` slice, right after the `auth-exchange-sts` step:

```go
			{
				ID:    "auth-agent-impersonation",
				Title: "Agent → MCP (Impersonation)",
				Explanation: "The same Standard/Impersonation exchange from the previous step, but now " +
					"the caller is a real agent instead of a browser hitting a demo echo endpoint. The " +
					"Repo Agent takes your real session token as-is and presents it straight to the repo " +
					"MCP server's gateway route; agentgateway performs the Impersonation exchange itself " +
					"before forwarding - the agent never talks to the STS.",
				Diagram: "sequenceDiagram\n" +
					"  participant You\n" +
					"  participant Wizard\n" +
					"  participant Repo as Repo Agent\n" +
					"  participant AGW as agentgateway\n" +
					"  participant KC as Keycloak\n" +
					"  participant MCP as Repo MCP server\n" +
					"  You->>Wizard: \"list our repos\"\n" +
					"  Wizard->>Repo: POST /repo-agent/run (token, mode=standard)\n" +
					"  Repo->>AGW: MCP call (Bearer: your token, unchanged)\n" +
					"  AGW->>KC: token-exchange (Impersonation)\n" +
					"  AGW->>MCP: forward with exchanged token\n" +
					"  MCP-->>Repo: tool result\n" +
					"  Repo-->>Wizard: answer\n" +
					"  Wizard-->>You: answer",
				Policies: []Policy{
					{
						ID:       "auth-agent-impersonation",
						Title:    "Repo Agent + Standard exchange route",
						ReadOnly: true,
						ViewRefs: []ViewRef{
							{GVR: opsAgentHTTPRouteGVR, Name: "repo-agent"},
							{GVR: opsAgentHTTPRouteGVR, Name: "repo-mcp-standard"},
						},
					},
				},
				OpsAgent: &OpsAgentDemo{
					Agent:         "repo",
					Mode:          "standard",
					DefaultPrompt: "List our repositories and summarize their status.",
				},
			},
			{
				ID:    "auth-agent-delegation",
				Title: "Agent → MCP (Delegation)",
				Explanation: "Same agent, same MCP server, different mode: this time the Repo Agent " +
					"exchanges your token at the STS itself before calling the gateway, presenting its " +
					"own Kubernetes ServiceAccount token as the actor. The resulting token carries both " +
					"sub (you) and act (the Repo Agent) - the gateway route behind this mode only ever " +
					"sees STS-issued tokens.",
				Diagram: "sequenceDiagram\n" +
					"  participant You\n" +
					"  participant Wizard\n" +
					"  participant Repo as Repo Agent\n" +
					"  participant STS\n" +
					"  participant AGW as agentgateway\n" +
					"  participant MCP as Repo MCP server\n" +
					"  You->>Wizard: \"list our repos\"\n" +
					"  Wizard->>Repo: POST /repo-agent/run (token, mode=sts-delegation)\n" +
					"  Repo->>STS: POST /oauth2/token (subject=your token, actor=Repo Agent's SA token)\n" +
					"  STS-->>Repo: token (sub=you, act=repo-agent)\n" +
					"  Repo->>AGW: MCP call (Bearer: delegated token)\n" +
					"  AGW->>STS: validate JWT\n" +
					"  AGW->>MCP: forward\n" +
					"  MCP-->>Repo: tool result\n" +
					"  Repo-->>Wizard: answer\n" +
					"  Wizard-->>You: answer",
				Policies: []Policy{
					{
						ID:       "auth-agent-delegation",
						Title:    "Repo Agent + STS delegation route",
						ReadOnly: true,
						ViewRefs: []ViewRef{
							{GVR: opsAgentHTTPRouteGVR, Name: "repo-agent"},
							{GVR: opsAgentHTTPRouteGVR, Name: "repo-mcp-delegation"},
						},
					},
				},
				OpsAgent: &OpsAgentDemo{
					Agent:         "repo",
					Mode:          "sts-delegation",
					DefaultPrompt: "List our repositories and summarize their status.",
				},
			},
			{
				ID:    "auth-agent-chain",
				Title: "Agent → agent → MCP (chain of custody)",
				Explanation: "A two-hop chain: the Release Manager Agent exchanges your token at the " +
					"STS, then calls the Deploy Agent - passing it the already-exchanged token, not your " +
					"raw one. The Deploy Agent exchanges it again before calling MCP, presenting its own " +
					"SA token as actor. The result is a token with a nested act claim: act.sub is the " +
					"Deploy Agent, act.act.sub is the Release Manager Agent - a cryptographic record of " +
					"every hop a request passed through, signed by the STS at each step. Per RFC 8693, " +
					"this nested chain is informational only - only the outermost actor governs access " +
					"control - but it gives a real audit trail of custody a single flat token can't.",
				Diagram: "sequenceDiagram\n" +
					"  participant You\n" +
					"  participant Wizard\n" +
					"  participant RM as Release Manager Agent\n" +
					"  participant STS\n" +
					"  participant DA as Deploy Agent\n" +
					"  participant AGW as agentgateway\n" +
					"  participant MCP as Repo MCP server\n" +
					"  You->>Wizard: \"deploy payments-service to staging\"\n" +
					"  Wizard->>RM: POST /release-manager-agent/run (token)\n" +
					"  RM->>STS: exchange (subject=your token, actor=RM's SA token)\n" +
					"  STS-->>RM: T1 (sub=you, act=release-manager-agent)\n" +
					"  RM->>DA: POST /deploy-agent/run (T1)\n" +
					"  DA->>STS: exchange (subject=T1, actor=DA's SA token)\n" +
					"  STS-->>DA: T2 (sub=you, act=deploy-agent, act.act=release-manager-agent)\n" +
					"  DA->>AGW: MCP call (Bearer: T2)\n" +
					"  AGW->>MCP: forward\n" +
					"  MCP-->>DA: tool result\n" +
					"  DA-->>RM: answer + decoded chain\n" +
					"  RM-->>Wizard: answer + chain\n" +
					"  Wizard-->>You: answer + chain",
				Policies: []Policy{
					{
						ID:       "auth-agent-chain",
						Title:    "Release Manager + Deploy Agent chain",
						ReadOnly: true,
						ViewRefs: []ViewRef{
							{GVR: opsAgentHTTPRouteGVR, Name: "release-manager-agent"},
							{GVR: opsAgentHTTPRouteGVR, Name: "deploy-agent"},
							{GVR: opsAgentHTTPRouteGVR, Name: "repo-mcp-delegation"},
						},
					},
				},
				OpsAgent: &OpsAgentDemo{
					Agent:         "release-manager",
					DefaultPrompt: "Deploy the payments-service to staging and confirm the PR is merged first.",
				},
			},
			{
				ID:    "auth-agent-elicitation",
				Title: "Agent → external API (Elicitation)",
				Explanation: "Not every credential an agent needs lives in Keycloak. The GitHub Agent " +
					"calls GitHub's own real remote MCP server on your behalf - but the gateway has no " +
					"GitHub token for you yet. Instead of failing, the STS returns a one-time consent " +
					"URL; after you approve it in the Solo UI, the STS stores a real GitHub OAuth token " +
					"and injects it into every later call automatically. The agent never sees or stores " +
					"that token itself.",
				Diagram: "sequenceDiagram\n" +
					"  participant You\n" +
					"  participant Wizard\n" +
					"  participant GHA as GitHub Agent\n" +
					"  participant AGW as agentgateway\n" +
					"  participant STS\n" +
					"  participant UI as Solo UI\n" +
					"  participant GitHub\n" +
					"  You->>Wizard: \"what are my GitHub repos?\"\n" +
					"  Wizard->>GHA: POST /github-agent/run (token)\n" +
					"  GHA->>AGW: MCP call (Bearer: your token)\n" +
					"  AGW->>STS: check for a stored GitHub token\n" +
					"  STS-->>AGW: none yet - elicitation required\n" +
					"  AGW-->>GHA: pending + consent URL\n" +
					"  GHA-->>Wizard: \"open this link to connect GitHub\"\n" +
					"  You->>UI: open the link, approve\n" +
					"  UI->>GitHub: OAuth consent\n" +
					"  GitHub-->>STS: GitHub access token (stored)\n" +
					"  You->>Wizard: retry\n" +
					"  Wizard->>GHA: POST /github-agent/run (token)\n" +
					"  GHA->>AGW: MCP call (Bearer: your token)\n" +
					"  AGW->>STS: check for a stored GitHub token\n" +
					"  STS-->>AGW: found - inject it\n" +
					"  AGW-->>GHA: real GitHub data",
				Policies: []Policy{
					{
						ID:       "auth-agent-elicitation",
						Title:    "GitHub Agent + Elicitation route",
						ReadOnly: true,
						ViewRefs: []ViewRef{
							{GVR: opsAgentHTTPRouteGVR, Name: "github-agent"},
							{GVR: opsAgentHTTPRouteGVR, Name: "mcp-github"},
						},
					},
				},
				OpsAgent: &OpsAgentDemo{
					Agent:         "github",
					DefaultPrompt: "What are my most recently updated GitHub repositories?",
				},
			},
```

- [ ] **Step 5: Run to verify pass**

Run: `go test ./internal/scenarios/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/scenarios/types.go internal/scenarios/registry.go internal/scenarios/auth_test.go
git commit -m "feat(scenarios): add the four ops-agent auth-pillar steps"
```

---

### Task C4: Frontend - OpsAgentRunPane and App wiring

**Files:**
- Create: `web/src/components/OpsAgentRunPane.tsx`
- Create: `web/src/components/OpsAgentRunPane.test.tsx`
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/App.tsx`

**Interfaces:**
- Produces: `<OpsAgentRunPane identity agent mode defaultPrompt />`, rendered by `App.tsx` whenever `activeStep.opsAgent` is set.
- Consumes: `POST /api/ops-agent/run` (Task C2).

- [ ] **Step 1: Write failing tests**

Create `web/src/components/OpsAgentRunPane.test.tsx`:

```tsx
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { OpsAgentRunPane } from "./OpsAgentRunPane";

function mockJsonFetch(body: unknown, ok = true) {
  return (() =>
    Promise.resolve({
      ok,
      status: ok ? 200 : 502,
      json: async () => body,
      text: async () => JSON.stringify(body),
    } as unknown as Response)) as unknown as typeof fetch;
}

describe("OpsAgentRunPane", () => {
  it("renders the agent's answer after a run", async () => {
    globalThis.fetch = mockJsonFetch({ answer: "3 repos found" });

    render(
      <OpsAgentRunPane
        identity="team-alpha"
        agent="repo"
        mode="standard"
        defaultPrompt="List our repos"
      />,
    );
    fireEvent.click(screen.getByText("Run agent"));

    await waitFor(() =>
      expect(screen.getByText("3 repos found")).toBeInTheDocument(),
    );
  });

  it("renders the decoded delegation chain when present", async () => {
    globalThis.fetch = mockJsonFetch({
      answer: "Deployed payments-service.",
      extra: { chain: { sub: "team-alpha", act: "deploy-agent", act_act: "release-manager-agent" } },
    });

    render(
      <OpsAgentRunPane
        identity="team-alpha"
        agent="release-manager"
        mode=""
        defaultPrompt="Deploy payments-service"
      />,
    );
    fireEvent.click(screen.getByText("Run agent"));

    await waitFor(() =>
      expect(screen.getByText(/act: deploy-agent/)).toBeInTheDocument(),
    );
    expect(screen.getByText(/act.act: release-manager-agent/)).toBeInTheDocument();
  });

  it("surfaces a non-OK response as an error instead of crashing on json()", async () => {
    globalThis.fetch = (() =>
      Promise.resolve({
        ok: false,
        status: 401,
        text: async () => "log in as \"team-alpha\" first",
      } as unknown as Response)) as unknown as typeof fetch;

    render(
      <OpsAgentRunPane
        identity="team-alpha"
        agent="repo"
        mode="standard"
        defaultPrompt="List our repos"
      />,
    );
    fireEvent.click(screen.getByText("Run agent"));

    await waitFor(() =>
      expect(screen.getByText('log in as "team-alpha" first')).toBeInTheDocument(),
    );
  });
});
```

- [ ] **Step 2: Run to verify fail**

Run: `cd web && npx vitest run src/components/OpsAgentRunPane.test.tsx`
Expected: FAIL - module not found.

- [ ] **Step 3: Write the component**

Create `web/src/components/OpsAgentRunPane.tsx`:

```tsx
import { useState } from "react";

type OpsAgentChain = {
  sub?: string;
  act?: string;
  act_act?: string;
};

type OpsAgentResult = {
  answer: string;
  extra?: { chain?: OpsAgentChain } & Record<string, unknown>;
};

function linkify(text: string) {
  const parts = text.split(/(https?:\/\/[^\s]+)/g);
  return parts.map((part, i) =>
    /^https?:\/\//.test(part) ? (
      <a
        key={i}
        href={part}
        target="_blank"
        rel="noreferrer"
        className="text-indigo-600 underline dark:text-indigo-400"
      >
        {part}
      </a>
    ) : (
      <span key={i}>{part}</span>
    ),
  );
}

export function OpsAgentRunPane({
  identity,
  agent,
  mode,
  defaultPrompt,
}: {
  identity: string;
  agent: string;
  mode: string;
  defaultPrompt: string;
}) {
  const [query, setQuery] = useState(defaultPrompt);
  const [result, setResult] = useState<OpsAgentResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [running, setRunning] = useState(false);

  async function run() {
    setRunning(true);
    setError(null);
    setResult(null);

    try {
      const res = await fetch("/api/ops-agent/run", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ identity, agent, mode, query }),
      });
      if (!res.ok) {
        setError(await res.text());
        return;
      }
      setResult((await res.json()) as OpsAgentResult);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setRunning(false);
    }
  }

  const chain = result?.extra?.chain;

  return (
    <div className="space-y-4">
      <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-500">
        Agent
      </h2>
      <div className="space-y-3 rounded-lg border border-slate-200 p-4 dark:border-slate-800">
        <textarea
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          rows={2}
          className="w-full rounded border border-slate-200 bg-transparent p-2 text-sm dark:border-slate-700"
        />
        <button
          onClick={run}
          disabled={running}
          className="rounded bg-slate-900 px-3 py-1 text-sm text-white disabled:opacity-50 dark:bg-slate-100 dark:text-slate-900"
        >
          Run agent
        </button>
      </div>

      {error && (
        <div className="rounded-lg border border-red-500 bg-red-50 p-3 text-sm dark:bg-red-950">
          {error}
        </div>
      )}

      {result && (
        <div className="space-y-3 rounded-lg border border-slate-200 p-3 text-sm dark:border-slate-800">
          <div>{linkify(result.answer)}</div>
          {chain && (
            <div className="flex flex-wrap items-center gap-2 text-xs text-slate-500">
              <span className="rounded bg-slate-100 px-2 py-1 dark:bg-slate-800">
                sub: {chain.sub}
              </span>
              {chain.act && (
                <>
                  <span>→</span>
                  <span className="rounded bg-slate-100 px-2 py-1 dark:bg-slate-800">
                    act: {chain.act}
                  </span>
                </>
              )}
              {chain.act_act && (
                <>
                  <span>→</span>
                  <span className="rounded bg-slate-100 px-2 py-1 dark:bg-slate-800">
                    act.act: {chain.act_act}
                  </span>
                </>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 4: Run to verify pass**

Run: `cd web && npx vitest run src/components/OpsAgentRunPane.test.tsx`
Expected: PASS (3 tests).

- [ ] **Step 5: Wire the type and App.tsx**

In `web/src/lib/api.ts`, add:

```ts
export type OpsAgentDemo = {
  agent: string;
  mode: string;
  defaultPrompt: string;
};
```

Add to the `Step` type:

```ts
  opsAgent?: OpsAgentDemo;
```

In `web/src/App.tsx`, add the import:

```tsx
import { OpsAgentRunPane } from "./components/OpsAgentRunPane";
```

Update `hasDemo`:

```tsx
  const hasDemo =
    activeStep.agentDemo ||
    activeStep.loginDemo ||
    !!activeStep.opsAgent ||
    activeStep.policies.length > 0 ||
    activeStep.presets.length > 0;
```

Update the right-column pane dispatch:

```tsx
            {activeStep.opsAgent ? (
              <OpsAgentRunPane
                key={`${activeStep.id}-ops-agent`}
                identity="team-alpha"
                agent={activeStep.opsAgent.agent}
                mode={activeStep.opsAgent.mode}
                defaultPrompt={activeStep.opsAgent.defaultPrompt}
              />
            ) : activeStep.agentDemo ? (
              <AgentRunPane
                key={`${activeStep.id}-agent`}
                identity="team-alpha"
                model="gpt-4o-mini"
              />
            ) : (
              <DriveRequestPane
                key={`${activeStep.id}-request`}
                presets={activeStep.presets}
                onResponse={recordResponseCost}
              />
            )}
```

- [ ] **Step 6: Run the full frontend test suite**

Run: `cd web && npm test`
Expected: PASS, all suites including `App`-level tests if any exist.

- [ ] **Step 7: Commit**

```bash
git add web/src/components/OpsAgentRunPane.tsx web/src/components/OpsAgentRunPane.test.tsx web/src/lib/api.ts web/src/App.tsx
git commit -m "feat(web): add OpsAgentRunPane and wire it into the auth pillar"
```

---

### Task C5: Full verification pass

**Files:** none (verification only)

- [ ] **Step 1: Backend**

Run: `go build ./... && go test ./...`
Expected: builds clean, all tests pass.

- [ ] **Step 2: Frontend**

Run: `cd web && npm test && npm run build`
Expected: all tests pass, production build succeeds.

- [ ] **Step 3: field-kit dry-run**

In the `agentgateway-field-kit` repo: `node bin/agentgateway-demo.js usecase deploy ops-agents --dry-run` (confirm the actual CLI entrypoint name from `package.json`'s `bin` field if different).
Expected: full YAML output, no errors, `preserveToken: true` present on the standard-exchange policy (confirms Task B1's fix is live).

- [ ] **Step 4: Build and push all five images (if not already done in Task A8)**

Confirm the five images referenced in `config/environments/aws-dev.yaml`'s `images.opsAgents` group actually exist in the registry.

- [ ] **Step 5: Live deploy and smoke test**

This step mutates the live cluster - confirm with the user before running. In the `agentgateway-field-kit` repo:

```bash
GITHUB_CLIENT_ID=<real or placeholder> GITHUB_CLIENT_SECRET=<real or placeholder> \
  node bin/agentgateway-demo.js usecase deploy ops-agents
```

Then, against the live wizard UI (or directly via curl, logged in as `team-alpha`):

1. **Impersonation step**: run the Repo Agent in `standard` mode. Confirm a real answer referencing `repo-server`'s `list_repos` data.
2. **Delegation step**: run the Repo Agent in `sts-delegation` mode. Confirm a real answer; decode the JWT the agent used (log it temporarily if needed) and confirm `act.sub == "repo-agent"`.
3. **Chain step**: run the Release Manager Agent. Confirm the response's `extra.chain` shows `sub` = your identity, `act` = `deploy-agent`, `act_act` = `release-manager-agent`.
4. **Elicitation step**: run the GitHub Agent with no prior consent. Confirm the answer surfaces a clickable URL (Task A7's open TODO - adjust `github_agent/main.py`'s pending-detection logic here if the real wire shape differs from the placeholder assumption). Complete consent in the Solo UI, retry, confirm real GitHub data comes back. If GitHub's MCP server 404s on the first attempt, apply Task B5's documented trailing-slash workaround to `MCP_GITHUB_URL` and redeploy.

- [ ] **Step 6: Self-review against the design spec**

Re-read `docs/superpowers/specs/2026-10-10-auth-pillar-agents-mcp-design.md` and confirm every locked decision is honored: Standard = proxy-native always (✓ Task B3); Delegation = Solo STS, never Keycloak-native (✓ Task B2/B3); Flow 2 = real per-hop re-exchange, nested `act` (✓ Task A6); repo `github.com/day0ops/ops-agents` (✓ Task A1); 4 agents + 1 new MCP server (✓ Tasks A4-A7, A2); GitHub OAuth app is a placeholder (✓ Task B5); `fd-loan-rbac-jwt-propagation` untouched (✓ - no task in this plan modifies it).

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-10-11-auth-pillar-agents-mcp.md`. Two execution options:

1. **Subagent-Driven (recommended)** - dispatch a fresh subagent per task, review between tasks, fast iteration.
2. **Inline Execution** - execute tasks in this session using executing-plans, batch execution with checkpoints.

Which approach?
