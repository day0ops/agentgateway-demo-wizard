package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

var (
	secretGVR    = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	rateLimitGVR = schema.GroupVersionResource{Group: "ratelimit.solo.io", Version: "v1alpha1", Resource: "ratelimitconfigs"}
	budgetGVR    = schema.GroupVersionResource{Group: "enterpriseagentgateway.solo.io", Version: "v1alpha1", Resource: "enterpriseagentgatewaybudgets"}
)

// These names match internal/manifests/templates/cost/budgets.yaml.tmpl's
// own resource names exactly - virtual-key lifecycle extends that step's
// existing resources, it doesn't own a separate set.
const (
	virtualKeysSecretName = "cost-budgets-virtual-keys"
	virtualKeysRateLimit  = "cost-budgets-ratelimit"
	virtualKeysBudgetName = "cost-budgets"
	virtualKeysPolicyID   = "cost-budgets"
)

type createVirtualKeyRequest struct {
	Name        string `json:"name"`
	UserID      string `json:"userId"`
	TokenBudget int64  `json:"tokenBudget"`
}

type virtualKeyResponse struct {
	Name   string `json:"name"`
	Key    string `json:"key"`
	UserID string `json:"userId"`
}

func randomKeySuffix() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating random key suffix: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func stringMapToAny(m map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (s *Server) handleVirtualKeyCreate(w http.ResponseWriter, r *http.Request) {
	var req createVirtualKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.UserID == "" || req.TokenBudget <= 0 {
		http.Error(w, "name, userId, and a positive tokenBudget are required", http.StatusBadRequest)
		return
	}

	namespace := s.manifestParams().Namespace

	suffix, err := randomKeySuffix()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	key := fmt.Sprintf("sk-%s-%s", req.Name, suffix)

	if err := s.addVirtualKeyToSecret(r.Context(), namespace, req.Name, key, req.UserID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.addVirtualKeyToRateLimit(r.Context(), namespace, req.UserID, req.TokenBudget); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.addVirtualKeyToBudget(r.Context(), namespace, req.Name, req.TokenBudget); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, virtualKeyResponse{Name: req.Name, Key: key, UserID: req.UserID})
}

// decodeExistingSecretEntries reads obj's existing entries back into
// plaintext JSON strings, so a new entry can be merged in and the whole map
// re-submitted as stringData. A real apiserver persists stringData as
// base64-encoded data and clears stringData on write, but the fake dynamic
// client used in tests doesn't replicate that Secret-specific admission
// behavior, so a just-seeded fixture's entries surface under stringData
// instead; reading both (stringData taking precedence, matching the real
// API's own write-semantics) keeps this correct in both environments.
func decodeExistingSecretEntries(obj *unstructured.Unstructured) (map[string]string, error) {
	entries := map[string]string{}

	data, _, err := unstructured.NestedStringMap(obj.Object, "data")
	if err != nil {
		return nil, fmt.Errorf("reading existing Secret data: %w", err)
	}
	for k, v := range data {
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, fmt.Errorf("decoding existing Secret key %q: %w", k, err)
		}
		entries[k] = string(decoded)
	}

	stringData, _, err := unstructured.NestedStringMap(obj.Object, "stringData")
	if err != nil {
		return nil, fmt.Errorf("reading existing Secret stringData: %w", err)
	}
	for k, v := range stringData {
		entries[k] = v
	}

	return entries, nil
}

func (s *Server) addVirtualKeyToSecret(ctx context.Context, namespace, name, key, userID string) error {
	obj, err := s.applier.Get(ctx, secretGVR, namespace, virtualKeysSecretName)
	if err != nil {
		return fmt.Errorf("the %q Secret doesn't exist yet - apply the Budgets and spend limits policy first: %w", virtualKeysSecretName, err)
	}

	entries, err := decodeExistingSecretEntries(obj)
	if err != nil {
		return err
	}

	entryJSON, err := json.Marshal(map[string]any{
		"key":      key,
		"metadata": map[string]any{"id": userID, "user_id": userID},
	})
	if err != nil {
		return fmt.Errorf("marshaling new Secret entry: %w", err)
	}
	entries[name] = string(entryJSON)

	merged := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]any{
			"name":      virtualKeysSecretName,
			"namespace": namespace,
			"labels":    map[string]any{"app.kubernetes.io/managed-by": "agentgateway-demo-wizard"},
		},
		"type":       "Opaque",
		"stringData": stringMapToAny(entries),
	}}

	doc, err := yaml.Marshal(merged.Object)
	if err != nil {
		return fmt.Errorf("marshaling merged Secret: %w", err)
	}
	_, err = s.applier.Apply(ctx, virtualKeysPolicyID, [][]byte{doc})
	return err
}

func (s *Server) handleVirtualKeyRotate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		http.Error(w, "missing key name", http.StatusBadRequest)
		return
	}

	namespace := s.manifestParams().Namespace

	suffix, err := randomKeySuffix()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	newKey := fmt.Sprintf("sk-%s-%s", name, suffix)

	if err := s.rotateVirtualKeyInSecret(r.Context(), namespace, name, newKey); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, virtualKeyResponse{Name: name, Key: newKey})
}

