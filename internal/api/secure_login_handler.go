package api

import (
	"net/http"
	"net/url"
)

// handleSecureLogin is reached only once agentgateway's ext-auth has
// already let the request through - meaning a session now exists. Its only
// job is sending the browser back into the wizard's own SPA at the right
// pillar: agentgateway owns the entire OIDC dance (redirect to Keycloak,
// callback, session banking) before this code ever runs.
func (s *Server) handleSecureLogin(w http.ResponseWriter, r *http.Request) {
	returnTo := r.URL.Query().Get("returnTo")
	http.Redirect(w, r, "/?returnTo="+url.QueryEscape(returnTo), http.StatusFound)
}
