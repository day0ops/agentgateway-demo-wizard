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
