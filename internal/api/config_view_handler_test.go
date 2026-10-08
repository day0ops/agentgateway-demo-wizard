package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/day0ops/agentgateway-demo-wizard/internal/k8s"
	"github.com/day0ops/agentgateway-demo-wizard/internal/k8s/k8stest"
	"github.com/day0ops/agentgateway-demo-wizard/internal/scenarios"
)

var testBackendGVR = schema.GroupVersionResource{
	Group:    "enterpriseagentgateway.solo.io",
	Version:  "v1alpha1",
	Resource: "enterpriseagentgatewaybackends",
}

// newApplierForViewTest is newApplierForTest (config_apply_handler_test.go)
// plus a REST mapping for EnterpriseAgentgatewayBackend: the shared helper's
// mapper only knows ConfigMap, and seeding a backend fixture below via
// applier.Apply needs to resolve that kind to testBackendGVR. k8s.Applier.Get
// - what the handler itself calls - takes a GVR directly and never consults
// the mapper, so production code needs no equivalent.
func newApplierForViewTest() *k8s.Applier {
	dyn := k8stest.WithApplyFallback(dynamicfake.NewSimpleDynamicClient(scheme.Scheme))
	mapper := meta.NewDefaultRESTMapper(nil)
	mapper.Add(schema.GroupVersionKind{Group: testBackendGVR.Group, Version: testBackendGVR.Version, Kind: "EnterpriseAgentgatewayBackend"}, meta.RESTScopeNamespace)
	return k8s.NewApplier(dyn, mapper)
}

func testRegistryWithReadOnlyPolicy() []scenarios.Pillar {
	return []scenarios.Pillar{{
		ID: "test", Title: "Test",
		Steps: []scenarios.Step{{
			ID: "test-step", Title: "Test step",
			Policies: []scenarios.Policy{{
				ID:       "test-view",
				Title:    "Test view policy",
				ReadOnly: true,
				ViewRefs: []scenarios.ViewRef{
					{GVR: testBackendGVR, Name: "present-backend"},
					{GVR: testBackendGVR, Name: "missing-backend"},
				},
			}},
		}},
	}}
}

