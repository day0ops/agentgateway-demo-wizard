package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/day0ops/agentgateway-demo-wizard/internal/k8s"
	"github.com/day0ops/agentgateway-demo-wizard/internal/k8s/k8stest"
	"github.com/day0ops/agentgateway-demo-wizard/internal/scenarios"
)

func newApplierForTest() *k8s.Applier {
	dyn := k8stest.WithApplyFallback(dynamicfake.NewSimpleDynamicClient(scheme.Scheme))
	mapper := meta.NewDefaultRESTMapper(nil)
	mapper.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)
	return k8s.NewApplier(dyn, mapper)
}

func testRegistryWithFixturePolicy() []scenarios.Pillar {
	return []scenarios.Pillar{{
		ID: "test", Title: "Test",
		Steps: []scenarios.Step{{
			ID: "test-step", Title: "Test step",
			Policies: []scenarios.Policy{{ID: "test-fixture", Title: "Test fixture", ManifestPath: "testfixture/backend.yaml.tmpl"}},
		}},
	}}
}

func TestConfigApplyStreamsDoneEvent(t *testing.T) {
	applier := newApplierForTest()
	s := NewServer(WithScenarios(testRegistryWithFixturePolicy()), WithApplier(applier))

	req := httptest.NewRequest(http.MethodPost, "/api/config/apply", strings.NewReader(`{"policyId":"test-fixture"}`))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"type":"done"`) {
		t.Fatalf("expected a done event, got: %s", rec.Body.String())
	}
	if !applier.IsApplied("test-fixture") {
		t.Fatalf("expected the policy to be marked applied")
	}
}

func TestConfigApplyUnknownPolicyReturns404(t *testing.T) {
	applier := newApplierForTest()
	s := NewServer(WithScenarios(testRegistryWithFixturePolicy()), WithApplier(applier))

	req := httptest.NewRequest(http.MethodPost, "/api/config/apply", strings.NewReader(`{"policyId":"does-not-exist"}`))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestConfigApplyReadOnlyPolicyReturns400(t *testing.T) {
	applier := newApplierForTest()
	s := NewServer(WithScenarios(testRegistryWithReadOnlyPolicy()), WithApplier(applier))

	req := httptest.NewRequest(http.MethodPost, "/api/config/apply", strings.NewReader(`{"policyId":"test-view"}`))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a read-only policy, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestConfigRevertReadOnlyPolicyReturns400(t *testing.T) {
	applier := newApplierForTest()
	s := NewServer(WithScenarios(testRegistryWithReadOnlyPolicy()), WithApplier(applier))

	req := httptest.NewRequest(http.MethodPost, "/api/config/revert", strings.NewReader(`{"policyId":"test-view"}`))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a read-only policy, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestConfigRevertThenReset(t *testing.T) {
	applier := newApplierForTest()
	s := NewServer(WithScenarios(testRegistryWithFixturePolicy()), WithApplier(applier))

	applyReq := httptest.NewRequest(http.MethodPost, "/api/config/apply", strings.NewReader(`{"policyId":"test-fixture"}`))
	s.ServeHTTP(httptest.NewRecorder(), applyReq)
	if !applier.IsApplied("test-fixture") {
		t.Fatalf("setup: expected policy to be applied")
	}

	revertReq := httptest.NewRequest(http.MethodPost, "/api/config/revert", strings.NewReader(`{"policyId":"test-fixture"}`))
	revertRec := httptest.NewRecorder()
	s.ServeHTTP(revertRec, revertReq)
	if revertRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", revertRec.Code, revertRec.Body.String())
	}
	if applier.IsApplied("test-fixture") {
		t.Fatalf("expected policy to be reverted")
	}

	s.ServeHTTP(httptest.NewRecorder(), applyReq)
	resetReq := httptest.NewRequest(http.MethodPost, "/api/config/reset", nil)
	resetRec := httptest.NewRecorder()
	s.ServeHTTP(resetRec, resetReq)
	if resetRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resetRec.Code, resetRec.Body.String())
	}
	if applier.IsApplied("test-fixture") {
		t.Fatalf("expected ResetAll to revert the policy")
	}
}

func TestConfigApplyHandlesPatchStylePolicies(t *testing.T) {
	applier := newApplierForTest()
	reg := []scenarios.Pillar{{
		ID: "test", Title: "Test",
		Steps: []scenarios.Step{{
			ID: "test-step", Title: "Test step",
			Policies: []scenarios.Policy{{
				ID:                      "patch-fixture",
				Title:                   "Patch fixture",
				PatchManifestPath:       "testfixture/patch.yaml.tmpl",
				PatchRevertManifestPath: "testfixture/patch-empty.yaml.tmpl",
			}},
		}},
	}}

	s := NewServer(WithScenarios(reg), WithApplier(applier))

	applyReq := httptest.NewRequest(http.MethodPost, "/api/config/apply", strings.NewReader(`{"policyId":"patch-fixture"}`))
	applyRec := httptest.NewRecorder()
	s.ServeHTTP(applyRec, applyReq)
	if applyRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", applyRec.Code, applyRec.Body.String())
	}
	if !applier.IsApplied("patch-fixture") {
		t.Fatalf("expected the policy to be marked applied")
	}

	revertReq := httptest.NewRequest(http.MethodPost, "/api/config/revert", strings.NewReader(`{"policyId":"patch-fixture"}`))
	revertRec := httptest.NewRecorder()
	s.ServeHTTP(revertRec, revertReq)
	if revertRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", revertRec.Code, revertRec.Body.String())
	}
	if applier.IsApplied("patch-fixture") {
		t.Fatalf("expected the policy to be reverted")
	}
}

func TestConfigResetHandlesPatchStylePolicies(t *testing.T) {
	applier := newApplierForTest()
	reg := []scenarios.Pillar{{
		ID: "test", Title: "Test",
		Steps: []scenarios.Step{{
			ID: "test-step", Title: "Test step",
			Policies: []scenarios.Policy{{
				ID:                      "patch-fixture",
				Title:                   "Patch fixture",
				PatchManifestPath:       "testfixture/patch.yaml.tmpl",
				PatchRevertManifestPath: "testfixture/patch-empty.yaml.tmpl",
			}},
		}},
	}}

	s := NewServer(WithScenarios(reg), WithApplier(applier))

	applyReq := httptest.NewRequest(http.MethodPost, "/api/config/apply", strings.NewReader(`{"policyId":"patch-fixture"}`))
	applyRec := httptest.NewRecorder()
	s.ServeHTTP(applyRec, applyReq)
	if applyRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", applyRec.Code, applyRec.Body.String())
	}
	if !applier.IsApplied("patch-fixture") {
		t.Fatalf("expected the policy to be marked applied")
	}

	resetReq := httptest.NewRequest(http.MethodPost, "/api/config/reset", nil)
	resetRec := httptest.NewRecorder()
	s.ServeHTTP(resetRec, resetReq)
	if resetRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resetRec.Code, resetRec.Body.String())
	}
	if applier.IsApplied("patch-fixture") {
		t.Fatalf("expected handleConfigReset's patch-release loop to revert the policy")
	}
}

func deploymentFixtureRegistry() []scenarios.Pillar {
	return []scenarios.Pillar{{
		ID: "test", Title: "Test",
		Steps: []scenarios.Step{{
			ID: "test-step", Title: "Test step",
			Policies: []scenarios.Policy{{ID: "deployment-fixture", Title: "Deployment fixture", ManifestPath: "testfixture/deployment.yaml.tmpl"}},
		}},
	}}
}

func newApplierForDeploymentTest() (dynamic.Interface, *k8s.Applier) {
	dyn := k8stest.WithApplyFallback(dynamicfake.NewSimpleDynamicClient(scheme.Scheme))
	mapper := meta.NewDefaultRESTMapper(nil)
	mapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
	return dyn, k8s.NewApplier(dyn, mapper)
}

func TestConfigApplyWaitsForDeploymentReadinessAndReportsTimeout(t *testing.T) {
	_, applier := newApplierForDeploymentTest()
	s := NewServer(WithScenarios(deploymentFixtureRegistry()), WithApplier(applier), WithDeploymentReadyTimeout(time.Second))

	req := httptest.NewRequest(http.MethodPost, "/api/config/apply", strings.NewReader(`{"policyId":"deployment-fixture"}`))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `"type":"progress"`) || !strings.Contains(body, "Waiting for") {
		t.Fatalf("expected a progress event mentioning waiting, got: %s", body)
	}
	if !strings.Contains(body, `"type":"error"`) {
		t.Fatalf("expected an error event since the deployment never becomes ready, got: %s", body)
	}
	if strings.Contains(body, `"type":"done"`) {
		t.Fatalf("expected no done event since the deployment never becomes ready, got: %s", body)
	}
}

func TestConfigApplyReachesDoneWhenDeploymentAlreadyReady(t *testing.T) {
	dyn, applier := newApplierForDeploymentTest()

	deploymentGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	ready := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": "test-fixture-deployment", "namespace": "agentgateway-system"},
		"spec":       map[string]interface{}{"replicas": int64(1)},
		"status":     map[string]interface{}{"readyReplicas": int64(1)},
	}}
	if _, err := dyn.Resource(deploymentGVR).Namespace("agentgateway-system").Create(context.Background(), ready, metav1.CreateOptions{}); err != nil {
		t.Fatalf("seeding a ready Deployment: %v", err)
	}

	s := NewServer(WithScenarios(deploymentFixtureRegistry()), WithApplier(applier), WithDeploymentReadyTimeout(time.Second))

	req := httptest.NewRequest(http.MethodPost, "/api/config/apply", strings.NewReader(`{"policyId":"deployment-fixture"}`))
	rec := httptest.NewRecorder()
	start := time.Now()
	s.ServeHTTP(rec, req)
	elapsed := time.Since(start)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"type":"done"`) {
		t.Fatalf("expected a done event, got: %s", rec.Body.String())
	}
	if elapsed >= 2*time.Second {
		t.Fatalf("expected handleConfigApply to reach done without waiting out a full poll interval, took %s", elapsed)
	}
}
