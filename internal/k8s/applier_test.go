package k8s

import (
	"context"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/day0ops/agentgateway-demo-wizard/internal/k8s/k8stest"
)

func newTestMapper() meta.RESTMapper {
	mapper := meta.NewDefaultRESTMapper(nil)
	mapper.Add(schema.GroupVersionKind{Group: "enterpriseagentgateway.solo.io", Version: "v1alpha1", Kind: "EnterpriseAgentgatewayBackend"}, meta.RESTScopeNamespace)
	mapper.Add(schema.GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1", Kind: "HTTPRoute"}, meta.RESTScopeNamespace)
	return mapper
}

const backendDoc = `
apiVersion: enterpriseagentgateway.solo.io/v1alpha1
kind: EnterpriseAgentgatewayBackend
metadata:
  name: openai
  namespace: agentgateway-system
spec:
  ai:
    provider:
      openai:
        model: gpt-4o-mini
`

const routeDoc = `
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: openai
  namespace: agentgateway-system
spec:
  parentRefs:
    - name: agentgateway-gw
`

func TestApplyThenRevertRemovesResources(t *testing.T) {
	dyn := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	applier := NewApplier(dyn, newTestMapper())
	ctx := context.Background()

	applied, err := applier.Apply(ctx, "openai-provider", [][]byte{[]byte(backendDoc), []byte(routeDoc)})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(applied) != 2 {
		t.Fatalf("expected 2 applied resources, got %d", len(applied))
	}
	if !applier.IsApplied("openai-provider") {
		t.Fatalf("expected IsApplied to be true after Apply")
	}

	backendGVR := schema.GroupVersionResource{Group: "enterpriseagentgateway.solo.io", Version: "v1alpha1", Resource: "enterpriseagentgatewaybackends"}
	if _, err := dyn.Resource(backendGVR).Namespace("agentgateway-system").Get(ctx, "openai", metav1.GetOptions{}); err != nil {
		t.Fatalf("expected backend to exist after Apply: %v", err)
	}

	if err := applier.Revert(ctx, "openai-provider"); err != nil {
		t.Fatalf("Revert: %v", err)
	}
	if applier.IsApplied("openai-provider") {
		t.Fatalf("expected IsApplied to be false after Revert")
	}
	if _, err := dyn.Resource(backendGVR).Namespace("agentgateway-system").Get(ctx, "openai", metav1.GetOptions{}); err == nil {
		t.Fatalf("expected backend to be deleted after Revert")
	}
}

func TestRevertIsIdempotent(t *testing.T) {
	dyn := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	applier := NewApplier(dyn, newTestMapper())
	ctx := context.Background()

	if err := applier.Revert(ctx, "never-applied"); err != nil {
		t.Fatalf("Revert on a never-applied policy should be a no-op, got: %v", err)
	}
}

func TestResetAllRevertsEveryAppliedPolicy(t *testing.T) {
	dyn := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	applier := NewApplier(dyn, newTestMapper())
	ctx := context.Background()

	if _, err := applier.Apply(ctx, "policy-a", [][]byte{[]byte(backendDoc)}); err != nil {
		t.Fatalf("Apply policy-a: %v", err)
	}
	if _, err := applier.Apply(ctx, "policy-b", [][]byte{[]byte(routeDoc)}); err != nil {
		t.Fatalf("Apply policy-b: %v", err)
	}

	if err := applier.ResetAll(ctx); err != nil {
		t.Fatalf("ResetAll: %v", err)
	}
	if applier.IsApplied("policy-a") || applier.IsApplied("policy-b") {
		t.Fatalf("expected ResetAll to clear every policy's applied state")
	}
}

func TestApplyRejectsUnparsableManifest(t *testing.T) {
	dyn := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	applier := NewApplier(dyn, newTestMapper())
	ctx := context.Background()

	_, err := applier.Apply(ctx, "broken", [][]byte{[]byte("not: valid: yaml: at: all:")})
	if err == nil {
		t.Fatalf("expected an error for unparsable YAML")
	}
	if !strings.Contains(err.Error(), "parsing manifest") {
		t.Fatalf("expected a 'parsing manifest' error, got: %v", err)
	}
}

func TestApplyTracksResourcesAppliedBeforeAPartialBatchFailure(t *testing.T) {
	dyn := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	applier := NewApplier(dyn, newTestMapper())
	ctx := context.Background()

	_, err := applier.Apply(ctx, "mixed", [][]byte{[]byte(backendDoc), []byte("not: valid: yaml: at: all:")})
	if err == nil {
		t.Fatalf("expected an error from the unparsable second document")
	}

	if !applier.IsApplied("mixed") {
		t.Fatalf("expected IsApplied to be true: the first document succeeded before the second failed")
	}

	backendGVR := schema.GroupVersionResource{Group: "enterpriseagentgateway.solo.io", Version: "v1alpha1", Resource: "enterpriseagentgatewaybackends"}
	if _, err := dyn.Resource(backendGVR).Namespace("agentgateway-system").Get(ctx, "openai", metav1.GetOptions{}); err != nil {
		t.Fatalf("expected backend from the first document to exist after the partial failure: %v", err)
	}

	if err := applier.Revert(ctx, "mixed"); err != nil {
		t.Fatalf("Revert: %v", err)
	}
	if applier.IsApplied("mixed") {
		t.Fatalf("expected IsApplied to be false after Revert")
	}
	if _, err := dyn.Resource(backendGVR).Namespace("agentgateway-system").Get(ctx, "openai", metav1.GetOptions{}); err == nil {
		t.Fatalf("expected backend to be deleted after Revert")
	}
}

func TestApplyPatchThenRevertPatchNeverDeletesTheSharedObject(t *testing.T) {
	dyn := k8stest.WithApplyFallback(dynamicfake.NewSimpleDynamicClient(scheme.Scheme))
	mapper := meta.NewDefaultRESTMapper(nil)
	// AddSpecific, not Add: Add's auto-pluralization heuristic guesses
	// "gatewaies" for a "Gateway" kind (it naively appends "ies" to any
	// kind ending in "y", vowel before it or not), which would silently
	// target the wrong GVR. Production uses a real discovery-based
	// RESTMapper and never hits this; only this heuristic test mapper does.
	gatewayGVK := schema.GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1", Kind: "Gateway"}
	gatewayGVR := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"}
	mapper.AddSpecific(gatewayGVK, gatewayGVR, gatewayGVR.GroupVersion().WithResource("gateway"), meta.RESTScopeNamespace)
	applier := NewApplier(dyn, mapper)
	ctx := context.Background()

	seed := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "Gateway",
		"metadata":   map[string]interface{}{"name": "agentgateway-gw", "namespace": "agentgateway-system"},
		"spec":       map[string]interface{}{"gatewayClassName": "enterprise-agentgateway"},
	}}
	if _, err := dyn.Resource(gatewayGVR).Namespace("agentgateway-system").Create(ctx, seed, metav1.CreateOptions{}); err != nil {
		t.Fatalf("seeding Gateway: %v", err)
	}

	patchDoc := []byte(`
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: agentgateway-gw
  namespace: agentgateway-system
spec:
  infrastructure:
    parametersRef:
      group: enterpriseagentgateway.solo.io
      kind: EnterpriseAgentgatewayParameters
      name: cost-attribution-params
`)

	if _, err := applier.ApplyPatch(ctx, "cost-attribution", [][]byte{patchDoc}); err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if !applier.IsApplied("cost-attribution") {
		t.Fatalf("expected IsApplied to be true after ApplyPatch")
	}

	emptyDoc := []byte(`
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: agentgateway-gw
  namespace: agentgateway-system
spec: {}
`)

	if err := applier.RevertPatch(ctx, "cost-attribution", [][]byte{emptyDoc}); err != nil {
		t.Fatalf("RevertPatch: %v", err)
	}
	if applier.IsApplied("cost-attribution") {
		t.Fatalf("expected IsApplied to be false after RevertPatch")
	}

	// The critical safety property: RevertPatch must never delete the shared
	// object, unlike Revert. Whether the API server actually clears the
	// specific field our manager released is real-cluster server-side-apply
	// behavior the fake dynamic client doesn't fully simulate, so it isn't
	// asserted here - it's covered by the manual verification step in the
	// README (Task 17).
	if _, err := dyn.Resource(gatewayGVR).Namespace("agentgateway-system").Get(ctx, "agentgateway-gw", metav1.GetOptions{}); err != nil {
		t.Fatalf("expected the Gateway to still exist after RevertPatch, got: %v", err)
	}
}

