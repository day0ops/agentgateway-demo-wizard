package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// decodeJWTClaim extracts one string claim from a JWT's payload segment
// without validating the signature - agentgateway's ext-auth has already
// validated this token before forwarding it as the Authorization header;
// this only reads a claim out of a token already trusted by the caller.
func decodeJWTClaim(bearer, claim string) (string, error) {
	token := strings.TrimPrefix(bearer, "Bearer ")
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("malformed JWT: expected 3 dot-separated segments, got %d", len(parts))
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decoding JWT payload: %w", err)
	}

	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("parsing JWT payload: %w", err)
	}

	v, ok := claims[claim]
	if !ok {
		return "", fmt.Errorf("claim %q not present in token", claim)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("claim %q is not a string", claim)
	}
	return s, nil
}
