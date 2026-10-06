package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/day0ops/agentgateway-demo-wizard/internal/k8s"
	"github.com/day0ops/agentgateway-demo-wizard/internal/k8s/k8stest"
)

func newApplierForVirtualKeysTest() *k8s.Applier {
	dyn := k8stest.WithApplyFallback(dynamicfake.NewSimpleDynamicClient(scheme.Scheme))
	mapper := meta.NewDefaultRESTMapper(nil)
	mapper.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"}, meta.RESTScopeNamespace)
	mapper.Add(schema.GroupVersionKind{Group: "ratelimit.solo.io", Version: "v1alpha1", Kind: "RateLimitConfig"}, meta.RESTScopeNamespace)
	mapper.Add(schema.GroupVersionKind{Group: "enterpriseagentgateway.solo.io", Version: "v1alpha1", Kind: "EnterpriseAgentgatewayBudget"}, meta.RESTScopeNamespace)
	return k8s.NewApplier(dyn, mapper)
}

const virtualKeysSecretFixture = `
apiVersion: v1
kind: Secret
metadata:
  name: cost-budgets-virtual-keys
  namespace: agentgateway-system
type: Opaque
stringData:
  team-alpha: '{"key":"sk-team-alpha-demo-budget","metadata":{"id":"team-alpha","user_id":"team-alpha"}}'
`

const rateLimitConfigFixture = `
apiVersion: ratelimit.solo.io/v1alpha1
kind: RateLimitConfig
metadata:
  name: cost-budgets-ratelimit
  namespace: agentgateway-system
spec:
  raw:
    descriptors:
      - key: user_id
        value: team-alpha
        rateLimit:
          unit: HOUR
          requestsPerUnit: 100000
    rateLimits:
      - actions:
          - cel:
              expression: apiKey.user_id
              key: user_id
        type: TOKEN
`

const budgetFixture = `
apiVersion: enterpriseagentgateway.solo.io/v1alpha1
kind: EnterpriseAgentgatewayBudget
metadata:
  name: cost-budgets
  namespace: agentgateway-system
spec:
  budgets:
    - name: team-alpha-daily-tokens
      subject:
        virtualKey: team-alpha
      limit:
        unit: Tokens
        amount: 1
      window:
        unit: Day
      onBudgetExceeded: Audit
`

func seedVirtualKeysFixtures(t *testing.T, applier *k8s.Applier) {
	t.Helper()
	ctx := context.Background()
	if _, err := applier.Apply(ctx, "cost-budgets", [][]byte{[]byte(virtualKeysSecretFixture)}); err != nil {
		t.Fatalf("seeding Secret: %v", err)
	}
	if _, err := applier.Apply(ctx, "cost-budgets", [][]byte{[]byte(rateLimitConfigFixture)}); err != nil {
		t.Fatalf("seeding RateLimitConfig: %v", err)
	}
	if _, err := applier.Apply(ctx, "cost-budgets", [][]byte{[]byte(budgetFixture)}); err != nil {
		t.Fatalf("seeding Budget: %v", err)
	}
}