func TestApplyPatchDoesNotAffectGenericRevert(t *testing.T) {
	dyn := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	mapper := meta.NewDefaultRESTMapper(nil)
	mapper.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)
	applier := NewApplier(dyn, mapper)
	ctx := context.Background()

	configMapDoc := []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: cost-attribution-catalog
  namespace: agentgateway-system
data:
  catalog.json: "{}"
`)

	if _, err := applier.Apply(ctx, "cost-attribution", [][]byte{configMapDoc}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if err := applier.Revert(ctx, "cost-attribution"); err != nil {
		t.Fatalf("Revert: %v", err)
	}
	if applier.IsApplied("cost-attribution") {
		t.Fatalf("expected IsApplied to be false after Revert")
	}
}

// TestApplyAndApplyPatchUnderSamePolicyIDRevertIndependently mirrors the real
// cost-attribution policy, which sets both ManifestPath (a wholly-owned
// ConfigMap, deleted on Revert) and PatchManifestPath/PatchRevertManifestPath
// (a field patch on the shared Gateway, released on RevertPatch) under one
// policy ID. The critical safety property: reverting the generic side must
// never delete the shared Gateway just because it shares a policy ID with a
// patch-tracked resource.
func TestApplyAndApplyPatchUnderSamePolicyIDRevertIndependently(t *testing.T) {
	dyn := k8stest.WithApplyFallback(dynamicfake.NewSimpleDynamicClient(scheme.Scheme))
	mapper := meta.NewDefaultRESTMapper(nil)
	mapper.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)
	// AddSpecific, not Add: see TestApplyPatchThenRevertPatchNeverDeletesTheSharedObject
	// above for why Add's pluralization heuristic can't be trusted for "Gateway".
	gatewayGVK := schema.GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1", Kind: "Gateway"}
	gatewayGVR := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"}
	mapper.AddSpecific(gatewayGVK, gatewayGVR, gatewayGVR.GroupVersion().WithResource("gateway"), meta.RESTScopeNamespace)
	applier := NewApplier(dyn, mapper)
	ctx := context.Background()

	seed := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "Gateway",
		"metadata":   map[string]interface{}{"name": "agentgateway-gw", "namespace": "agentgateway-system"},
		"spec":       map[string]interface{}{"gatewayClassName": "enterprise-agentgateway"},
	}}
	if _, err := dyn.Resource(gatewayGVR).Namespace("agentgateway-system").Create(ctx, seed, metav1.CreateOptions{}); err != nil {
		t.Fatalf("seeding Gateway: %v", err)
	}

	configMapDoc := []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: cost-attribution-catalog
  namespace: agentgateway-system
data:
  catalog.json: "{}"
`)

	if _, err := applier.Apply(ctx, "cost-attribution", [][]byte{configMapDoc}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	patchDoc := []byte(`
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: agentgateway-gw
  namespace: agentgateway-system
spec:
  infrastructure:
    parametersRef:
      group: enterpriseagentgateway.solo.io
      kind: EnterpriseAgentgatewayParameters
      name: cost-attribution-params
`)

	if _, err := applier.ApplyPatch(ctx, "cost-attribution", [][]byte{patchDoc}); err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}

	if !applier.IsApplied("cost-attribution") {
		t.Fatalf("expected IsApplied to be true after Apply+ApplyPatch")
	}

	configMapGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}

	if err := applier.Revert(ctx, "cost-attribution"); err != nil {
		t.Fatalf("Revert: %v", err)
	}
	if _, err := dyn.Resource(configMapGVR).Namespace("agentgateway-system").Get(ctx, "cost-attribution-catalog", metav1.GetOptions{}); err == nil {
		t.Fatalf("expected the ConfigMap to be deleted after Revert")
	}
	if _, err := dyn.Resource(gatewayGVR).Namespace("agentgateway-system").Get(ctx, "agentgateway-gw", metav1.GetOptions{}); err != nil {
		t.Fatalf("expected the Gateway to still exist after Revert (shared object, never deleted), got: %v", err)
	}

	emptyDoc := []byte(`
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: agentgateway-gw
  namespace: agentgateway-system
spec: {}
`)

	if err := applier.RevertPatch(ctx, "cost-attribution", [][]byte{emptyDoc}); err != nil {
		t.Fatalf("RevertPatch: %v", err)
	}
	if applier.IsApplied("cost-attribution") {
		t.Fatalf("expected IsApplied to be false after Revert+RevertPatch")
	}
	if _, err := dyn.Resource(gatewayGVR).Namespace("agentgateway-system").Get(ctx, "agentgateway-gw", metav1.GetOptions{}); err != nil {
		t.Fatalf("expected the Gateway to still exist after RevertPatch, got: %v", err)
	}
}

