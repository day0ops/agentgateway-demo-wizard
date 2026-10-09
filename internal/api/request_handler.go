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
