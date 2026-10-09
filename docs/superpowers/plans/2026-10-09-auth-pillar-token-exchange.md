# Auth Pillar Token Exchange Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the "auth" pillar's password-grant-based JWT demo with a real Authorization Code + PKCE login, and add two new steps demonstrating agentgateway's standard (proxy-native) and STS (legacy, delegation) token exchange patterns.

**Architecture:** A new `internal/oauthlogin` package holds PKCE/state generation, an in-memory pending-flow + captured-token store, and the two outbound token-exchange calls (Keycloak Authorization Code, Solo STS). New HTTP handlers (`/auth/login`, `/auth/callback`, `/api/auth/status`) perform the real login using the wizard itself as the OAuth client (`agw-client-public`, already public + authorization-code-enabled with a wildcard redirect URI). `request_handler.go` gains two preset flags (`RequiresSessionToken`, `ExchangeViaSTS`) that make it use the captured real token - optionally STS-exchanged first - instead of the existing ROPC path, which stays untouched for every other pillar.

**Tech Stack:** Go (stdlib `net/http`, `crypto/rand`, `crypto/sha256`), React/TypeScript, Vitest, `httptest`.

## Global Constraints

- Every other pillar keeps using the existing ROPC-based `gateway.Client.Token()` - it is not removed, only stopped-using within the auth pillar. `agw-client-public`'s `directAccessGrantsEnabled` in Keycloak stays on.
- `agw-client-public` already has `redirectUris: ['*']` in the field-kit's Keycloak addon - no field-kit change needed for the login flow itself.
- The STS host/port default is `enterprise-agentgateway.agentgateway-system.svc.cluster.local:7777`, confirmed against `agentgateway-field-kit`'s `config/environments/aws-dev.yaml` (`spec.internal.stsIssuer`).
- No silent fallback to ROPC when a required real token is missing - return a clear "log in first" error instead; falling back would undercut the pillar's own lesson.
- Follow this repo's existing conventions throughout: functional `Option`s on `Server`, `httptest`-based handler tests, table-free straightforward `t.Fatalf` assertions, manifest templates rendered via `internal/manifests.Render`.

---

### Task 1: `internal/oauthlogin` core - PKCE, state, in-memory store

**Files:**
- Create: `internal/oauthlogin/oauthlogin.go`
- Test: `internal/oauthlogin/oauthlogin_test.go`

**Interfaces:**
- Produces: `oauthlogin.SessionCookieName` (string const), `oauthlogin.Flow{Identity, ReturnTo, Verifier string}`, `oauthlogin.Token{AccessToken string, ExpiresAt time.Time}`, `oauthlogin.NewStore() *Store`, `(*Store).SaveFlow(state string, flow Flow)`, `(*Store).TakeFlow(state string) (Flow, bool)`, `(*Store).PutToken(sessionID, identity string, tok Token)`, `(*Store).GetToken(sessionID, identity string) (Token, bool)`, `oauthlogin.NewSessionID() (string, error)`, `oauthlogin.GeneratePKCE() (verifier, challenge string, err error)`, `oauthlogin.GenerateState() (string, error)`.

- [ ] **Step 1: Write the failing test**

Create `internal/oauthlogin/oauthlogin_test.go`:

```go
package oauthlogin

import (
	"testing"
	"time"
)

func TestGeneratePKCEProducesVerifierAndMatchingChallenge(t *testing.T) {
	verifier, challenge, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE: %v", err)
	}
	if verifier == "" || challenge == "" {
		t.Fatalf("expected non-empty verifier and challenge, got %q and %q", verifier, challenge)
	}
	if verifier == challenge {
		t.Fatalf("expected the challenge to be derived from the verifier, not equal to it")
	}

	_, challenge2, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE: %v", err)
	}
	if challenge == challenge2 {
		t.Fatalf("expected two calls to produce different values")
	}
}

func TestGenerateStateProducesDistinctValues(t *testing.T) {
	a, err := GenerateState()
	if err != nil {
		t.Fatalf("GenerateState: %v", err)
	}
	b, err := GenerateState()
	if err != nil {
		t.Fatalf("GenerateState: %v", err)
	}
	if a == b {
		t.Fatalf("expected two calls to produce different values")
	}
}

func TestSaveFlowThenTakeFlowRoundTrips(t *testing.T) {
	store := NewStore()
	store.SaveFlow("state-1", Flow{Identity: "team-alpha", ReturnTo: "auth", Verifier: "verifier-1"})

	flow, ok := store.TakeFlow("state-1")
	if !ok {
		t.Fatalf("expected TakeFlow to find the saved flow")
	}
	if flow.Identity != "team-alpha" || flow.ReturnTo != "auth" || flow.Verifier != "verifier-1" {
		t.Fatalf("unexpected flow: %+v", flow)
	}
}

func TestTakeFlowIsOneTimeUse(t *testing.T) {
	store := NewStore()
	store.SaveFlow("state-1", Flow{Identity: "team-alpha"})

	if _, ok := store.TakeFlow("state-1"); !ok {
		t.Fatalf("expected the first TakeFlow to succeed")
	}
	if _, ok := store.TakeFlow("state-1"); ok {
		t.Fatalf("expected the second TakeFlow to fail - state should be consumed")
	}
}

func TestTakeFlowUnknownStateFails(t *testing.T) {
	store := NewStore()
	if _, ok := store.TakeFlow("never-saved"); ok {
		t.Fatalf("expected an unknown state to fail")
	}
}

func TestPutTokenThenGetTokenRoundTrips(t *testing.T) {
	store := NewStore()
	store.PutToken("session-1", "team-alpha", Token{AccessToken: "tok-1", ExpiresAt: time.Now().Add(time.Hour)})

	tok, ok := store.GetToken("session-1", "team-alpha")
	if !ok {
		t.Fatalf("expected GetToken to find the stored token")
	}
	if tok.AccessToken != "tok-1" {
		t.Fatalf("expected tok-1, got %q", tok.AccessToken)
	}
}

func TestGetTokenMissingIdentityFails(t *testing.T) {
	store := NewStore()
	store.PutToken("session-1", "team-alpha", Token{AccessToken: "tok-1", ExpiresAt: time.Now().Add(time.Hour)})

	if _, ok := store.GetToken("session-1", "team-beta"); ok {
		t.Fatalf("expected a different identity in the same session to find nothing")
	}
	if _, ok := store.GetToken("session-2", "team-alpha"); ok {
		t.Fatalf("expected a different session to find nothing")
	}
}

func TestGetTokenExpiredFails(t *testing.T) {
	store := NewStore()
	store.PutToken("session-1", "team-alpha", Token{AccessToken: "tok-1", ExpiresAt: time.Now().Add(-time.Minute)})

	if _, ok := store.GetToken("session-1", "team-alpha"); ok {
		t.Fatalf("expected an expired token to be treated as missing")
	}
}

func TestNewSessionIDProducesDistinctValues(t *testing.T) {
	a, err := NewSessionID()
	if err != nil {
		t.Fatalf("NewSessionID: %v", err)
	}
	b, err := NewSessionID()
	if err != nil {
		t.Fatalf("NewSessionID: %v", err)
	}
	if a == b {
		t.Fatalf("expected two calls to produce different values")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/oauthlogin/... -v`
Expected: FAIL with "no required module provides package" / "undefined: GeneratePKCE" (package doesn't exist yet)

- [ ] **Step 3: Write the implementation**

Create `internal/oauthlogin/oauthlogin.go`:

```go
// Package oauthlogin implements a real Authorization Code + PKCE login
// against Keycloak, with the wizard itself as the OAuth client - replacing
// the Resource Owner Password Credentials shortcut gateway.Client.Token
// uses for every other pillar. It also performs the two downstream
// exchanges the auth pillar demonstrates: the standard RFC 8693 exchange
// happens inside agentgateway itself (backend.auth.oauthTokenExchange, no
// code here), but the STS Delegation exchange is a call the wizard's own
// backend makes directly - see sts.go.
package oauthlogin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

// SessionCookieName is the wizard's own session cookie, set after a
// successful Authorization Code callback - distinct from any cookie
// agentgateway or Keycloak might set, since the wizard is a separate OAuth
// client in this flow.
const SessionCookieName = "wizard_session"

// flowTTL bounds how long a pending login (state issued, callback not yet
// received) stays valid - long enough for a presenter to type a password
// into Keycloak's real login page, short enough that a stale state can't
// be replayed.
const flowTTL = 10 * time.Minute

// Flow is the state the wizard stashes between redirecting to Keycloak and
// receiving the callback - everything handleAuthCallback needs that can't
// round-trip through the browser as anything other than the opaque
// `state` value.
type Flow struct {
	Identity string
	ReturnTo string
	Verifier string
	expires  time.Time
}

// Token is a captured, real access token for one identity within one
// wizard session - the result of a completed Authorization Code exchange.
type Token struct {
	AccessToken string
	ExpiresAt   time.Time
}

// Store holds in-memory login state for the lifetime of the wizard
// process - a process restart logs every browser out, which is acceptable
// for a single-instance demo tool (the same tradeoff internal/k8s.Applier
// already makes for tracking applied policies).
type Store struct {
	mu     sync.Mutex
	flows  map[string]Flow
	tokens map[string]map[string]Token
}

// NewStore builds an empty Store.
func NewStore() *Store {
	return &Store{
		flows:  make(map[string]Flow),
		tokens: make(map[string]map[string]Token),
	}
}

// SaveFlow stashes a pending login under state, to be retrieved once by
// TakeFlow when Keycloak calls back.
func (s *Store) SaveFlow(state string, flow Flow) {
	flow.expires = time.Now().Add(flowTTL)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flows[state] = flow
}

// TakeFlow retrieves and deletes the pending login for state - one-time
// use, so a replayed callback can't reuse it. Returns false if state is
// unknown or its flow has expired.
func (s *Store) TakeFlow(state string) (Flow, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	flow, ok := s.flows[state]
	if !ok {
		return Flow{}, false
	}
	delete(s.flows, state)
	if time.Now().After(flow.expires) {
		return Flow{}, false
	}
	return flow, true
}

// PutToken records a captured token for identity within sessionID.
func (s *Store) PutToken(sessionID, identity string, tok Token) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tokens[sessionID] == nil {
		s.tokens[sessionID] = make(map[string]Token)
	}
	s.tokens[sessionID][identity] = tok
}

// GetToken returns the captured token for identity within sessionID, if
// one exists and hasn't expired.
func (s *Store) GetToken(sessionID, identity string) (Token, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	byIdentity, ok := s.tokens[sessionID]
	if !ok {
		return Token{}, false
	}
	tok, ok := byIdentity[identity]
	if !ok || time.Now().After(tok.ExpiresAt) {
		return Token{}, false
	}
	return tok, true
}

// NewSessionID returns a fresh opaque session identifier for the wizard's
// own session cookie.
func NewSessionID() (string, error) {
	return randomURLSafeString(32)
}

// GeneratePKCE returns a fresh RFC 7636 code_verifier and its S256
// code_challenge, for a public OAuth client that can't hold a secret.
func GeneratePKCE() (verifier, challenge string, err error) {
	verifier, err = randomURLSafeString(32)
	if err != nil {
		return "", "", fmt.Errorf("generating code verifier: %w", err)
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

// GenerateState returns a fresh opaque anti-CSRF state value.
func GenerateState() (string, error) {
	return randomURLSafeString(16)
}

func randomURLSafeString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/oauthlogin/... -v`
Expected: PASS (9 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/oauthlogin/oauthlogin.go internal/oauthlogin/oauthlogin_test.go
git commit -m "feat(oauthlogin): add PKCE, state, and in-memory login store"
```

---

### Task 2: `internal/oauthlogin` - Keycloak Authorization Code exchange

**Files:**
- Create: `internal/oauthlogin/keycloak.go`
- Test: `internal/oauthlogin/keycloak_test.go`

**Interfaces:**
- Consumes: `oauthlogin.Token` (Task 1)
- Produces: `oauthlogin.ExchangeCode(ctx context.Context, httpClient *http.Client, keycloakBaseURL, clientID, redirectURI, code, verifier string) (Token, error)`

- [ ] **Step 1: Write the failing test**

Create `internal/oauthlogin/keycloak_test.go`:

```go
package oauthlogin

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExchangeCodePostsExpectedFormAndParsesToken(t *testing.T) {
	var gotBody string
	keycloak := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/realms/agw-dev/protocol/openid-connect/token" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"real-token","expires_in":300}`))
	}))
	defer keycloak.Close()

	tok, err := ExchangeCode(context.Background(), keycloak.Client(), keycloak.URL, "agw-client-public", "https://wizard.example.com/auth/callback", "auth-code-1", "verifier-1")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if tok.AccessToken != "real-token" {
		t.Fatalf("expected real-token, got %q", tok.AccessToken)
	}
	if time.Until(tok.ExpiresAt) <= 0 || time.Until(tok.ExpiresAt) > 300*time.Second {
		t.Fatalf("expected ExpiresAt roughly 300s out, got %v", tok.ExpiresAt)
	}
	for _, want := range []string{"grant_type=authorization_code", "client_id=agw-client-public", "code=auth-code-1", "code_verifier=verifier-1"} {
		if !strings.Contains(gotBody, want) {
			t.Fatalf("expected form body to contain %q, got: %s", want, gotBody)
		}
	}
}

