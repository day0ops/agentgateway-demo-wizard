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