func newTestDeployment(readyReplicas int64) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": "echo", "namespace": "agentgateway-system"},
		"spec":       map[string]interface{}{"replicas": int64(1)},
		"status":     map[string]interface{}{"readyReplicas": readyReplicas},
	}}
}

func TestWaitForDeploymentReadyTimesOutWhenNeverReady(t *testing.T) {
	dyn := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	applier := NewApplier(dyn, newTestMapper())
	ctx := context.Background()

	deploymentGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	if _, err := dyn.Resource(deploymentGVR).Namespace("agentgateway-system").Create(ctx, newTestDeployment(0), metav1.CreateOptions{}); err != nil {
		t.Fatalf("seeding Deployment: %v", err)
	}

	// Shorter than deploymentPollInterval: catches the timeout rounding up to
	// a full poll interval instead of returning once the deadline passes.
	const timeout = 300 * time.Millisecond
	start := time.Now()
	err := applier.WaitForDeploymentReady(ctx, "agentgateway-system", "echo", timeout)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected a timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "echo") {
		t.Fatalf("expected the error to name the deployment, got: %v", err)
	}
	if elapsed < timeout {
		t.Fatalf("expected to wait at least the configured timeout %s, took %s", timeout, elapsed)
	}
	if elapsed > timeout+500*time.Millisecond {
		t.Fatalf("expected to return close to the configured timeout %s, not rounded up to the %s poll interval, took %s", timeout, deploymentPollInterval, elapsed)
	}
}