func TestExchangeCodeSurfacesNonOKStatus(t *testing.T) {
	keycloak := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer keycloak.Close()

	_, err := ExchangeCode(context.Background(), keycloak.Client(), keycloak.URL, "agw-client-public", "https://wizard.example.com/auth/callback", "bad-code", "verifier-1")
	if err == nil {
		t.Fatalf("expected an error for a non-200 response")
	}
	if !strings.Contains(err.Error(), "invalid_grant") {
		t.Fatalf("expected the error to include the raw response body, got: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/oauthlogin/... -run TestExchangeCode -v`
Expected: FAIL with "undefined: ExchangeCode"

- [ ] **Step 3: Write the implementation**

Create `internal/oauthlogin/keycloak.go`:

```go
package oauthlogin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ExchangeCode completes a standard Authorization Code + PKCE exchange
// against Keycloak's token endpoint - the wizard acting as its own OAuth
// client (agw-client-public, already public + authorization-code-enabled
// with a wildcard redirect URI in the agw-dev realm), rather than relying
// on agentgateway's own OAuth-authorization-code extauth feature, which
// would leave the resulting token known only to the gateway's session
// cookie and never to the wizard itself.
func ExchangeCode(ctx context.Context, httpClient *http.Client, keycloakBaseURL, clientID, redirectURI, code, verifier string) (Token, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}

	endpoint := strings.TrimRight(keycloakBaseURL, "/") + "/realms/agw-dev/protocol/openid-connect/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, fmt.Errorf("building code exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return Token{}, fmt.Errorf("exchanging code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Token{}, fmt.Errorf("reading code exchange response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return Token{}, fmt.Errorf("keycloak code exchange failed with status %d: %s", resp.StatusCode, body)
	}

	var parsed struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Token{}, fmt.Errorf("parsing code exchange response: %w", err)
	}
	return Token{
		AccessToken: parsed.AccessToken,
		ExpiresAt:   time.Now().Add(time.Duration(parsed.ExpiresIn) * time.Second),
	}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/oauthlogin/... -v`
Expected: PASS (11 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/oauthlogin/keycloak.go internal/oauthlogin/keycloak_test.go
git commit -m "feat(oauthlogin): add Keycloak Authorization Code exchange"
```

---

### Task 3: `internal/oauthlogin` - STS Delegation exchange + actor token reader

**Files:**
- Create: `internal/oauthlogin/sts.go`
- Test: `internal/oauthlogin/sts_test.go`

**Interfaces:**
- Produces: `oauthlogin.DefaultActorTokenPath` (string const), `oauthlogin.ReadActorToken(path string) (string, error)`, `oauthlogin.ExchangeViaSTS(ctx context.Context, httpClient *http.Client, stsHost, subjectToken, actorToken string) (string, error)`

- [ ] **Step 1: Write the failing test**

Create `internal/oauthlogin/sts_test.go`:

```go
package oauthlogin

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadActorTokenTrimsWhitespace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("sa-token-1\n"), 0o600); err != nil {
		t.Fatalf("writing fixture token: %v", err)
	}

	tok, err := ReadActorToken(path)
	if err != nil {
		t.Fatalf("ReadActorToken: %v", err)
	}
	if tok != "sa-token-1" {
		t.Fatalf("expected sa-token-1, got %q", tok)
	}
}

func TestReadActorTokenMissingFileErrors(t *testing.T) {
	_, err := ReadActorToken(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatalf("expected an error for a missing file")
	}
}

func TestExchangeViaSTSPostsExpectedFormAndParsesToken(t *testing.T) {
	var gotBody string
	sts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth2/token" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"sts-exchanged-token"}`))
	}))
	defer sts.Close()

	host := strings.TrimPrefix(sts.URL, "http://")
	token, err := ExchangeViaSTS(context.Background(), sts.Client(), host, "user-subject-token", "wizard-actor-token")
	if err != nil {
		t.Fatalf("ExchangeViaSTS: %v", err)
	}
	if token != "sts-exchanged-token" {
		t.Fatalf("expected sts-exchanged-token, got %q", token)
	}
	for _, want := range []string{
		"grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Atoken-exchange",
		"subject_token=user-subject-token",
		"actor_token=wizard-actor-token",
	} {
		if !strings.Contains(gotBody, want) {
			t.Fatalf("expected form body to contain %q, got: %s", want, gotBody)
		}
	}
}

func TestExchangeViaSTSSurfacesNonOKStatus(t *testing.T) {
	sts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"invalid_actor_token"}`))
	}))
	defer sts.Close()

	host := strings.TrimPrefix(sts.URL, "http://")
	_, err := ExchangeViaSTS(context.Background(), sts.Client(), host, "subject", "actor")
	if err == nil {
		t.Fatalf("expected an error for a non-200 response")
	}
	if !strings.Contains(err.Error(), "invalid_actor_token") {
		t.Fatalf("expected the error to include the raw response body, got: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/oauthlogin/... -run 'TestReadActorToken|TestExchangeViaSTS' -v`
Expected: FAIL with "undefined: ReadActorToken" / "undefined: ExchangeViaSTS"

- [ ] **Step 3: Write the implementation**

Create `internal/oauthlogin/sts.go`:

```go
package oauthlogin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// DefaultActorTokenPath is where Kubernetes projects this Pod's own
// ServiceAccount token - the wizard's identity when it acts as the
// delegating agent in the STS Delegation flow (see ExchangeViaSTS).
const DefaultActorTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"

// ReadActorToken reads the wizard's own actor token (its Kubernetes
// ServiceAccount token) from path.
func ReadActorToken(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading actor token from %q: %w", path, err)
	}
	return strings.TrimSpace(string(b)), nil
}

// ExchangeViaSTS performs a Delegation token exchange against Solo's
// legacy controller-side Security Token Service: subjectToken (the real
// user token captured in the login step) plus actorToken (the wizard's own
// identity) become one token carrying both `sub` (the user) and `act` (the
// wizard), for downstream delegation auditing. Contrast with the Standard
// (Impersonation) exchange, which agentgateway performs itself via
// backend.auth.oauthTokenExchange and needs no call like this one.
func ExchangeViaSTS(ctx context.Context, httpClient *http.Client, stsHost, subjectToken, actorToken string) (string, error) {
	form := url.Values{
		"grant_type":         {"urn:ietf:params:oauth:grant-type:token-exchange"},
		"subject_token":      {subjectToken},
		"subject_token_type": {"urn:ietf:params:oauth:token-type:access_token"},
		"actor_token":        {actorToken},
		"actor_token_type":   {"urn:ietf:params:oauth:token-type:access_token"},
		"audience":           {"agentgateway"},
	}

	endpoint := "http://" + strings.TrimRight(stsHost, "/") + "/oauth2/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("building STS exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling STS: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading STS response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("STS exchange failed with status %d: %s", resp.StatusCode, body)
	}

	var parsed struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parsing STS response: %w", err)
	}
	return parsed.AccessToken, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/oauthlogin/... -v`
