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
		// prompt=login forces Keycloak to show its login form even if an SSO
		// session cookie already exists - without it, switching identities
		// (e.g. team-alpha -> team-beta) silently re-authenticates as the
		// previous user, filing the wrong team's token under the new identity.
		"prompt": {"login"},
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
// needed. The wizard is only ever reached through a TLS-terminating
// ingress in practice, so the scheme is always https.
func (s *Server) authCallbackURL(r *http.Request) string {
	return "https://" + r.Host + "/auth/callback"
}

func (s *Server) sessionIDFromCookie(r *http.Request) string {
	c, err := r.Cookie(oauthlogin.SessionCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}
