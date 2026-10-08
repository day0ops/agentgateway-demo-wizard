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