Expected: PASS (15 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/oauthlogin/sts.go internal/oauthlogin/sts_test.go
git commit -m "feat(oauthlogin): add STS Delegation exchange and actor token reader"
```

---

### Task 4: `config.Config` - add `STSHost`

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces: `config.Config.STSHost string` (new field, not part of `Missing()`)

- [ ] **Step 1: Write the failing test**

Append to `internal/config/config_test.go`:

```go
func TestLoadDefaultsSTSHostWhenUnset(t *testing.T) {
	t.Setenv("STS_HOST", "")

	cfg := Load()

	if cfg.STSHost != defaultSTSHost {
		t.Fatalf("expected the default STS host, got %q", cfg.STSHost)
	}
}

func TestLoadRespectsSTSHostOverride(t *testing.T) {
	t.Setenv("STS_HOST", "sts.other-namespace.svc.cluster.local:7777")

	cfg := Load()

	if cfg.STSHost != "sts.other-namespace.svc.cluster.local:7777" {
		t.Fatalf("expected the overridden STS host, got %q", cfg.STSHost)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -run TestLoad.*STS -v`
Expected: FAIL with "undefined: defaultSTSHost" / `cfg.STSHost` compile error

- [ ] **Step 3: Write the implementation**

In `internal/config/config.go`, modify the const block:

```go
const (
	defaultStockMCPImage    = "australia-southeast1-docker.pkg.dev/field-engineering-apac/kasunt/stock-server-mcp:0.1.1"
	defaultCurrencyMCPImage = "australia-southeast1-docker.pkg.dev/field-engineering-apac/kasunt/currency-server-mcp:0.1.1"
	// defaultSTSHost is Solo's enterprise-agentgateway Helm chart's own
	// in-cluster STS service name and port, confirmed against
	// agentgateway-field-kit's config/environments/aws-dev.yaml
	// (spec.internal.stsIssuer) - internal-only, never a public demo
	// hostname like GatewayHost/KeycloakHost, so it only needs a default,
	// never a "missing" check.
	defaultSTSHost = "enterprise-agentgateway.agentgateway-system.svc.cluster.local:7777"
)
```

Modify the `Config` struct:

```go
type Config struct {
	GatewayHost                    string
	KeycloakHost                   string
	OpenAIKey                      string
	StockMCPImage                  string
	CurrencyMCPImage               string
	OAuthTokenExchangeClientSecret string
	DemoUserPassword               string
	STSHost                        string
}
```

Modify `Load()`:

```go
func Load() *Config {
	return &Config{
		GatewayHost:                    os.Getenv("AGW_HOST"),
		KeycloakHost:                   os.Getenv("KEYCLOAK_HOST"),
		OpenAIKey:                      os.Getenv("OPENAI_API_KEY"),
		StockMCPImage:                  envOrDefault("STOCK_SERVER_MCP_IMAGE", defaultStockMCPImage),
		CurrencyMCPImage:               envOrDefault("CURRENCY_SERVER_MCP_IMAGE", defaultCurrencyMCPImage),
		OAuthTokenExchangeClientSecret: os.Getenv("OAUTH_TOKEN_EXCHANGE_CLIENT_SECRET"),
		DemoUserPassword:               os.Getenv("DEMO_USER_PASSWORD"),
		STSHost:                        envOrDefault("STS_HOST", defaultSTSHost),
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -v`
Expected: PASS (all config tests, including the 2 new ones)

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add STSHost for the STS Delegation exchange"
```

---

### Task 5: `gateway.Client` - add `ProxyWithToken`

**Files:**
- Modify: `internal/gateway/client.go`
- Test: `internal/gateway/client_test.go`

**Interfaces:**
- Produces: `(*gateway.Client).ProxyWithToken(ctx context.Context, req Request, token string) (*Response, error)`

- [ ] **Step 1: Write the failing test**

Append to `internal/gateway/client_test.go`:

```go
func TestProxyWithTokenUsesSuppliedTokenWithoutCallingKeycloak(t *testing.T) {
	keycloakCalled := false
	keycloak := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keycloakCalled = true
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "ropc-token"})
	}))
	defer keycloak.Close()

	var gotAuth string
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer gatewayServer.Close()

	client := NewClient(keycloak.URL, gatewayServer.URL, "test-password")

	resp, err := client.ProxyWithToken(context.Background(), Request{Identity: "team-alpha", Path: "/secure/exchange-standard", Body: "{}"}, "real-captured-token")
	if err != nil {
		t.Fatalf("ProxyWithToken: %v", err)
	}
	if keycloakCalled {
		t.Fatalf("expected ProxyWithToken to never call Keycloak's token endpoint")
	}
	if gotAuth != "Bearer real-captured-token" {
		t.Fatalf("expected the supplied token to be forwarded as-is, got %q", gotAuth)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestProxyWithTokenEmptyTokenOmitsAuthorizationHeader(t *testing.T) {
	var gotAuth string
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer gatewayServer.Close()

	client := NewClient("http://unused", gatewayServer.URL, "test-password")

	if _, err := client.ProxyWithToken(context.Background(), Request{Path: "/secure/exchange-standard", Body: "{}"}, ""); err != nil {
		t.Fatalf("ProxyWithToken: %v", err)
	}
	if gotAuth != "" {
		t.Fatalf("expected no Authorization header for an empty token, got %q", gotAuth)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gateway/... -run TestProxyWithToken -v`
Expected: FAIL with "client.ProxyWithToken undefined"

- [ ] **Step 3: Write the implementation**

In `internal/gateway/client.go`, replace the `Proxy` function body with a thin wrapper and extract the shared logic into the new exported `ProxyWithToken`:

```go
// Proxy acquires a token for req.Identity (unless it is "" or "anonymous")
// and forwards req to agentgateway, returning the raw response.
func (c *Client) Proxy(ctx context.Context, req Request) (*Response, error) {
	var token string
	if req.Identity != "" && req.Identity != "anonymous" {
		t, err := c.Token(ctx, req.Identity)
		if err != nil {
			return nil, fmt.Errorf("acquiring token for %q: %w", req.Identity, err)
		}
		token = t
	}
	return c.ProxyWithToken(ctx, req, token)
}

// ProxyWithToken forwards req to agentgateway using token directly as the
// bearer credential, skipping Token() entirely - for callers that already
// hold a real, interactively-obtained token (see internal/oauthlogin)
// rather than one of this client's own fixed demo identities. An empty
// token forwards the request with no Authorization header, same as Proxy
// does for "anonymous".
func (c *Client) ProxyWithToken(ctx context.Context, req Request, token string) (*Response, error) {
	method := req.Method
	if method == "" {
		method = http.MethodPost
	}

	endpoint := c.gatewayBaseURL + req.Path
	httpReq, err := http.NewRequestWithContext(ctx, method, endpoint, strings.NewReader(req.Body))
	if err != nil {
		return nil, fmt.Errorf("building gateway request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	if token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("calling gateway: %w", err)
	}
	defer resp.Body.Close()
	latency := time.Since(start)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading gateway response: %w", err)
	}

	return &Response{StatusCode: resp.StatusCode, Body: string(body), LatencyMS: latency.Milliseconds()}, nil
}
```

This removes the old body of `Proxy` (the method/endpoint/httpReq/token-header/do/read block) - everything after the `token` variable is resolved now lives only in `ProxyWithToken`. `ProxyStream` is untouched; it already builds its own request independently and isn't part of this refactor.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gateway/... -v`
Expected: PASS (all gateway tests, including the 2 new ones)

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/client.go internal/gateway/client_test.go
git commit -m "refactor(gateway): extract ProxyWithToken from Proxy"
```

---

### Task 6: `scenarios` and `manifests` types - new preset/step/param fields

**Files:**
- Modify: `internal/manifests/render.go`
- Modify: `internal/scenarios/types.go`

**Interfaces:**
- Produces: `manifests.Params.STSHost string` (and transitively `scenarios.ManifestParams.STSHost`, since that's a type alias), `scenarios.RequestPreset.RequiresSessionToken bool`, `scenarios.RequestPreset.ExchangeViaSTS bool`, `scenarios.Step.LoginDemo bool`

This task has no new behavior of its own to test directly - it's struct-field plumbing consumed by Tasks 8, 9, and 11. Compilation (via the existing full test suite) is the verification.

- [ ] **Step 1: Add `STSHost` to `manifests.Params`**

In `internal/manifests/render.go`, modify the `Params` struct:

```go
// Params are the template variables available to every manifest template.
type Params struct {
	Namespace                      string
	GatewayName                    string
	GatewayNS                      string
	GatewayHost                    string
	KeycloakHost                   string
	OpenAIKey                      string
	StockMCPImage                  string
	CurrencyMCPImage               string
	OAuthTokenExchangeClientSecret string
	STSHost                        string
}
```

- [ ] **Step 2: Add the new fields to `scenarios/types.go`**

Modify the `Step` struct:

```go
// Step is one screen of the wizard within a Pillar.
type Step struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Explanation string          `json:"explanation"`
	Diagram     string          `json:"diagram"`
	Policies    []Policy        `json:"policies"`
	Presets     []RequestPreset `json:"presets"`
	AgentDemo   bool            `json:"agentDemo"`
	VirtualKeys bool            `json:"virtualKeys"`
	// LoginDemo marks the one step (auth pillar's "Real login") that
	// renders a dedicated login pane instead of Configure/Request panes -
	// there's no policy to apply and no request to drive, just a real
	// Authorization Code redirect.
	LoginDemo bool `json:"loginDemo"`
}
```

Modify the `RequestPreset` struct:

```go
// RequestPreset is one clickops Drive-Request-pane button: a pre-filled,
// editable request the presenter can send.
type RequestPreset struct {
	ID       string            `json:"id"`
	Title    string            `json:"title"`
	Identity string            `json:"identity"`
	Method   string            `json:"method"`
	Path     string            `json:"path"`
	Headers  map[string]string `json:"headers"`
	Body     string            `json:"body"`
	Stream   bool              `json:"stream"`
	// RequiresSessionToken marks a preset that must use the real token
	// captured by the auth pillar's login step instead of
	// gateway.Client's own Resource Owner Password Credentials shortcut -
	// handleRequest returns a clear "log in first" error rather than
	// silently falling back when one isn't present.
	RequiresSessionToken bool `json:"requiresSessionToken"`
	// ExchangeViaSTS marks a preset whose captured session token must be
	// exchanged through Solo's legacy STS (Delegation, sub+act claims)
	// before being forwarded - only meaningful alongside
	// RequiresSessionToken.
	ExchangeViaSTS bool `json:"exchangeViaSts"`
}
```

- [ ] **Step 3: Run the full Go test suite to confirm nothing broke**

Run: `go build ./... && go test ./... -v`
Expected: PASS (no behavior changed yet - `scenarios.ManifestParams` is a type alias for `manifests.Params`, so it picks up `STSHost` automatically; the two new preset/step bools default to `false` everywhere they aren't explicitly set, which is every existing call site)

- [ ] **Step 4: Commit**

```bash
git add internal/manifests/render.go internal/scenarios/types.go
git commit -m "feat(scenarios): add session-token and login-demo fields"
```

---

### Task 7: HTTP handlers - `/auth/login`, `/auth/callback`, `/api/auth/status`

**Files:**
- Modify: `internal/api/server.go`
- Create: `internal/api/auth_login_handler.go`
- Test: `internal/api/auth_login_handler_test.go`

**Interfaces:**
- Consumes: `oauthlogin.NewStore`, `oauthlogin.Flow`, `oauthlogin.Token`, `oauthlogin.GeneratePKCE`, `oauthlogin.GenerateState`, `oauthlogin.ExchangeCode`, `oauthlogin.NewSessionID`, `oauthlogin.SessionCookieName` (Tasks 1-2)
- Produces: `api.WithOAuthLogin(keycloakBaseURL, actorTokenPath string) Option`, `(*Server).sessionIDFromCookie(r *http.Request) string` (also consumed by Task 8), routes `GET /auth/login`, `GET /auth/callback`, `GET /api/auth/status`

- [ ] **Step 1: Write the failing test**

Create `internal/api/auth_login_handler_test.go`:

```go
package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/day0ops/agentgateway-demo-wizard/internal/oauthlogin"
)

func TestHandleAuthLoginRedirectsToKeycloakWithPKCE(t *testing.T) {
	s := NewServer(WithOAuthLogin("https://keycloak.demo.example.com", "/dev/null"))

	req := httptest.NewRequest(http.MethodGet, "/auth/login?identity=team-alpha&return_to=auth", nil)
	req.Host = "wizard.demo.example.com"
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parsing Location: %v", err)
	}
	if !strings.HasPrefix(loc.String(), "https://keycloak.demo.example.com/realms/agw-dev/protocol/openid-connect/auth") {
		t.Fatalf("expected a redirect to Keycloak's authorize endpoint, got %q", loc.String())
	}
	q := loc.Query()
	if q.Get("client_id") != "agw-client-public" {
		t.Fatalf("expected client_id=agw-client-public, got %q", q.Get("client_id"))
	}
	if q.Get("redirect_uri") != "https://wizard.demo.example.com/auth/callback" {
		t.Fatalf("expected the redirect_uri to be built from the request's own Host, got %q", q.Get("redirect_uri"))
	}
	if q.Get("code_challenge") == "" || q.Get("state") == "" {
		t.Fatalf("expected non-empty code_challenge and state, got %+v", q)
	}
	if q.Get("code_challenge_method") != "S256" {
		t.Fatalf("expected S256, got %q", q.Get("code_challenge_method"))
	}
}

func TestHandleAuthLoginRequiresIdentity(t *testing.T) {
	s := NewServer(WithOAuthLogin("https://keycloak.demo.example.com", "/dev/null"))

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a missing identity, got %d", rec.Code)
	}
}

func TestHandleAuthCallbackCompletesLoginAndSetsCookie(t *testing.T) {
	keycloak := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"real-token","expires_in":300}`))
	}))
	defer keycloak.Close()

	s := NewServer(WithOAuthLogin(keycloak.URL, "/dev/null"))

	// Drive handleAuthLogin first so a real state/verifier is on file -
	// the callback can't be exercised with a fabricated state (TakeFlow
	// would reject it).
	loginReq := httptest.NewRequest(http.MethodGet, "/auth/login?identity=team-alpha&return_to=auth", nil)
	loginReq.Host = "wizard.demo.example.com"
	loginRec := httptest.NewRecorder()
	s.ServeHTTP(loginRec, loginReq)
	loc, _ := url.Parse(loginRec.Header().Get("Location"))
	state := loc.Query().Get("state")

	callbackReq := httptest.NewRequest(http.MethodGet, "/auth/callback?code=auth-code-1&state="+state, nil)
	callbackReq.Host = "wizard.demo.example.com"
	callbackRec := httptest.NewRecorder()
	s.ServeHTTP(callbackRec, callbackReq)

	if callbackRec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d: %s", callbackRec.Code, callbackRec.Body.String())
	}
	if callbackRec.Header().Get("Location") != "/?returnTo=auth" {
		t.Fatalf("expected a redirect back into the SPA with returnTo=auth, got %q", callbackRec.Header().Get("Location"))
	}
	cookies := callbackRec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != oauthlogin.SessionCookieName {
		t.Fatalf("expected a %s cookie to be set, got %+v", oauthlogin.SessionCookieName, cookies)
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/auth/status?identity=team-alpha", nil)
	statusReq.AddCookie(cookies[0])
	statusRec := httptest.NewRecorder()
	s.ServeHTTP(statusRec, statusReq)
	if !strings.Contains(statusRec.Body.String(), `"loggedIn":true`) {
		t.Fatalf("expected loggedIn:true after a completed login, got: %s", statusRec.Body.String())
	}
}

func TestHandleAuthCallbackRejectsUnknownState(t *testing.T) {
	s := NewServer(WithOAuthLogin("https://keycloak.demo.example.com", "/dev/null"))

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=auth-code-1&state=never-issued", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unknown state, got %d", rec.Code)
	}
}

func TestHandleAuthStatusFalseWithNoSession(t *testing.T) {
	s := NewServer(WithOAuthLogin("https://keycloak.demo.example.com", "/dev/null"))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/status?identity=team-alpha", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `"loggedIn":false`) {
		t.Fatalf("expected loggedIn:false with no session cookie, got: %s", rec.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/... -run TestHandleAuth -v`
Expected: FAIL to compile with "undefined: WithOAuthLogin"

- [ ] **Step 3: Write the implementation**

In `internal/api/server.go`, add the `strings` import, `oauthlogin` import, new `Server` fields, and the new `Option`:

```go
package api

import (
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/day0ops/agentgateway-demo-wizard/internal/agent"
	"github.com/day0ops/agentgateway-demo-wizard/internal/config"
	"github.com/day0ops/agentgateway-demo-wizard/internal/gateway"
	"github.com/day0ops/agentgateway-demo-wizard/internal/k8s"
	"github.com/day0ops/agentgateway-demo-wizard/internal/oauthlogin"
	"github.com/day0ops/agentgateway-demo-wizard/internal/scenarios"
)

// Server is the wizard's HTTP server. Routes are registered in NewServer;
// later packages extend it by adding an Option that stores a new dependency
// and registers the routes that need it.
type Server struct {
	mux                    *http.ServeMux
	spaFS                  fs.FS
	cfg                    *config.Config
	scenarios              []scenarios.Pillar
	applier                *k8s.Applier
	deploymentReadyTimeout time.Duration
	gatewayClient          *gateway.Client
	agentRunner            *agent.Runner
	agentTools             []agent.ToolSpec
	version                string
	oauthStore             *oauthlogin.Store
	keycloakBaseURL        string
	actorTokenPath         string
	oauthHTTPClient        *http.Client
}

// Option configures a Server at construction time.
type Option func(*Server)

// WithVersion sets the version string GET /api/config reports - the git tag
// this binary was built from, injected via -ldflags at build time (see
// cmd/wizard/main.go). Empty if never called (local `go run`/`go test`
// without ldflags).
func WithVersion(v string) Option {
	return func(s *Server) {
		s.version = v
	}
}

// WithOAuthLogin registers the real Authorization Code + PKCE login flow
// (GET /auth/login, GET /auth/callback, GET /api/auth/status) used by the
// auth pillar in place of the wizard's own Resource Owner Password
// Credentials shortcut. keycloakBaseURL must include a scheme, matching
// gateway.NewClient's own convention. actorTokenPath is where the wizard's
// own Kubernetes ServiceAccount token is projected - see
// oauthlogin.DefaultActorTokenPath for the production default; tests
// override it to point at a fixture file.
func WithOAuthLogin(keycloakBaseURL, actorTokenPath string) Option {
	return func(s *Server) {
		s.oauthStore = oauthlogin.NewStore()
		s.keycloakBaseURL = strings.TrimRight(keycloakBaseURL, "/")
		s.actorTokenPath = actorTokenPath
		s.oauthHTTPClient = &http.Client{Timeout: 15 * time.Second}
	}
}

// NewServer builds the wizard's HTTP server with all routes registered.
func NewServer(opts ...Option) *Server {
	s := &Server{mux: http.NewServeMux(), deploymentReadyTimeout: defaultDeploymentReadyTimeout}

	for _, opt := range opts {
		opt(s)
	}

	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /api/config", s.handleConfig)
	s.mux.HandleFunc("GET /api/scenarios", s.handleScenarios)
	s.mux.HandleFunc("GET /api/config/view", s.handleConfigView)
	s.mux.HandleFunc("POST /api/config/apply", s.handleConfigApply)
	s.mux.HandleFunc("POST /api/config/revert", s.handleConfigRevert)
	s.mux.HandleFunc("POST /api/config/reset", s.handleConfigReset)
	s.mux.HandleFunc("POST /api/request", s.handleRequest)
	s.mux.HandleFunc("POST /api/agent/run", s.handleAgentRun)
	s.mux.HandleFunc("POST /api/virtual-keys", s.handleVirtualKeyCreate)
	s.mux.HandleFunc("POST /api/virtual-keys/{name}/rotate", s.handleVirtualKeyRotate)
	s.mux.HandleFunc("GET /auth/login", s.handleAuthLogin)
	s.mux.HandleFunc("GET /auth/callback", s.handleAuthCallback)
	s.mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	s.registerSPA()

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
```

Create `internal/api/auth_login_handler.go`:

```go
package api

import (
	"net/http"
	"net/url"

	"github.com/day0ops/agentgateway-demo-wizard/internal/oauthlogin"
)

const authClientID = "agw-client-public"

// handleAuthLogin starts a real Authorization Code + PKCE login for
// identity, redirecting the browser to Keycloak's own hosted login page -
// the wizard acting as its own OAuth client (see oauthlogin.ExchangeCode),
// not a password grant.
func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	identity := r.URL.Query().Get("identity")
	if identity == "" {
		http.Error(w, "missing identity", http.StatusBadRequest)
		return
	}
	returnTo := r.URL.Query().Get("return_to")

	verifier, challenge, err := oauthlogin.GeneratePKCE()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	state, err := oauthlogin.GenerateState()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	s.oauthStore.SaveFlow(state, oauthlogin.Flow{Identity: identity, ReturnTo: returnTo, Verifier: verifier})

	params := url.Values{
		"client_id":             {authClientID},
		"redirect_uri":          {s.authCallbackURL(r)},
		"response_type":         {"code"},
		"scope":                 {"openid"},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	authorizeURL := s.keycloakBaseURL + "/realms/agw-dev/protocol/openid-connect/auth?" + params.Encode()
	http.Redirect(w, r, authorizeURL, http.StatusFound)
}

// handleAuthCallback completes the exchange Keycloak's redirect carries
// the authorization code back for, captures the real token server-side,
// and sends the browser back into the SPA.
func (s *Server) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		http.Error(w, "missing state or code", http.StatusBadRequest)
		return
	}

	flow, ok := s.oauthStore.TakeFlow(state)
	if !ok {
		http.Error(w, "unknown or expired login", http.StatusBadRequest)
		return
	}

	tok, err := oauthlogin.ExchangeCode(r.Context(), s.oauthHTTPClient, s.keycloakBaseURL, authClientID, s.authCallbackURL(r), code, flow.Verifier)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err)
		return
	}

	sessionID := s.sessionIDFromCookie(r)
	if sessionID == "" {
		sessionID, err = oauthlogin.NewSessionID()
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     oauthlogin.SessionCookieName,
			Value:    sessionID,
			Path:     "/",
			HttpOnly: true,
			Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
			SameSite: http.SameSiteLaxMode,
		})
	}
	s.oauthStore.PutToken(sessionID, flow.Identity, tok)

	http.Redirect(w, r, "/?returnTo="+url.QueryEscape(flow.ReturnTo), http.StatusFound)
}

// handleAuthStatus reports whether the current browser session already
// holds a real, captured token for identity - the SPA polls this after
// landing back from a login redirect, since the session cookie itself is
// httpOnly and unreadable from JS by design.
func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	identity := r.URL.Query().Get("identity")
	sessionID := s.sessionIDFromCookie(r)
	_, loggedIn := s.oauthStore.GetToken(sessionID, identity)
	writeJSON(w, http.StatusOK, map[string]bool{"loggedIn": loggedIn})
}

// authCallbackURL builds this wizard's own callback URL from the incoming
// request's own Host header - agw-client-public's redirect URIs are a
// wildcard in the agw-dev realm (confirmed in agentgateway-field-kit's
// Keycloak addon), so any value works and no fixed public-URL config is
// needed.
func (s *Server) authCallbackURL(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
		scheme = "http"
	}
	return scheme + "://" + r.Host + "/auth/callback"
}

func (s *Server) sessionIDFromCookie(r *http.Request) string {
	c, err := r.Cookie(oauthlogin.SessionCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/api/... -v`
Expected: PASS (all api tests, including the 5 new ones)

- [ ] **Step 5: Commit**

```bash
git add internal/api/server.go internal/api/auth_login_handler.go internal/api/auth_login_handler_test.go
git commit -m "feat(api): add real Authorization Code + PKCE login endpoints"
```

---

### Task 8: `handleRequest` - use captured session tokens

**Files:**
- Modify: `internal/api/request_handler.go`
- Test: `internal/api/request_handler_test.go`

**Interfaces:**
- Consumes: `(*Server).sessionIDFromCookie` (Task 7), `(*Server).oauthStore`/`actorTokenPath`/`oauthHTTPClient`/`cfg` (Task 7, Task 4), `oauthlogin.ReadActorToken`, `oauthlogin.ExchangeViaSTS` (Task 3), `(*gateway.Client).ProxyWithToken` (Task 5), `scenarios.RequestPreset.RequiresSessionToken`/`ExchangeViaSTS` (Task 6, threaded through `driveRequest`)

- [ ] **Step 1: Write the failing test**

Append to `internal/api/request_handler_test.go` (add `"os"`, `"path/filepath"`, `"time"`, and `"github.com/day0ops/agentgateway-demo-wizard/internal/config"`, `"github.com/day0ops/agentgateway-demo-wizard/internal/oauthlogin"` to the import block):

```go
func TestRequestHandlerRequiresLoginForSessionTokenPresets(t *testing.T) {
	client := gateway.NewClient("http://unused", "http://unused", "test-password")
	s := NewServer(WithGateway(client), WithOAuthLogin("https://keycloak.demo.example.com", "/dev/null"))

	reqBody := `{"identity":"team-alpha","method":"GET","path":"/secure/exchange-standard","requiresSessionToken":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/request", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when no real session token has been captured, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "log in as") {
		t.Fatalf("expected a clear log-in-first error, got: %s", rec.Body.String())
	}
}

