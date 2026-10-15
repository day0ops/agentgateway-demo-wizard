package api

import (
	"fmt"
	"net/http"
)

type secureSessionResponse struct {
	Identity string `json:"identity"`
}

// handleSecureSession is reached only once agentgateway's ext-auth has
// already let the request through - meaning a session exists and the
// Authorization header carries a real token. The frontend uses this to show
// who's logged in; it has no other way to read the session, since the
// cookie ext-auth sets is httpOnly.
func (s *Server) handleSecureSession(w http.ResponseWriter, r *http.Request) {
	bearer := r.Header.Get("Authorization")
	if bearer == "" {
		writeJSONError(w, http.StatusUnauthorized, fmt.Errorf("not logged in"))
		return
	}
	identity, err := decodeJWTClaim(bearer, "preferred_username")
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, err)
		return
	}
	writeJSON(w, http.StatusOK, secureSessionResponse{Identity: identity})
}
