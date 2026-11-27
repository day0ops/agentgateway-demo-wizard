package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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
	// Gateway selects which Gateway destination to proxy through: "" (the
	// default) uses the shared Gateway via the routing below, "tenants"
	// uses the dedicated tenants Gateway via ProxyTenant. Any other value
	// is rejected.
	Gateway string `json:"gateway"`
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

	// Gateway selects an explicit destination before the stream/MCP
	// branching below - it must run first because a presenter can
	// hand-edit the request body, so a tenants-destined request could
	// otherwise be misrouted through ProxyStream/ProxyMCP against the
	// wrong gateway.
	switch req.Gateway {
	case "":
		// Falls through to the existing routing below.
	case "tenants":
		if isStreamingRequest(req.Body) || strings.HasSuffix(req.Path, "/mcp") {
			writeJSONError(w, http.StatusBadRequest, fmt.Errorf("gateway %q does not support streaming or MCP requests", req.Gateway))
			return
		}
		resp, err := s.gatewayClient.ProxyTenant(r.Context(), gwReq)
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, driveResponse{StatusCode: resp.StatusCode, Body: resp.Body, LatencyMS: resp.LatencyMS})
		return
	default:
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("unrecognized gateway %q", req.Gateway))
		return
	}

	if isStreamingRequest(req.Body) {
		if err := s.gatewayClient.ProxyStream(r.Context(), gwReq, w); err != nil {
			writeJSONError(w, http.StatusBadGateway, err)
		}
		return
	}

	// The MCP Streamable HTTP transport requires an established session for
	// any non-initialize request - a preset's bare one-shot body needs the
	// handshake performed first, which the generic Proxy doesn't do.
	var resp *gateway.Response
	var err error
	if strings.HasSuffix(req.Path, "/mcp") {
		resp, err = s.gatewayClient.ProxyMCP(r.Context(), gwReq)
	} else {
		resp, err = s.gatewayClient.Proxy(r.Context(), gwReq)
	}
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err)
		return
	}

	writeJSON(w, http.StatusOK, driveResponse{StatusCode: resp.StatusCode, Body: resp.Body, LatencyMS: resp.LatencyMS})
}