func TestConfigViewReturnsFoundAndNotFoundRefs(t *testing.T) {
	applier := newApplierForViewTest()
	// Seed one of the two ViewRefs directly via Apply so it exists in the fake
	// dynamic client; the other is deliberately left absent.
	backendDoc := `
apiVersion: enterpriseagentgateway.solo.io/v1alpha1
kind: EnterpriseAgentgatewayBackend
metadata:
  name: present-backend
  namespace: agentgateway-system
spec:
  ai:
    provider:
      openai:
        model: gpt-4o-mini
`
	if _, err := applier.Apply(context.Background(), "seed", [][]byte{[]byte(backendDoc)}); err != nil {
		t.Fatalf("seeding fixture: %v", err)
	}

	s := NewServer(WithScenarios(testRegistryWithReadOnlyPolicy()), WithApplier(applier))

	req := httptest.NewRequest(http.MethodGet, "/api/config/view?policyId=test-view", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"name":"present-backend"`) || !strings.Contains(body, `"found":true`) {
		t.Fatalf("expected present-backend found:true, got: %s", body)
	}
	if !strings.Contains(body, `"name":"missing-backend"`) || !strings.Contains(body, `"found":false`) {
		t.Fatalf("expected missing-backend found:false, got: %s", body)
	}
	if !strings.Contains(body, "gpt-4o-mini") {
		t.Fatalf("expected the found backend's YAML to include its spec, got: %s", body)
	}
}

func TestConfigViewOmitsServerManagedNoiseFromYAML(t *testing.T) {
	applier := newApplierForViewTest()
	// managedFields/resourceVersion/uid/status aren't things a caller sets via
	// Apply against a real API server either - they're server-assigned - but
	// seeding them directly on the fixture is the simplest way to exercise the
	// same object shape a real SSA response would have, since the fake dynamic
	// client's Apply doesn't synthesize them itself.
	backendDoc := `
apiVersion: enterpriseagentgateway.solo.io/v1alpha1
kind: EnterpriseAgentgatewayBackend
metadata:
  name: present-backend
  namespace: agentgateway-system
  resourceVersion: "12345"
  uid: abc-123-def
  generation: 2
  managedFields:
    - manager: kubectl
      operation: Apply
status:
  conditions:
    - type: Ready
      status: "True"
spec:
  ai:
    provider:
      openai:
        model: gpt-4o-mini
`
	if _, err := applier.Apply(context.Background(), "seed", [][]byte{[]byte(backendDoc)}); err != nil {
		t.Fatalf("seeding fixture: %v", err)
	}

	s := NewServer(WithScenarios(testRegistryWithReadOnlyPolicy()), WithApplier(applier))

	req := httptest.NewRequest(http.MethodGet, "/api/config/view?policyId=test-view", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, noisy := range []string{"managedFields", "resourceVersion", "uid", "generation", "status"} {
		if strings.Contains(body, noisy) {
			t.Fatalf("expected %q to be stripped from the view response, got: %s", noisy, body)
		}
	}
	if !strings.Contains(body, "gpt-4o-mini") {
		t.Fatalf("expected the backend's spec to survive stripping, got: %s", body)
	}
}

func TestStripAppliedStateRemovesServerManagedNoise(t *testing.T) {
	obj := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":              "foo",
			"namespace":         "demo",
			"resourceVersion":   "12345",
			"uid":               "abc-123",
			"generation":        int64(2),
			"creationTimestamp": "2026-10-08T00:00:00Z",
			"managedFields":     []interface{}{map[string]interface{}{"manager": "kubectl"}},
		},
		"status": map[string]interface{}{"phase": "Active"},
		"data":   map[string]interface{}{"key": "value"},
	}

	stripAppliedState(obj)

	metadata, _ := obj["metadata"].(map[string]interface{})
	for _, noisy := range []string{"resourceVersion", "uid", "generation", "creationTimestamp", "managedFields"} {
		if _, present := metadata[noisy]; present {
			t.Fatalf("expected metadata.%s to be stripped, got: %v", noisy, metadata)
		}
	}
	if _, present := obj["status"]; present {
		t.Fatalf("expected status to be stripped, got: %v", obj)
	}
	if metadata["name"] != "foo" || metadata["namespace"] != "demo" {
		t.Fatalf("expected name/namespace preserved, got: %v", metadata)
	}
	if data, _ := obj["data"].(map[string]interface{}); data["key"] != "value" {
		t.Fatalf("expected spec/data fields preserved, got: %v", obj)
	}
}

func TestConfigViewUnknownPolicyReturns404(t *testing.T) {
	applier := newApplierForTest()
	s := NewServer(WithScenarios(testRegistryWithReadOnlyPolicy()), WithApplier(applier))

	req := httptest.NewRequest(http.MethodGet, "/api/config/view?policyId=does-not-exist", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestConfigViewRejectsNonReadOnlyPolicy(t *testing.T) {
	applier := newApplierForTest()
	s := NewServer(WithScenarios(testRegistryWithFixturePolicy()), WithApplier(applier)) // "test-fixture", ReadOnly: false

	req := httptest.NewRequest(http.MethodGet, "/api/config/view?policyId=test-fixture", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a non-read-only policy, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestConfigViewNeverReturnsASecretKind(t *testing.T) {
	// Defense-in-depth: even if a future policy's ViewRefs mistakenly named a
	// Secret, the handler must refuse rather than leak it. This asserts the
	// guard exists, not just that nobody has used it wrong yet.
	applier := newApplierForTest()
	secretGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	reg := []scenarios.Pillar{{
		ID: "test", Steps: []scenarios.Step{{
			ID: "test-step",
			Policies: []scenarios.Policy{{
				ID: "test-secret-view", ReadOnly: true,
				ViewRefs: []scenarios.ViewRef{{GVR: secretGVR, Name: "whatever"}},
			}},
		}},
	}}
	s := NewServer(WithScenarios(reg), WithApplier(applier))

	req := httptest.NewRequest(http.MethodGet, "/api/config/view?policyId=test-secret-view", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 refusing to view a Secret-kind ref, got %d: %s", rec.Code, rec.Body.String())
	}
}
