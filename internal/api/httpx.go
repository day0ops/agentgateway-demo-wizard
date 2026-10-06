package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type sseEvent struct {
	Type    string               `json:"type"`
	Message string               `json:"message"`
	Applied []appliedResourceDTO `json:"applied,omitempty"`
	YAML    string               `json:"yaml,omitempty"`
}

type appliedResourceDTO struct {
	Resource  string `json:"resource"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, event sseEvent) {
	data, _ := json.Marshal(event)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeJSONError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