func TestRequestHandlerUsesCapturedSessionTokenWhenPresent(t *testing.T) {
	var gotAuth string
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"headers":{}}`))
	}))
	defer gatewayServer.Close()

	client := gateway.NewClient("http://unused", gatewayServer.URL, "test-password")
	s := NewServer(WithGateway(client), WithOAuthLogin("https://keycloak.demo.example.com", "/dev/null"))
	s.oauthStore.PutToken("session-1", "team-alpha", oauthlogin.Token{AccessToken: "real-token", ExpiresAt: time.Now().Add(time.Hour)})

	reqBody := `{"identity":"team-alpha","method":"GET","path":"/secure/exchange-standard","requiresSessionToken":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/request", strings.NewReader(reqBody))
	req.AddCookie(&http.Cookie{Name: oauthlogin.SessionCookieName, Value: "session-1"})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if gotAuth != "Bearer real-token" {
		t.Fatalf("expected the captured real token to be forwarded, got %q", gotAuth)
	}
}

func TestRequestHandlerExchangesViaSTSWhenRequested(t *testing.T) {
	sts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"sts-exchanged-token"}`))
	}))
	defer sts.Close()
	stsHost := strings.TrimPrefix(sts.URL, "http://")

	var gotAuth string
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer gatewayServer.Close()

	actorTokenPath := filepath.Join(t.TempDir(), "actor-token")
	if err := os.WriteFile(actorTokenPath, []byte("wizard-sa-token"), 0o600); err != nil {
		t.Fatalf("writing actor token fixture: %v", err)
	}

	client := gateway.NewClient("http://unused", gatewayServer.URL, "test-password")
	s := NewServer(
		WithGateway(client),
		WithOAuthLogin("https://keycloak.demo.example.com", actorTokenPath),
		WithConfig(&config.Config{STSHost: stsHost}),
	)
	s.oauthStore.PutToken("session-1", "team-alpha", oauthlogin.Token{AccessToken: "real-token", ExpiresAt: time.Now().Add(time.Hour)})

	reqBody := `{"identity":"team-alpha","method":"GET","path":"/secure/exchange-sts","requiresSessionToken":true,"exchangeViaSts":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/request", strings.NewReader(reqBody))
	req.AddCookie(&http.Cookie{Name: oauthlogin.SessionCookieName, Value: "session-1"})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if gotAuth != "Bearer sts-exchanged-token" {
		t.Fatalf("expected the STS-exchanged token to be forwarded, got %q", gotAuth)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/... -run TestRequestHandler -v`
Expected: FAIL - `requiresSessionToken`/`exchangeViaSts` aren't read from the request body yet, so all three new assertions fail (first two get a normal ROPC-path 502/200 instead of the 401/real-token behavior; the third never reaches the STS server)

- [ ] **Step 3: Write the implementation**

Replace the whole of `internal/api/request_handler.go`:

```go
package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/day0ops/agentgateway-demo-wizard/internal/gateway"
	"github.com/day0ops/agentgateway-demo-wizard/internal/oauthlogin"
)

// WithGateway registers POST /api/request, backed by client.
func WithGateway(client *gateway.Client) Option {
	return func(s *Server) {
		s.gatewayClient = client
	}
}

type driveRequest struct {
	Identity             string            `json:"identity"`
	Method               string            `json:"method"`
	Path                 string            `json:"path"`
	Headers              map[string]string `json:"headers"`
	Body                 string            `json:"body"`
	RequiresSessionToken bool              `json:"requiresSessionToken"`
	ExchangeViaSTS       bool              `json:"exchangeViaSts"`
}

type driveResponse struct {
	StatusCode int    `json:"statusCode"`
	Body       string `json:"body"`
	LatencyMS  int64  `json:"latencyMs"`
}

// isStreamingRequest reports whether req's body asks for a streamed
// response, by checking for a top-level "stream": true field - the same
// field OpenAI's own API reads, so the wizard doesn't need its own parallel
// flag. A malformed body is treated as non-streaming; handleRequest's
// normal JSON decode already validates the outer request shape.
func isStreamingRequest(body string) bool {
	var parsed struct {
		Stream bool `json:"stream"`
	}
	_ = json.Unmarshal([]byte(body), &parsed)
	return parsed.Stream
}

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	var req driveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	gwReq := gateway.Request{
		Identity: req.Identity,
		Method:   req.Method,
		Path:     req.Path,
		Headers:  req.Headers,
		Body:     req.Body,
	}

	if req.RequiresSessionToken {
		s.handleSessionTokenRequest(w, r, req, gwReq)
		return
	}

	if isStreamingRequest(req.Body) {
		if err := s.gatewayClient.ProxyStream(r.Context(), gwReq, w); err != nil {
			writeJSONError(w, http.StatusBadGateway, err)
		}
		return
	}

	resp, err := s.gatewayClient.Proxy(r.Context(), gwReq)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, driveResponse{StatusCode: resp.StatusCode, Body: resp.Body, LatencyMS: resp.LatencyMS})
}

