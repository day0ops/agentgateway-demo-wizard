# Auth pillar rework: real login + token exchange

## Problem

The "Who's allowed to do what" pillar's two existing steps (`auth-jwt`, `auth-rbac`) demonstrate JWT validation and CEL-based RBAC, but the JWT they validate is minted by the wizard itself via an OAuth2 Resource Owner Password Credentials grant (`internal/gateway/client.go`'s `Token()` - the wizard POSTs `grant_type=password` plus the demo user's username and the shared realm password directly to Keycloak's token endpoint). ROPC requires a client to handle a real user's password directly and is a deprecated anti-pattern under current OAuth guidance. Worse, the step's own sequence diagram narrates "Client->>KC: password grant" as part of the lesson - a security-education demo is showing the insecure pattern as if it were normal.

Separately, neither step demonstrates token exchange at all, despite agentgateway supporting two distinct, real patterns for it (linked by the user): standard RFC 8693 exchange (proxy-native, no extra infrastructure) and STS-based exchange (a legacy, still-supported controller-side component used for delegation with an `act` claim). The pillar should show both, clearly labeled.

## Scope

Reworks the "auth" pillar (`internal/scenarios/registry.go`'s `authPillar()`) into four steps:

1. **Real login** - replaces ROPC with a real Authorization Code + PKCE browser login against Keycloak.
2. **RBAC** - unchanged CEL authorization, now evaluated against the real, interactively-obtained token.
3. **Standard token exchange (Impersonation)** - proxy-native RFC 8693 exchange, explicitly labeled as the modern/recommended path.
4. **STS token exchange (Delegation)** - legacy controller-side STS, explicitly labeled as such, producing a token with both `sub` (the user) and `act` (the wizard, as the delegating agent) claims.

**Out of scope:** every other pillar (routing, guardrails, cost, agent) keeps using the existing ROPC-based `Token()` for its own identity-switching convenience - that mechanism isn't being removed from `gateway/client.go`, only stopped-using and stopped-showing within this one pillar. `agw-client-public`'s `directAccessGrantsEnabled` in Keycloak stays on for that reason.

## Why standard vs. STS aren't interchangeable

Both are real, currently-working patterns in `agentgateway-field-kit`, reused rather than invented:

| | Standard (Impersonation) | STS (Delegation) |
|---|---|---|
| Mechanism | `backend.auth.oauthTokenExchange` on an `EnterpriseAgentgatewayPolicy`, calling Keycloak's own token endpoint (RFC 8693) | A separate Solo STS controller component (Helm `tokenExchange.*` values, own Postgres), called directly by the agent with `subject_token` + `actor_token` |
| `act` claim | No - re-signs the same identity | Yes - audits both the user and the delegating agent |
| New infra needed | None - reuses the `agw-token-exchange` Keycloak client already provisioned and already used by this wizard's agent pillar (`agent-token-exchange.yaml.tmpl`) | Yes - Postgres deployment, STS Helm toggle, subject/actor/api validators, all new to this environment |
| field-kit's own characterization | "the proxy-native way... prefer ... instead" | "LEGACY (still supported, not recommended for new work)" |

Labeling both clearly in the demo copy (not just internally) is part of the ask - each step's explanation names its mechanism and says which one agentgateway recommends for new work.

## Step 1: Real login

### Flow

1. Frontend: a new "Log in as team-alpha" / "Log in as team-beta" action does a full browser navigation (not `fetch`) to `GET /auth/login?identity=team-alpha&return_to=auth` on the wizard's own backend - not agentgateway's own OAuth-authorization-code extauth feature, which would terminate the session at the gateway and leave the wizard with no knowledge of the token (the gap flagged during design).
2. Wizard backend generates a PKCE `code_verifier`/`code_challenge` (S256) and an opaque `state`, stores `{state: {identity, return_to, verifier, createdAt}}` in an in-memory map (single-process demo app - no new datastore), and 302s the browser to Keycloak's `/realms/agw-dev/protocol/openid-connect/auth` with `client_id=agw-client-public` (already a public, authorization-code-enabled client with `redirectUris: ['*']` - confirmed in the field-kit's Keycloak addon, so no field-kit change needed here), `redirect_uri` built from the incoming request's own `Host` header, `response_type=code`, `scope=openid`, `state`, `code_challenge`, `code_challenge_method=S256`.
3. Presenter logs in on Keycloak's real hosted login page with the existing demo user (e.g. `user1`/the realm's shared password) - no change to how demo users are provisioned.
4. Keycloak redirects back to `GET /auth/callback?code=...&state=...` **on the wizard**, not the gateway.
5. Wizard backend looks up `state`, exchanges `code` + `code_verifier` directly against Keycloak's token endpoint (standard Authorization Code grant, public client, PKCE instead of a secret), and now holds a real access token server-side.
6. Wizard issues its own opaque session cookie (`wizard_session`, httpOnly, Secure, SameSite=Lax) if the browser doesn't already have one, and caches the token in-memory keyed by `(sessionID, identity)`, with the token's own `expires_in`.
7. Redirects the browser to `/?returnTo=auth` (see Frontend changes) so the presenter lands back on the auth pillar instead of the welcome screen.

### Backend changes (`agentgateway-demo-wizard`)