func (s *Server) rotateVirtualKeyInSecret(ctx context.Context, namespace, name, newKey string) error {
	obj, err := s.applier.Get(ctx, secretGVR, namespace, virtualKeysSecretName)
	if err != nil {
		return fmt.Errorf("the %q Secret doesn't exist - nothing to rotate: %w", virtualKeysSecretName, err)
	}

	entries, err := decodeExistingSecretEntries(obj)
	if err != nil {
		return err
	}

	existingEntryJSON, ok := entries[name]
	if !ok {
		return fmt.Errorf("no virtual key named %q exists to rotate", name)
	}
	var entry struct {
		Key      string         `json:"key"`
		Metadata map[string]any `json:"metadata"`
	}
	if err := json.Unmarshal([]byte(existingEntryJSON), &entry); err != nil {
		return fmt.Errorf("parsing existing entry %q: %w", name, err)
	}
	entry.Key = newKey

	entryJSON, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling rotated entry: %w", err)
	}
	entries[name] = string(entryJSON)

	merged := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]any{
			"name":      virtualKeysSecretName,
			"namespace": namespace,
			"labels":    map[string]any{"app.kubernetes.io/managed-by": "agentgateway-demo-wizard"},
		},
		"type":       "Opaque",
		"stringData": stringMapToAny(entries),
	}}

	doc, err := yaml.Marshal(merged.Object)
	if err != nil {
		return fmt.Errorf("marshaling merged Secret: %w", err)
	}
	_, err = s.applier.Apply(ctx, virtualKeysPolicyID, [][]byte{doc})
	return err
}

func (s *Server) addVirtualKeyToRateLimit(ctx context.Context, namespace, userID string, tokenBudget int64) error {
	obj, err := s.applier.Get(ctx, rateLimitGVR, namespace, virtualKeysRateLimit)
	if err != nil {
		return fmt.Errorf("the %q RateLimitConfig doesn't exist yet - apply the Budgets and spend limits policy first: %w", virtualKeysRateLimit, err)
	}

	descriptors, _, err := unstructured.NestedSlice(obj.Object, "spec", "raw", "descriptors")
	if err != nil {
		return fmt.Errorf("reading existing descriptors: %w", err)
	}

	descriptors = append(descriptors, map[string]any{
		"key":   "user_id",
		"value": userID,
		"rateLimit": map[string]any{
			"unit":            "HOUR",
			"requestsPerUnit": tokenBudget,
		},
	})

	merged := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "ratelimit.solo.io/v1alpha1",
		"kind":       "RateLimitConfig",
		"metadata": map[string]any{
			"name":      virtualKeysRateLimit,
			"namespace": namespace,
			"labels":    map[string]any{"app.kubernetes.io/managed-by": "agentgateway-demo-wizard"},
		},
		"spec": map[string]any{
			"raw": map[string]any{
				"descriptors": descriptors,
				"rateLimits": []any{
					map[string]any{
						"actions": []any{
							map[string]any{"cel": map[string]any{"expression": "apiKey.user_id", "key": "user_id"}},
						},
						"type": "TOKEN",
					},
				},
			},
		},
	}}

	doc, err := yaml.Marshal(merged.Object)
	if err != nil {
		return fmt.Errorf("marshaling merged RateLimitConfig: %w", err)
	}
	_, err = s.applier.Apply(ctx, virtualKeysPolicyID, [][]byte{doc})
	return err
}

func (s *Server) addVirtualKeyToBudget(ctx context.Context, namespace, name string, tokenBudget int64) error {
	obj, err := s.applier.Get(ctx, budgetGVR, namespace, virtualKeysBudgetName)
	if err != nil {
		return fmt.Errorf("the %q Budget doesn't exist yet - apply the Budgets and spend limits policy first: %w", virtualKeysBudgetName, err)
	}

	budgets, _, err := unstructured.NestedSlice(obj.Object, "spec", "budgets")
	if err != nil {
		return fmt.Errorf("reading existing budgets: %w", err)
	}

	budgets = append(budgets, map[string]any{
		"name":             fmt.Sprintf("%s-daily-tokens", name),
		"subject":          map[string]any{"virtualKey": name},
		"limit":            map[string]any{"unit": "Tokens", "amount": tokenBudget},
		"window":           map[string]any{"unit": "Day"},
		"onBudgetExceeded": "Block",
	})

	merged := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "enterpriseagentgateway.solo.io/v1alpha1",
		"kind":       "EnterpriseAgentgatewayBudget",
		"metadata": map[string]any{
			"name":      virtualKeysBudgetName,
			"namespace": namespace,
			"labels":    map[string]any{"app.kubernetes.io/managed-by": "agentgateway-demo-wizard"},
		},
		"spec": map[string]any{"budgets": budgets},
	}}

	doc, err := yaml.Marshal(merged.Object)
	if err != nil {
		return fmt.Errorf("marshaling merged Budget: %w", err)
	}
	_, err = s.applier.Apply(ctx, virtualKeysPolicyID, [][]byte{doc})
	return err
}