// handleSessionTokenRequest serves presets that require a real,
// interactively-obtained token from the auth pillar's login step, instead
// of one of gateway.Client's own fixed-identity Resource Owner Password
// Credentials tokens. It never falls back to that mechanism on a missing
// token - silently doing so would undercut the pillar's entire point.
func (s *Server) handleSessionTokenRequest(w http.ResponseWriter, r *http.Request, req driveRequest, gwReq gateway.Request) {
	sessionID := s.sessionIDFromCookie(r)
	tok, ok := s.oauthStore.GetToken(sessionID, req.Identity)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, fmt.Errorf("log in as %q first - see the login step above", req.Identity))
		return
	}

	bearer := tok.AccessToken
	if req.ExchangeViaSTS {
		actorToken, err := oauthlogin.ReadActorToken(s.actorTokenPath)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err)
			return
		}
		exchanged, err := oauthlogin.ExchangeViaSTS(r.Context(), s.oauthHTTPClient, s.cfg.STSHost, bearer, actorToken)
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, err)
			return
		}
		bearer = exchanged
	}

	resp, err := s.gatewayClient.ProxyWithToken(r.Context(), gwReq, bearer)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, driveResponse{StatusCode: resp.StatusCode, Body: resp.Body, LatencyMS: resp.LatencyMS})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/api/... -v`
Expected: PASS (all api tests, including the 3 new ones)

- [ ] **Step 5: Commit**

```bash
git add internal/api/request_handler.go internal/api/request_handler_test.go
git commit -m "feat(api): serve session-token presets with the real captured token"
```

---

### Task 9: `manifestParams()` - thread `STSHost` through

**Files:**
- Modify: `internal/api/config_apply_handler.go`

**Interfaces:**
- Consumes: `config.Config.STSHost` (Task 4), `scenarios.ManifestParams.STSHost` (Task 6)

No new test - this is covered transitively by Task 11's manifest-rendering test, since that's the first place `STSHost` actually needs to show up in rendered output.

- [ ] **Step 1: Modify `manifestParams()`**

In `internal/api/config_apply_handler.go`, modify the function:

```go
func (s *Server) manifestParams() scenarios.ManifestParams {
	params := scenarios.ManifestParams{
		Namespace:   "agentgateway-system",
		GatewayName: "agentgateway-gw",
		GatewayNS:   "agentgateway-system",
	}
	if s.cfg != nil {
		params.GatewayHost = s.cfg.GatewayHost
		params.KeycloakHost = s.cfg.KeycloakHost
		params.OpenAIKey = s.cfg.OpenAIKey
		params.StockMCPImage = s.cfg.StockMCPImage
		params.CurrencyMCPImage = s.cfg.CurrencyMCPImage
		params.OAuthTokenExchangeClientSecret = s.cfg.OAuthTokenExchangeClientSecret
		params.STSHost = s.cfg.STSHost
	}
	return params
}
```

- [ ] **Step 2: Run the full Go test suite to confirm nothing broke**

Run: `go build ./... && go test ./... -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/api/config_apply_handler.go
git commit -m "feat(api): thread STSHost into manifest params"
```

---

### Task 10: Manifest templates - Standard and STS token exchange; drop orphaned `auth/jwt.yaml.tmpl`

**Files:**
- Create: `internal/manifests/templates/auth/token-exchange-standard.yaml.tmpl`
- Create: `internal/manifests/templates/auth/token-exchange-sts.yaml.tmpl`
- Delete: `internal/manifests/templates/auth/jwt.yaml.tmpl`

**Interfaces:**
- Consumes: `manifests.Params.STSHost`, `.KeycloakHost`, `.OAuthTokenExchangeClientSecret`, `.Namespace`, `.GatewayName`, `.GatewayNS` (Task 6 and pre-existing)

This task has no Go code of its own - manifest templates render via `internal/manifests.Render`, already covered by `TestAuthPoliciesRenderValidManifests` in Task 11. There's nothing to TDD here directly; verification is `go build ./...` (confirms the `//go:embed templates` directive still finds a valid tree) followed by Task 11's test exercising the actual render.

- [ ] **Step 1: Create the Standard (Impersonation) exchange template**

Modeled directly on the already-working `internal/manifests/templates/agent/token-exchange.yaml.tmpl`: validates the real user JWT, then has agentgateway itself call Keycloak's token endpoint to exchange it for a backend-scoped token before forwarding - no extra infrastructure, reusing the already-provisioned `agw-token-exchange` Keycloak client.

Create `internal/manifests/templates/auth/token-exchange-standard.yaml.tmpl`:

