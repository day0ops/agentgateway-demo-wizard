package api

import (
	"encoding/json"
	"net/http"

	"github.com/day0ops/agentgateway-demo-wizard/internal/gateway"
)

// WithGateway registers POST /api/request, backed by client.
func WithGateway(client *gateway.Client) Option {
	return func(s *Server) {
		s.gatewayClient = client
	}
}

type driveRequest struct {
	Identity string            `json:"identity"`
	Method   string            `json:"method"`
	Path     string            `json:"path"`
	Headers  map[string]string `json:"headers"`
	Body     string            `json:"body"`
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