func TestWaitForDeploymentReadyTreatsExplicitZeroReplicasAsReady(t *testing.T) {
	dyn := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	applier := NewApplier(dyn, newTestMapper())
	ctx := context.Background()

	scaledDown := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": "echo", "namespace": "agentgateway-system"},
		"spec":       map[string]interface{}{"replicas": int64(0)},
		"status":     map[string]interface{}{"readyReplicas": int64(0)},
	}}
	deploymentGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	if _, err := dyn.Resource(deploymentGVR).Namespace("agentgateway-system").Create(ctx, scaledDown, metav1.CreateOptions{}); err != nil {
		t.Fatalf("seeding Deployment: %v", err)
	}

	start := time.Now()
	if err := applier.WaitForDeploymentReady(ctx, "agentgateway-system", "echo", 2*time.Second); err != nil {
		t.Fatalf("WaitForDeploymentReady: %v", err)
	}
	if elapsed := time.Since(start); elapsed >= deploymentPollInterval {
		t.Fatalf("expected a Deployment explicitly scaled to 0 replicas to be ready immediately, took %s", elapsed)
	}
}

func TestWaitForDeploymentReadyDistinguishesNotFoundFromTimeout(t *testing.T) {
	dyn := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	applier := NewApplier(dyn, newTestMapper())
	ctx := context.Background()

	start := time.Now()
	err := applier.WaitForDeploymentReady(ctx, "agentgateway-system", "never-created", 2*time.Second)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected an error for a Deployment that was never created")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected a distinct 'not found' error, got: %v", err)
	}
	if strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected a not-found error distinct from the generic timeout message, got: %v", err)
	}
	if elapsed >= deploymentPollInterval {
		t.Fatalf("expected WaitForDeploymentReady to return immediately on NotFound instead of polling, took %s", elapsed)
	}
}

func TestWaitForDeploymentReadyReturnsImmediatelyWhenAlreadyReady(t *testing.T) {
	dyn := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	applier := NewApplier(dyn, newTestMapper())
	ctx := context.Background()

	deploymentGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	if _, err := dyn.Resource(deploymentGVR).Namespace("agentgateway-system").Create(ctx, newTestDeployment(1), metav1.CreateOptions{}); err != nil {
		t.Fatalf("seeding Deployment: %v", err)
	}

	start := time.Now()
	if err := applier.WaitForDeploymentReady(ctx, "agentgateway-system", "echo", 2*time.Second); err != nil {
		t.Fatalf("WaitForDeploymentReady: %v", err)
	}
	if elapsed := time.Since(start); elapsed >= deploymentPollInterval {
		t.Fatalf("expected WaitForDeploymentReady to return before the first poll interval elapsed, took %s", elapsed)
	}
}

func TestGetReturnsAppliedResource(t *testing.T) {
	dyn := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	applier := NewApplier(dyn, newTestMapper())
	ctx := context.Background()

	if _, err := applier.Apply(ctx, "openai-provider", [][]byte{[]byte(backendDoc)}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	backendGVR := schema.GroupVersionResource{Group: "enterpriseagentgateway.solo.io", Version: "v1alpha1", Resource: "enterpriseagentgatewaybackends"}
	obj, err := applier.Get(ctx, backendGVR, "agentgateway-system", "openai")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if obj.GetName() != "openai" {
		t.Fatalf("expected name 'openai', got %q", obj.GetName())
	}
}

func TestGetReturnsNotFoundForMissingResource(t *testing.T) {
	dyn := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	applier := NewApplier(dyn, newTestMapper())
	ctx := context.Background()

	backendGVR := schema.GroupVersionResource{Group: "enterpriseagentgateway.solo.io", Version: "v1alpha1", Resource: "enterpriseagentgatewaybackends"}
	_, err := applier.Get(ctx, backendGVR, "agentgateway-system", "does-not-exist")
	if err == nil {
		t.Fatalf("expected an error for a missing resource")
	}
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected a NotFound error, got: %v", err)
	}
}