```yaml
apiVersion: enterpriseagentgateway.solo.io/v1alpha1
kind: EnterpriseAgentgatewayBackend
metadata:
  name: auth-standard-exchange-jwks
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/managed-by: agentgateway-demo-wizard
spec:
  policies:
    tls:
      sni: {{ .KeycloakHost }}
  static:
    host: {{ .KeycloakHost }}
    port: 443
---
apiVersion: enterpriseagentgateway.solo.io/v1alpha1
kind: EnterpriseAgentgatewayBackend
metadata:
  name: auth-standard-exchange-endpoint
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/managed-by: agentgateway-demo-wizard
spec:
  policies:
    tls:
      sni: {{ .KeycloakHost }}
  static:
    host: {{ .KeycloakHost }}
    port: 443
---
apiVersion: v1
kind: Secret
metadata:
  name: auth-standard-exchange-client-secret
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/managed-by: agentgateway-demo-wizard
type: Opaque
stringData:
  client_secret: {{ .OAuthTokenExchangeClientSecret }}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: auth-standard-exchange-echo
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/managed-by: agentgateway-demo-wizard
    app: auth-standard-exchange-echo
spec:
  replicas: 1
  selector:
    matchLabels:
      app: auth-standard-exchange-echo
  template:
    metadata:
      labels:
        app: auth-standard-exchange-echo
    spec:
      containers:
        - name: httpbin
          image: kennethreitz/httpbin
          ports:
            - containerPort: 80
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 128Mi
---
apiVersion: v1
kind: Service
metadata:
  name: auth-standard-exchange-echo
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/managed-by: agentgateway-demo-wizard
spec:
  selector:
    app: auth-standard-exchange-echo
  ports:
    - port: 80
      targetPort: 80
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: auth-standard-exchange-echo
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/managed-by: agentgateway-demo-wizard
spec:
  parentRefs:
    - name: {{ .GatewayName }}
      namespace: {{ .GatewayNS }}
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /secure/exchange-standard
      filters:
        - type: URLRewrite
          urlRewrite:
            path:
              type: ReplacePrefixMatch
              replacePrefixMatch: /headers
      backendRefs:
        - name: auth-standard-exchange-echo
          port: 80
---
apiVersion: enterpriseagentgateway.solo.io/v1alpha1
kind: EnterpriseAgentgatewayPolicy
metadata:
  name: auth-standard-exchange
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/managed-by: agentgateway-demo-wizard
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      name: auth-standard-exchange-echo
  traffic:
    jwtAuthentication:
      mode: Strict
      providers:
        - issuer: https://{{ .KeycloakHost }}/realms/agw-dev
          audiences:
            - account
          jwks:
            remote:
              jwksPath: realms/agw-dev/protocol/openid-connect/certs
              cacheDuration: 5m
              backendRef:
                group: enterpriseagentgateway.solo.io
                kind: EnterpriseAgentgatewayBackend
                name: auth-standard-exchange-jwks
  backend:
    auth:
      oauthTokenExchange:
        backendRef:
          group: enterpriseagentgateway.solo.io
          kind: EnterpriseAgentgatewayBackend
          name: auth-standard-exchange-endpoint
        path: /realms/agw-dev/protocol/openid-connect/token
        grantType: TokenExchange
        clientAuth:
          method: ClientSecretBasic
          clientId: agw-token-exchange
          secretRef:
            name: auth-standard-exchange-client-secret
            key: client_secret
        subjectToken:
          tokenType: AccessToken
```

- [ ] **Step 2: Create the STS (Delegation) exchange template**

Modeled on `obo-token-exchange`'s pattern: the JWT this backend validates is minted by the STS itself (issuer = STS, not Keycloak), since by the time a request reaches this route the wizard has already performed the STS exchange server-side (see Task 8's `handleSessionTokenRequest`) and forwards the resulting `sub`+`act` token directly.

Create `internal/manifests/templates/auth/token-exchange-sts.yaml.tmpl`:

```yaml
apiVersion: enterpriseagentgateway.solo.io/v1alpha1
kind: EnterpriseAgentgatewayBackend
metadata:
  name: auth-sts-exchange-jwks
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/managed-by: agentgateway-demo-wizard
spec:
  static:
    host: {{ .STSHost }}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: auth-sts-exchange-echo
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/managed-by: agentgateway-demo-wizard
    app: auth-sts-exchange-echo
spec:
  replicas: 1
  selector:
    matchLabels:
      app: auth-sts-exchange-echo
  template:
    metadata:
      labels:
        app: auth-sts-exchange-echo
    spec:
      containers:
        - name: httpbin
          image: kennethreitz/httpbin
          ports:
            - containerPort: 80
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 128Mi
---
apiVersion: v1
kind: Service
metadata:
  name: auth-sts-exchange-echo
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/managed-by: agentgateway-demo-wizard
spec:
  selector:
    app: auth-sts-exchange-echo
  ports:
    - port: 80
      targetPort: 80
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: auth-sts-exchange-echo
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/managed-by: agentgateway-demo-wizard
spec:
  parentRefs:
    - name: {{ .GatewayName }}
      namespace: {{ .GatewayNS }}
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /secure/exchange-sts
      filters:
        - type: URLRewrite
          urlRewrite:
            path:
              type: ReplacePrefixMatch
              replacePrefixMatch: /headers
      backendRefs:
        - name: auth-sts-exchange-echo
          port: 80
---
apiVersion: enterpriseagentgateway.solo.io/v1alpha1
kind: EnterpriseAgentgatewayPolicy
metadata:
  name: auth-sts-exchange
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/managed-by: agentgateway-demo-wizard
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      name: auth-sts-exchange-echo
  traffic:
    jwtAuthentication:
      mode: Strict
      providers:
        - issuer: http://{{ .STSHost }}
          audiences:
            - agentgateway
          jwks:
            remote:
              jwksPath: .well-known/jwks.json
              cacheDuration: 5m
              backendRef:
                group: enterpriseagentgateway.solo.io
                kind: EnterpriseAgentgatewayBackend
                name: auth-sts-exchange-jwks
```

- [ ] **Step 3: Delete the orphaned JWT-echo template**

`auth/jwt.yaml.tmpl`'s `auth-jwt-echo`/`auth-jwt-jwks` resources back the old `auth-jwt` step being removed in Task 11 - nothing else references `/secure/jwt` after that step is gone.

```bash
rm internal/manifests/templates/auth/jwt.yaml.tmpl
```

- [ ] **Step 4: Run build to verify the embedded template tree still compiles**

Run: `go build ./...`
Expected: PASS (the `//go:embed templates` directive in `internal/manifests/render.go` re-scans the directory tree at build time; a missing or malformed template file would fail here)

- [ ] **Step 5: Commit**

```bash
git add internal/manifests/templates/auth/token-exchange-standard.yaml.tmpl internal/manifests/templates/auth/token-exchange-sts.yaml.tmpl
git rm internal/manifests/templates/auth/jwt.yaml.tmpl
git commit -m "feat(manifests): add standard and STS token-exchange templates"
```

---

### Task 11: Rewrite `authPillar()` into 4 steps; rewrite `auth_test.go`

**Files:**
- Modify: `internal/scenarios/registry.go`
- Modify: `internal/scenarios/auth_test.go`

**Interfaces:**
- Consumes: `scenarios.Step.LoginDemo`, `RequestPreset.RequiresSessionToken`/`ExchangeViaSTS` (Task 6), `auth/token-exchange-standard.yaml.tmpl`, `auth/token-exchange-sts.yaml.tmpl` (Task 10)

- [ ] **Step 1: Write the failing test**

Replace the whole of `internal/scenarios/auth_test.go`:

```go
package scenarios

import (
	"strings"
	"testing"
)

func TestRegistryIncludesPillar2Auth(t *testing.T) {
	reg := Registry()
	if len(reg) != 7 {
		t.Fatalf("expected 7 pillars (intro, routing, auth, guardrails, cost, agent, wrap), got %d", len(reg))
	}

	auth := reg[2]
	if auth.ID != "auth" {
		t.Fatalf("expected pillar[2].ID 'auth', got %q", auth.ID)
	}
	if len(auth.Steps) != 4 {
		t.Fatalf("expected 4 auth steps, got %d", len(auth.Steps))
	}

	wantStepIDs := []string{"auth-login", "auth-rbac", "auth-exchange-standard", "auth-exchange-sts"}
	for i, want := range wantStepIDs {
		if auth.Steps[i].ID != want {
			t.Fatalf("expected step[%d].ID %q, got %q", i, want, auth.Steps[i].ID)
		}
	}
}

func TestAuthLoginStepHasNoPoliciesOrPresets(t *testing.T) {
	auth := Registry()[2]
	login := auth.Steps[0]

	if !login.LoginDemo {
		t.Fatalf("expected the login step to set LoginDemo")
	}
	if len(login.Policies) != 0 {
		t.Fatalf("expected the login step to have no policies, got %d", len(login.Policies))
	}
	if len(login.Presets) != 0 {
		t.Fatalf("expected the login step to have no presets, got %d", len(login.Presets))
	}
}

func TestAuthPoliciesRenderValidManifests(t *testing.T) {
	params := ManifestParams{
		Namespace:    "agentgateway-system",
		GatewayName:  "agentgateway-gw",
		GatewayNS:    "agentgateway-system",
		KeycloakHost: "keycloak.demo.example.com",
		STSHost:      "sts.demo.example.com:7777",
	}

	auth := Registry()[2]

	for _, step := range auth.Steps {
		if step.LoginDemo {
			continue // no policies to render - see TestAuthLoginStepHasNoPoliciesOrPresets
		}
		if len(step.Policies) == 0 {
			t.Fatalf("step %q has no policies", step.ID)
		}
		for _, policy := range step.Policies {
			docs, err := policy.Render(params)
			if err != nil {
				t.Fatalf("rendering policy %q: %v", policy.ID, err)
			}
			if len(docs) == 0 {
				t.Fatalf("policy %q rendered no documents", policy.ID)
			}
			joined := strings.Join(toStrings(docs), "\n---\n")
			if !strings.Contains(joined, "jwtAuthentication") {
				t.Fatalf("policy %q: expected jwtAuthentication, got: %s", policy.ID, joined)
			}
			if strings.Contains(joined, "{{") {
				t.Fatalf("policy %q: unresolved template variable in output: %s", policy.ID, joined)
			}
		}
	}

	rbac := findStep(auth.Steps, "auth-rbac")
	rbacDocs, err := rbac.Policies[0].Render(params)
	if err != nil {
		t.Fatalf("rendering auth-rbac: %v", err)
	}
	if !strings.Contains(strings.Join(toStrings(rbacDocs), "\n"), "authorization") {
		t.Fatalf("expected the auth-rbac policy to contain an authorization block")
	}

	standard := findStep(auth.Steps, "auth-exchange-standard")
	standardDocs, err := standard.Policies[0].Render(params)
	if err != nil {
		t.Fatalf("rendering auth-exchange-standard: %v", err)
	}
	standardJoined := strings.Join(toStrings(standardDocs), "\n")
	if !strings.Contains(standardJoined, "oauthTokenExchange") {
		t.Fatalf("expected the standard-exchange policy to contain oauthTokenExchange")
	}
	if !strings.Contains(standardJoined, params.KeycloakHost) {
		t.Fatalf("expected the standard-exchange policy to reference the Keycloak host")
	}

	sts := findStep(auth.Steps, "auth-exchange-sts")
	stsDocs, err := sts.Policies[0].Render(params)
	if err != nil {
		t.Fatalf("rendering auth-exchange-sts: %v", err)
	}
	stsJoined := strings.Join(toStrings(stsDocs), "\n")
	if !strings.Contains(stsJoined, params.STSHost) {
		t.Fatalf("expected the STS-exchange policy to reference the STS host, got: %s", stsJoined)
	}
}

func findStep(steps []Step, id string) Step {
	for _, s := range steps {
		if s.ID == id {
			return s
		}
	}
	return Step{}
}

func toStrings(docs [][]byte) []string {
	out := make([]string, len(docs))
	for i, d := range docs {
		out[i] = string(d)
	}
	return out
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scenarios/... -v`
Expected: FAIL - `authPillar()` still returns the old 2-step shape (`auth-jwt`, `auth-rbac`), so step count and step-ID assertions fail