// TestVirtualKeyCreateAddsEntryWithoutDroppingExisting is the test this
// plan's read-modify-write design exists for: a naive partial-object SSA
// apply on the list fields below would silently replace team-alpha's
// existing descriptor/budget/secret entry with just the new one.
func TestVirtualKeyCreateAddsEntryWithoutDroppingExisting(t *testing.T) {
	applier := newApplierForVirtualKeysTest()
	seedVirtualKeysFixtures(t, applier)
	s := NewServer(WithApplier(applier))

	req := httptest.NewRequest(http.MethodPost, "/api/virtual-keys", strings.NewReader(`{"name":"team-gamma","userId":"team-gamma","tokenBudget":5000}`))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"name":"team-gamma"`) {
		t.Fatalf("expected the new key's name in the response, got: %s", rec.Body.String())
	}

	secretGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	secretObj, err := applier.Get(context.Background(), secretGVR, "agentgateway-system", "cost-budgets-virtual-keys")
	if err != nil {
		t.Fatalf("Get Secret: %v", err)
	}
	// The fake dynamic client doesn't replicate a real apiserver's
	// Secret-specific admission behavior (converting stringData into
	// base64 data and clearing stringData), so stringData is what's
	// actually populated here - the same field the handler itself writes.
	data, _, _ := unstructured.NestedStringMap(secretObj.Object, "stringData")
	if _, ok := data["team-alpha"]; !ok {
		t.Fatalf("expected team-alpha's existing Secret entry to survive, got stringData keys: %v", data)
	}
	if _, ok := data["team-gamma"]; !ok {
		t.Fatalf("expected team-gamma's new Secret entry to be added, got stringData keys: %v", data)
	}

	rateLimitGVR := schema.GroupVersionResource{Group: "ratelimit.solo.io", Version: "v1alpha1", Resource: "ratelimitconfigs"}
	rlObj, err := applier.Get(context.Background(), rateLimitGVR, "agentgateway-system", "cost-budgets-ratelimit")
	if err != nil {
		t.Fatalf("Get RateLimitConfig: %v", err)
	}
	descriptors, _, _ := unstructured.NestedSlice(rlObj.Object, "spec", "raw", "descriptors")
	if len(descriptors) != 2 {
		t.Fatalf("expected 2 descriptors (team-alpha survives + team-gamma added), got %d: %v", len(descriptors), descriptors)
	}

	budgetGVR := schema.GroupVersionResource{Group: "enterpriseagentgateway.solo.io", Version: "v1alpha1", Resource: "enterpriseagentgatewaybudgets"}
	budgetObj, err := applier.Get(context.Background(), budgetGVR, "agentgateway-system", "cost-budgets")
	if err != nil {
		t.Fatalf("Get Budget: %v", err)
	}
	budgets, _, _ := unstructured.NestedSlice(budgetObj.Object, "spec", "budgets")
	if len(budgets) != 2 {
		t.Fatalf("expected 2 budgets (team-alpha survives + team-gamma added), got %d: %v", len(budgets), budgets)
	}
}

func TestVirtualKeyCreateRejectsMissingFields(t *testing.T) {
	applier := newApplierForVirtualKeysTest()
	s := NewServer(WithApplier(applier))

	req := httptest.NewRequest(http.MethodPost, "/api/virtual-keys", strings.NewReader(`{"name":"","userId":"","tokenBudget":0}`))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestVirtualKeyCreateErrorsWhenBudgetsPolicyNotYetApplied(t *testing.T) {
	applier := newApplierForVirtualKeysTest()
	s := NewServer(WithApplier(applier))

	req := httptest.NewRequest(http.MethodPost, "/api/virtual-keys", strings.NewReader(`{"name":"team-gamma","userId":"team-gamma","tokenBudget":5000}`))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when the budgets policy hasn't been applied yet, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "apply the Budgets and spend limits policy first") {
		t.Fatalf("expected a helpful error message, got: %s", rec.Body.String())
	}
}

func TestVirtualKeyRotateChangesKeyValueButKeepsUserIDAndOtherKeys(t *testing.T) {
	applier := newApplierForVirtualKeysTest()
	seedVirtualKeysFixtures(t, applier)
	s := NewServer(WithApplier(applier))

	req := httptest.NewRequest(http.MethodPost, "/api/virtual-keys/team-alpha/rotate", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "sk-team-alpha-demo-budget") {
		t.Fatalf("expected a newly generated key value, not the old one, got: %s", rec.Body.String())
	}

	secretGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	secretObj, err := applier.Get(context.Background(), secretGVR, "agentgateway-system", "cost-budgets-virtual-keys")
	if err != nil {
		t.Fatalf("Get Secret: %v", err)
	}
	// The fake dynamic client doesn't replicate a real apiserver's
	// Secret-specific admission behavior (converting stringData into base64
	// data and clearing stringData), so stringData is what's actually
	// populated here - the same field the handler itself writes - and it's
	// plaintext, not base64.
	data, _, _ := unstructured.NestedStringMap(secretObj.Object, "stringData")
	decoded, ok := data["team-alpha"]
	if !ok {
		t.Fatalf("expected team-alpha's entry to still exist after rotation")
	}
	if strings.Contains(decoded, "sk-team-alpha-demo-budget") {
		t.Fatalf("expected the stored key value to have changed, got: %s", decoded)
	}
	if !strings.Contains(decoded, `"user_id":"team-alpha"`) {
		t.Fatalf("expected user_id metadata to be preserved across rotation, got: %s", decoded)
	}
}

func TestVirtualKeyRotateErrorsForUnknownKey(t *testing.T) {
	applier := newApplierForVirtualKeysTest()
	seedVirtualKeysFixtures(t, applier)
	s := NewServer(WithApplier(applier))

	req := httptest.NewRequest(http.MethodPost, "/api/virtual-keys/does-not-exist/rotate", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	// writeJSONError JSON-encodes the message, so a raw substring check
	// against rec.Body would need to match backslash-escaped quotes; decode
	// it first so the check is against the actual error text.
	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("decoding error response: %v", err)
	}
	if !strings.Contains(errResp.Error, `no virtual key named "does-not-exist"`) {
		t.Fatalf("expected a helpful error message, got: %s", errResp.Error)
	}
}
