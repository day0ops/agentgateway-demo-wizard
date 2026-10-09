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
//
// Token type and header requirements were confirmed live against a real
// deployment, not just RFC 8693's text: this STS rejects
// urn:ietf:params:oauth:token-type:access_token with "unsupported token
// type" for both subject and actor tokens - only the generic "jwt" token
// type is accepted - and it also requires the subject token to be repeated
// as the request's own Authorization header, not just the subject_token
// form field.
func ExchangeViaSTS(ctx context.Context, httpClient *http.Client, stsHost, subjectToken, actorToken string) (string, error) {
	form := url.Values{
		"grant_type":         {"urn:ietf:params:oauth:grant-type:token-exchange"},
		"subject_token":      {subjectToken},
		"subject_token_type": {"urn:ietf:params:oauth:token-type:jwt"},
		"actor_token":        {actorToken},
		"actor_token_type":   {"urn:ietf:params:oauth:token-type:jwt"},
		"audience":           {"agentgateway"},
	}

	endpoint := "http://" + strings.TrimRight(stsHost, "/") + "/oauth2/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("building STS exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+subjectToken)

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
