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