- New package or file, e.g. `internal/oauthlogin/`, holding: the in-memory pending-flow map (state -> identity/verifier/return_to, TTL-expired), the in-memory captured-token store (sessionID+identity -> token+expiry), and the two HTTP handlers (`/auth/login`, `/auth/callback`) registered in `server.go`.
- `gateway.Client` gains a way to proxy a request using an explicitly-supplied bearer token, bypassing `Token()` entirely, for presets that require the real captured token.
- `request_handler.go`: presets that require a real session token carry a new explicit flag (see below); when set, the handler reads the `wizard_session` cookie, looks up the captured token for `(sessionID, identity)`, and uses it directly. No token found -> a clear error ("log in as team-alpha first"), not a silent fallback to ROPC - falling back would quietly undercut the lesson this pillar exists to teach.
- `scenarios.RequestPreset` gains a `RequiresSessionToken bool` field (name open to refinement), set `true` only for this pillar's presets (steps 2-4), threaded through the JSON API to the frontend and back.

### Frontend changes

- `DriveRequestPane` (or a small new component used only by this pillar) gains a real navigation-triggering login action per identity, instead of (or alongside) the existing instant preset buttons.
- `App.tsx` reads a `returnTo` query param once on mount (`useEffect`, `URLSearchParams`) and, if it names a pillar id present in the loaded registry, sets `activePillarIndex`/`activeStepIndex` to land there instead of defaulting to the welcome screen - the only other pillars affected are none; this is additive and only triggers when the param is present.
- Some visible "logged in as team-alpha (real token)" / "not logged in" state on the auth pillar, likely via a small status check endpoint (e.g. `GET /api/auth/status?identity=team-alpha`) the SPA calls on mount/after redirect, since the session cookie itself is httpOnly and unreadable from JS by design.

## Step 2: RBAC

No manifest or policy change - `auth/rbac.yaml.tmpl`'s CEL authorization stays as-is. The only change is that its presets now carry `RequiresSessionToken: true` and the explanation/diagram text drops any remaining ROPC narrative, since by this step the real token from step 1 is already in hand.

## Step 3: Standard token exchange (Impersonation)

New manifest template modeled directly on `agent/token-exchange.yaml.tmpl` (already working in this repo): a new echo backend (or reuse `auth-rbac-echo`'s shape) fronted by an `EnterpriseAgentgatewayPolicy` that both validates the JWT (as today) **and** sets `backend.auth.oauthTokenExchange` pointing at the same `agw-token-exchange` Keycloak client already provisioned, so the gateway exchanges the real token for a backend-scoped one (different `aud`/`azp`, same `sub`) before forwarding. New registry step with its own diagram explicitly labeled "Standard token exchange (Impersonation) - proxy-native, no controller component" and a preset requiring the session token from step 1.

## Step 4: STS token exchange (Delegation)

The heaviest piece, requiring real new infrastructure in `agentgateway-field-kit`:

- Deploy Postgres and enable the `tokenExchange.*` Helm values (the STS controller) - field-kit's `features/token-exchange` and `features/postgres` already do this for other usecases (`fd-loan-rbac-jwt-propagation`, `workload-identity-chain`); reuse that feature rather than hand-rolling new Helm wiring.
- Configure the STS's `subjectValidator` (validates the real user JWT from step 1 via Keycloak's JWKS, same issuer as today), `actorValidator` (validates a Kubernetes ServiceAccount token), and `apiValidator` (required even if unused).
- The wizard's own Pod identity becomes the delegating agent: its Go backend reads its own projected ServiceAccount token (the standard `/var/run/secrets/kubernetes.io/serviceaccount/token` every Pod gets) as the `actor_token`, and calls the STS's token endpoint directly with `subject_token` = the real captured user JWT from step 1 + `actor_token`, getting back a token carrying `sub=<user>` and `act=<wizard-service-account>`.
- New wizard manifest template: an `obo-token-exchange`-style policy (JWT auth whose issuer is the STS, not Keycloak directly) protecting a new echo backend.
- New registry step, diagram, and explanation explicitly labeled "STS token exchange (Delegation) - legacy controller-side STS, kept for audit trails that need to show both the user and the acting agent." A preset requiring the step-1 session token, driving the wizard's own STS call server-side (the preset itself doesn't carry a raw bearer token the way others do - the handler performs the STS exchange using the cached session token as `subject_token` before forwarding).

This step's exact Helm values and validator config will be confirmed against field-kit's working `workload-identity-chain`/`agent-workload-identity` references at implementation time rather than guessed here.

## Testing

- Go: unit tests for the PKCE state store (TTL expiry, one-time use), the callback handler (code exchange success/failure), and `request_handler.go`'s "missing session token -> clear error" path, following this repo's existing `httptest`-based handler test conventions.
- Frontend: a test for the `returnTo` deep-link restore in `App.tsx`, and for the login-status check rendering.
- Manual/live verification: the actual browser redirect dance against a real Keycloak (not mockable in `vitest`/Go unit tests) - documented as a manual verification step, consistent with how this repo has handled other live-infrastructure-dependent changes.

## Risks / open items

- The in-memory session/token store means a wizard process restart logs everyone out - acceptable for a single-process demo tool, consistent with how `k8s.Applier` already tracks applied state in-memory.
- Step 4's exact STS Helm values and validator wiring are confirmed against field-kit's existing reference usecases during implementation, not fully specified here.
- The field-kit side of steps 3 (none needed - reuses existing client) and 4 (Postgres + STS Helm toggle + validators) is out of this repo's git history; those changes land in `agentgateway-field-kit`'s own working tree the same way prior cross-repo fixes in this project have.