- [ ] **Step 3: Write the implementation**

In `internal/scenarios/registry.go`, replace the whole `authPillar()` function:

```go
func authPillar() Pillar {
	return Pillar{
		ID:     "auth",
		Title:  "Who's allowed to do what",
		Teaser: "Identity-aware access control - every call is authenticated and authorized.",
		Steps: []Step{
			{
				ID:        "auth-login",
				Title:     "Real login",
				LoginDemo: true,
				Explanation: "Every other step in this pillar - and every other pillar in this wizard - " +
					"uses a quick password grant: the wizard itself asks Keycloak for a token using a " +
					"fixed demo username and password. That's fine for driving routing or cost demos, but " +
					"it's the wrong way to authenticate a real user, since it means handing a password " +
					"straight to a client. This step replaces it with a real Authorization Code + PKCE " +
					"login: the wizard redirects your browser to Keycloak's own hosted login page, " +
					"Keycloak redirects back with an authorization code, and the wizard exchanges that " +
					"code for a real access token server-side. Every step after this one uses that real " +
					"token, not a password grant.",
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
						ID:                   "team-alpha-allowed",
						Title:                "team-alpha (allowed)",
						Identity:             "team-alpha",
						Method:               "GET",
						Path:                 "/secure/rbac",
						Headers:              map[string]string{},
						Body:                 "",
						RequiresSessionToken: true,
					},
					{
						ID:                   "team-beta-denied",
						Title:                "team-beta (denied)",
						Identity:             "team-beta",
						Method:               "GET",
						Path:                 "/secure/rbac",
						Headers:              map[string]string{},
						Body:                 "",
						RequiresSessionToken: true,
					},
				},
			},
			{
				ID:    "auth-exchange-standard",
				Title: "Standard token exchange (Impersonation)",
				Explanation: "agentgateway supports two real token-exchange patterns; this is the one " +
					"agentgateway recommends for new work. The gateway itself performs an RFC 8693 token " +
					"exchange against Keycloak's own token endpoint - no extra infrastructure, reusing " +
					"the same agw-token-exchange client already provisioned for the agent pillar's " +
					"On-Behalf-Of step. The caller's real token (from the login step) is exchanged for a " +
					"new one scoped to this backend before the request is forwarded - the same identity, " +
					"re-signed (Impersonation), with no act claim.",
				Diagram: "sequenceDiagram\n" +
					"  participant Client\n" +
					"  participant AGW as agentgateway\n" +
					"  participant KC as Keycloak\n" +
					"  participant Echo as Echo backend\n" +
					"  Client->>AGW: GET /secure/exchange-standard (Bearer real token)\n" +
					"  AGW->>KC: validate JWT (JWKS)\n" +
					"  AGW->>KC: POST /token (grant_type=token-exchange)\n" +
					"  KC-->>AGW: backend-scoped token\n" +
					"  AGW->>Echo: forward with exchanged token\n" +
					"  Echo-->>Client: 200",
				Policies: []Policy{
					{ID: "auth-exchange-standard", Title: "Standard token exchange (Impersonation)", ManifestPath: "auth/token-exchange-standard.yaml.tmpl"},
				},
				Presets: []RequestPreset{
					{
						ID:                   "exchange-standard",
						Title:                "Call through the exchange",
						Identity:             "team-alpha",
						Method:               "GET",
						Path:                 "/secure/exchange-standard",
						Headers:              map[string]string{},
						Body:                 "",
						RequiresSessionToken: true,
					},
				},
			},
			{
				ID:    "auth-exchange-sts",
				Title: "STS token exchange (Delegation)",
				Explanation: "The second pattern is Solo's legacy, controller-side Security Token " +
					"Service - still supported, not recommended for new work, but the only one of the " +
					"two that produces an act claim: a token carrying both sub (the real user from the " +
					"login step) and act (the wizard itself, as the delegating agent). That's the " +
					"tradeoff - a separate STS component to run, in exchange for an audit trail that " +
					"shows who asked and which agent acted on their behalf. The wizard's own backend " +
					"calls the STS directly, using its own Kubernetes ServiceAccount token as the " +
					"actor_token.",
				Diagram: "sequenceDiagram\n" +
					"  participant Client\n" +
					"  participant Wizard\n" +
					"  participant STS\n" +
					"  participant AGW as agentgateway\n" +
					"  participant Echo as Echo backend\n" +
					"  Client->>Wizard: drive request (real session token)\n" +
					"  Wizard->>STS: POST /oauth2/token (subject_token=user, actor_token=wizard SA)\n" +
					"  STS-->>Wizard: token (sub=user, act=wizard)\n" +
					"  Wizard->>AGW: GET /secure/exchange-sts (Bearer delegated token)\n" +
					"  AGW->>STS: validate JWT (JWKS)\n" +
					"  AGW->>Echo: forward\n" +
					"  Echo-->>Client: 200",
				Policies: []Policy{
					{ID: "auth-exchange-sts", Title: "STS token exchange (Delegation)", ManifestPath: "auth/token-exchange-sts.yaml.tmpl"},
				},
				Presets: []RequestPreset{
					{
						ID:                   "exchange-sts",
						Title:                "Call through the STS exchange",
						Identity:             "team-alpha",
						Method:               "GET",
						Path:                 "/secure/exchange-sts",
						Headers:              map[string]string{},
						Body:                 "",
						RequiresSessionToken: true,
						ExchangeViaSTS:       true,
					},
				},
			},
		},
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scenarios/... -v`
Expected: PASS (all scenarios tests, including the 2 new ones)

- [ ] **Step 5: Commit**

```bash
git add internal/scenarios/registry.go internal/scenarios/auth_test.go
git commit -m "feat(scenarios): rework auth pillar into login, RBAC, standard-exchange, and STS-exchange steps"
```

---

### Task 12: Wire `WithOAuthLogin` into `cmd/wizard/main.go`

**Files:**
- Modify: `cmd/wizard/main.go`

**Interfaces:**
- Consumes: `api.WithOAuthLogin(keycloakBaseURL, actorTokenPath string) Option` (Task 7), `oauthlogin.DefaultActorTokenPath` (Task 3)

No new test - `main()` isn't unit-tested in this repo today (confirmed: no `main_test.go` exists). Verification is a successful build.

- [ ] **Step 1: Add the import and the new Option**

In `cmd/wizard/main.go`, add the `oauthlogin` import:

```go
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
	"github.com/day0ops/agentgateway-demo-wizard/internal/oauthlogin"
	"github.com/day0ops/agentgateway-demo-wizard/internal/scenarios"
	"github.com/day0ops/agentgateway-demo-wizard/web"
)
```

Add `api.WithOAuthLogin(...)` to the `api.NewServer(...)` call:

```go
	srv := api.NewServer(
		api.WithSPA(web.DistFS()),
		api.WithConfig(cfg),
		api.WithScenarios(scenarios.Registry()),
		api.WithApplier(applier),
		api.WithGateway(gatewayClient),
		api.WithAgent(agentRunner, agentToolSpecs()),
		api.WithOAuthLogin("https://"+cfg.KeycloakHost, oauthlogin.DefaultActorTokenPath),
		api.WithVersion(version),
	)
```

- [ ] **Step 2: Run the build to verify it compiles**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add cmd/wizard/main.go
git commit -m "feat(wizard): wire the real-login OAuth flow into main"
```

---

### Task 13: Frontend types - `web/src/lib/api.ts`

**Files:**
- Modify: `web/src/lib/api.ts`

**Interfaces:**
- Produces: `RequestPreset.requiresSessionToken: boolean`, `RequestPreset.exchangeViaSts: boolean`, `Step.loginDemo: boolean`

No new test - this file has no existing test of its own (its exported functions are exercised indirectly through `App.test.tsx`, see Task 15). Verification is `npx tsc -b`. **Correction (found during implementation):** `App.test.tsx`'s own `pillars` arrays are indeed untyped object literals passed through functions typed `unknown`, but three other component test files type their fixtures explicitly against these exported types - `Breadcrumb.test.tsx` (`pillars: Pillar[]`), `ExplanationBand.test.tsx` (`step: Step`), and `DriveRequestPane.test.tsx` (`preset`/`headerPreset`/`streamPreset`: `RequestPreset`). Adding these new required fields breaks `tsc -b` for those three files. This task must also add `loginDemo: false` to every `Step` fixture in `Breadcrumb.test.tsx`/`ExplanationBand.test.tsx` and `requiresSessionToken: false, exchangeViaSts: false` to every `RequestPreset` fixture in `DriveRequestPane.test.tsx`, in the same commit, so `tsc -b` stays clean.

- [ ] **Step 1: Add the new fields**

In `web/src/lib/api.ts`, modify `RequestPreset`:

```ts
export type RequestPreset = {
  id: string;
  title: string;
  identity: string;
  method: string;
  path: string;
  headers: Record<string, string>;
  body: string;
  stream: boolean;
  requiresSessionToken: boolean;
  exchangeViaSts: boolean;
};
```

Modify `Step`:

```ts
export type Step = {
  id: string;
  title: string;
  explanation: string;
  diagram: string;
  policies: Policy[];
  presets: RequestPreset[];
  agentDemo: boolean;
  virtualKeys: boolean;
  loginDemo: boolean;
};
```

- [ ] **Step 2: Run the typecheck to verify it passes**

Run: `cd web && npx tsc -b`
Expected: PASS (no code reads these new fields yet, so nothing can fail to compile against them; Task 15 is the first consumer)

- [ ] **Step 3: Commit**

```bash
git add web/src/lib/api.ts
git commit -m "feat(ui): add requiresSessionToken, exchangeViaSts, and loginDemo types"
```

---

### Task 14: `LoginPane` component

**Files:**
- Create: `web/src/components/LoginPane.tsx`
- Test: `web/src/components/LoginPane.test.tsx`

**Interfaces:**
- Consumes: `GET /api/auth/status?identity=...` (Task 7)
- Produces: `LoginPane({ identities: string[] })` React component

- [ ] **Step 1: Write the failing test**

Create `web/src/components/LoginPane.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { LoginPane } from "./LoginPane";

