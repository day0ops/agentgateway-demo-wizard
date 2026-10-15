package api

import "testing"

func TestDecodeJWTClaimExtractsPreferredUsername(t *testing.T) {
	// {"preferred_username":"team-alpha"} base64url-encoded, no signature validation.
	payload := "eyJwcmVmZXJyZWRfdXNlcm5hbWUiOiJ0ZWFtLWFscGhhIn0"
	token := "header." + payload + ".signature"

	got, err := decodeJWTClaim(token, "preferred_username")
	if err != nil {
		t.Fatalf("decodeJWTClaim: %v", err)
	}
	if got != "team-alpha" {
		t.Fatalf("expected team-alpha, got %q", got)
	}
}

func TestDecodeJWTClaimRejectsMalformedToken(t *testing.T) {
	if _, err := decodeJWTClaim("not-a-jwt", "preferred_username"); err == nil {
		t.Fatalf("expected an error for a malformed token")
	}
}

func TestDecodeJWTClaimRejectsMissingClaim(t *testing.T) {
	payload := "eyJzdWIiOiJzb21lLXN1YiJ9" // {"sub":"some-sub"}, no preferred_username
	token := "header." + payload + ".signature"
	if _, err := decodeJWTClaim(token, "preferred_username"); err == nil {
		t.Fatalf("expected an error when the claim is absent")
	}
}