describe("LoginPane", () => {
  it("shows a login link for an identity with no captured token", async () => {
    globalThis.fetch = (async () =>
      ({
        ok: true,
        json: async () => ({ loggedIn: false }),
      }) as unknown as Response) as unknown as typeof fetch;

    render(<LoginPane identities={["team-alpha"]} />);

    expect(
      await screen.findByRole("link", { name: /log in as team-alpha/i }),
    ).toHaveAttribute(
      "href",
      "/auth/login?identity=team-alpha&return_to=auth",
    );
  });

  it("shows a logged-in badge once a real token has been captured", async () => {
    globalThis.fetch = (async () =>
      ({
        ok: true,
        json: async () => ({ loggedIn: true }),
      }) as unknown as Response) as unknown as typeof fetch;

    render(<LoginPane identities={["team-alpha"]} />);

    expect(
      await screen.findByText(/logged in \(real token\)/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: /log in as team-alpha/i }),
    ).not.toBeInTheDocument();
  });

  it("checks status independently for each identity", async () => {
    globalThis.fetch = ((url: RequestInfo | URL) => {
      const loggedIn = String(url).includes("team-alpha");
      return Promise.resolve({
        ok: true,
        json: async () => ({ loggedIn }),
      } as unknown as Response);
    }) as unknown as typeof fetch;

    render(<LoginPane identities={["team-alpha", "team-beta"]} />);

    expect(
      await screen.findByText(/logged in \(real token\)/i),
    ).toBeInTheDocument();
    expect(
      await screen.findByRole("link", { name: /log in as team-beta/i }),
    ).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && npx vitest run LoginPane`
Expected: FAIL - `./LoginPane` module doesn't exist yet

- [ ] **Step 3: Write the implementation**

Create `web/src/components/LoginPane.tsx`:

```tsx
import { useEffect, useState } from "react";
import { LogIn, CheckCircle2 } from "lucide-react";

type LoginStatus = Record<string, boolean>;

// LoginPane replaces Configure/Request for the auth pillar's login step -
// there's no policy to apply and no request to drive, just a real
// Authorization Code redirect per identity. The session cookie set by
// /auth/callback is httpOnly, so status is read back from the server
// instead of from anything client-readable.
export function LoginPane({ identities }: { identities: string[] }) {
  const [status, setStatus] = useState<LoginStatus>({});

  useEffect(() => {
    identities.forEach((identity) => {
      fetch(`/api/auth/status?identity=${identity}`)
        .then((res) => res.json())
        .then((data: { loggedIn: boolean }) =>
          setStatus((s) => ({ ...s, [identity]: data.loggedIn })),
        )
        .catch(() => undefined);
    });
  }, [identities]);

  return (
    <div className="space-y-4">
      <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-500">
        Log in
      </h2>
      <div className="space-y-3 rounded-xl border border-slate-200 p-4 shadow-sm dark:border-slate-800">
        {identities.map((identity) => (
          <div key={identity} className="flex items-center justify-between">
            <span className="font-mono text-sm">{identity}</span>
            {status[identity] ? (
              <span className="inline-flex items-center gap-1.5 rounded-full bg-green-50 px-2.5 py-0.5 text-xs font-medium text-green-700 dark:bg-green-950 dark:text-green-400">
                <CheckCircle2 size={12} />
                logged in (real token)
              </span>
            ) : (
              <a
                href={`/auth/login?identity=${identity}&return_to=auth`}
                className="inline-flex items-center gap-1.5 rounded-lg bg-slate-900 px-3 py-1 text-sm text-white shadow-sm dark:bg-slate-100 dark:text-slate-900"
              >
                <LogIn size={14} />
                Log in as {identity}
              </a>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd web && npx vitest run LoginPane`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add web/src/components/LoginPane.tsx web/src/components/LoginPane.test.tsx
git commit -m "feat(ui): add LoginPane for the real-login auth step"
```

---

### Task 15: `App.tsx` - render `LoginPane` for the login step; restore pillar position after redirect

**Files:**
- Modify: `web/src/App.tsx`
- Test: `web/src/App.test.tsx`

**Interfaces:**
- Consumes: `LoginPane` (Task 14), `Step.loginDemo` (Task 13)

- [ ] **Step 1: Write the failing test**

Append to `web/src/App.test.tsx`, inside the existing `describe("App", ...)` block:

```tsx
  it("shows the login pane for a step with loginDemo instead of Configure/Request panes", async () => {
    const pillars = [
      {
        id: "auth",
        title: "Who's allowed to do what",
        teaser: "",
        steps: [
          {
            id: "auth-login",
            title: "Real login",
            explanation: "",
            diagram: "",
            policies: [],
            presets: [],
            agentDemo: false,
            virtualKeys: false,
            loginDemo: true,
          },
        ],
      },
    ];
    globalThis.fetch = ((url: RequestInfo | URL) => {
      if (url === "/api/config") {
        return Promise.resolve({
          ok: true,
          json: async () => emptyConfig,
        } as unknown as Response);
      }
      if (typeof url === "string" && url.startsWith("/api/auth/status")) {
        return Promise.resolve({
          ok: true,
          json: async () => ({ loggedIn: false }),
        } as unknown as Response);
      }
      return Promise.resolve({
        ok: true,
        json: async () => pillars,
      } as unknown as Response);
    }) as unknown as typeof fetch;

    render(<App />);

    expect(await screen.findByText("Real login")).toBeInTheDocument();
    expect(
      await screen.findByRole("link", { name: /log in as team-alpha/i }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Configure")).not.toBeInTheDocument();
  });

  it("restores the matching pillar when the URL carries a returnTo param", async () => {
    const pillars = [
      {
        id: "intro",
        title: "Welcome",
        teaser: "",
        steps: [
          {
            id: "welcome",
            title: "Welcome",
            explanation: "",
            diagram: "",
            policies: [],
            presets: [],
            agentDemo: false,
            virtualKeys: false,
            loginDemo: false,
          },
        ],
      },
      {
        id: "auth",
        title: "Who's allowed to do what",
        teaser: "",
        steps: [
          {
            id: "auth-login",
            title: "Real login",
            explanation: "",
            diagram: "",
            policies: [],
            presets: [],
            agentDemo: false,
            virtualKeys: false,
            loginDemo: true,
          },
        ],
      },
    ];
    globalThis.fetch = ((url: RequestInfo | URL) => {
      if (url === "/api/config") {
        return Promise.resolve({
          ok: true,
          json: async () => emptyConfig,
        } as unknown as Response);
      }
      if (typeof url === "string" && url.startsWith("/api/auth/status")) {
        return Promise.resolve({
          ok: true,
          json: async () => ({ loggedIn: false }),
        } as unknown as Response);
      }
      return Promise.resolve({
        ok: true,
        json: async () => pillars,
      } as unknown as Response);
    }) as unknown as typeof fetch;

    const originalLocation = window.location;
    Object.defineProperty(window, "location", {
      value: { ...originalLocation, search: "?returnTo=auth" },
      writable: true,
    });

    render(<App />);

    expect(await screen.findByText("Real login")).toBeInTheDocument();

    Object.defineProperty(window, "location", {
      value: originalLocation,
      writable: true,
    });
  });
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && npx vitest run App`
Expected: FAIL - `activeStep.loginDemo` isn't read anywhere yet, so the login step falls through to the normal `hasDemo` branch (which renders an empty Configure/DriveRequestPane, not `LoginPane`); the `returnTo` test fails because nothing reads `window.location.search`

- [ ] **Step 3: Write the implementation**

In `web/src/App.tsx`, add the `LoginPane` import:

```tsx
import { useEffect, useState } from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { fetchConfig, fetchScenarios, type Pillar } from "./lib/api";
import { initTheme } from "./lib/theme";
import { estimateCostFromResponseBody } from "./lib/cost";
import { Breadcrumb } from "./components/Breadcrumb";
import { ExplanationBand } from "./components/ExplanationBand";
import { ThemeToggle } from "./components/ThemeToggle";
import { CostTicker } from "./components/CostTicker";
import { ConfigurePane } from "./components/ConfigurePane";
import { DriveRequestPane } from "./components/DriveRequestPane";
import { AgentRunPane } from "./components/AgentRunPane";
import { VirtualKeyPane } from "./components/VirtualKeyPane";
import { WelcomeFeatures } from "./components/WelcomeFeatures";
import { LoginPane } from "./components/LoginPane";
```

Add a second `useEffect` right after the existing fetch-on-mount effect, to restore the pillar a login redirect sent the browser back for:

```tsx
  useEffect(() => {
    initTheme();
    fetchScenarios()
      .then(setPillars)
      .catch((err) =>
        setScenariosError(err instanceof Error ? err.message : String(err)),
      );
    fetchConfig()
      .then((config) => {
        setMissingEnvVars(config.missing);
        setVersion(config.version);
      })
      .catch((err) => console.error("failed to load config", err));
  }, []);

  // Additive: a login redirect (see LoginPane) sends the browser back to
  // "/?returnTo=<pillar id>" instead of the default welcome screen - this
  // only fires once pillars are loaded, since it needs to find the matching
  // index.
  useEffect(() => {
    if (pillars.length === 0) return;
    const returnTo = new URLSearchParams(window.location.search).get(
      "returnTo",
    );
    if (!returnTo) return;
    const index = pillars.findIndex((p) => p.id === returnTo);
    if (index >= 0) {
      setActivePillarIndex(index);
      setActiveStepIndex(0);
    }
  }, [pillars]);
```

Add `loginDemo` to the `hasDemo` check:

```tsx
  const activePillar = pillars[activePillarIndex];
  const activeStep = activePillar.steps[activeStepIndex];
  const hasDemo =
    activeStep.agentDemo ||
    activeStep.loginDemo ||
    activeStep.policies.length > 0 ||
    activeStep.presets.length > 0;
```

Split the demo-rendering block so `LoginPane` renders full-width instead of squeezed into the Configure/Request two-column grid:

```tsx
        {activeStep.loginDemo && (
          <div className="px-6 py-6">
            <LoginPane
              key={`${activeStep.id}-login`}
              identities={["team-alpha", "team-beta"]}
            />
          </div>
        )}
        {hasDemo && !activeStep.loginDemo && (
          <div className="grid grid-cols-2 gap-6 px-6 py-6">
            <ConfigurePane
              key={`${activeStep.id}-configure`}
              policies={activeStep.policies}
            />
            {activeStep.agentDemo ? (
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
          </div>
        )}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd web && npx vitest run`
Expected: PASS (all frontend tests, including the 2 new ones)

- [ ] **Step 5: Commit**

```bash
git add web/src/App.tsx web/src/App.test.tsx
git commit -m "feat(ui): render LoginPane for the login step, restore pillar after redirect"
```

---

### Task 16: Full-repo verification pass

**Files:** none (verification only)

- [ ] **Step 1: Run the Go build and test suite**

Run: `go build ./... && go test ./... -v`
Expected: PASS across every package touched by Tasks 1-12

- [ ] **Step 2: Run the frontend test suite and typecheck**

Run: `cd web && npx vitest run && npx tsc -b`
Expected: PASS

- [ ] **Step 3: Run lint and format checks**

Run: `cd web && npm run lint && npm run format:check`
Expected: PASS

- [ ] **Step 4: Note what this plan cannot verify automatically**

The actual browser Authorization Code + PKCE redirect dance against a real Keycloak, and the STS Delegation exchange against a real Solo STS, aren't mockable in `vitest`/Go unit tests - they require a live deployment to exercise end to end. This is a manual/live verification step, consistent with how this repo has handled other live-infrastructure-dependent changes. Two prerequisites live outside this repo, in `agentgateway-field-kit`'s own working tree, before Steps 3-4 can be demoed live:

- Step 3 (Standard exchange) needs no new field-kit infrastructure - it reuses the already-provisioned `agw-token-exchange` Keycloak client.
- Step 4 (STS exchange) needs Postgres deployed and the `tokenExchange.*` Helm values enabled, with `subjectValidator`/`actorValidator`/`apiValidator` configured - tracked as follow-up work in that sibling repo, not part of this plan.

If any of Steps 1-3 fail, fix the issue in the task that owns the failing file and re-run that task's own test command before re-running the full suite.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-10-09-auth-pillar-token-exchange.md`. Two execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
