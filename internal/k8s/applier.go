// Package k8s pushes the wizard's own agentgateway CRD manifests to the
// cluster via server-side apply, and tracks which resources each policy
// applied so Revert and ResetAll can remove exactly those resources.
package k8s

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/yaml"
)

const fieldManager = "agentgateway-demo-wizard"

// deploymentGVR identifies the apps/v1 Deployment resource
// WaitForDeploymentReady polls.
var deploymentGVR = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

// deploymentPollInterval is how often WaitForDeploymentReady re-checks a
// Deployment's status while waiting for it to become ready.
const deploymentPollInterval = 2 * time.Second

// AppliedResource identifies one resource a policy applied to the cluster.
type AppliedResource struct {
	GVR       schema.GroupVersionResource
	Namespace string
	Name      string
}

// Applier applies and reverts the wizard's manifests via server-side apply,
// tracking what each policy ID has applied so Revert only removes that
// policy's own resources.
//
// Apply/Revert and ApplyPatch/RevertPatch are tracked separately. Apply's
// resources are wholly owned by the wizard and Revert deletes them outright.
// ApplyPatch is for partial manifests against a shared, pre-provisioned
// object (e.g. just spec.infrastructure.parametersRef on the Gateway) -
// RevertPatch never deletes that object, it re-applies an "empty" version of
// the same fields under the wizard's field manager, which releases their
// ownership instead.
type Applier struct {
	dynamic dynamic.Interface
	mapper  meta.RESTMapper

	mu           sync.Mutex
	applied      map[string][]AppliedResource
	patchTracked map[string][]AppliedResource
}

// NewApplier builds an Applier backed by dyn, resolving GroupVersionKind to
// GroupVersionResource via mapper.
func NewApplier(dyn dynamic.Interface, mapper meta.RESTMapper) *Applier {
	return &Applier{
		dynamic:      dyn,
		mapper:       mapper,
		applied:      map[string][]AppliedResource{},
		patchTracked: map[string][]AppliedResource{},
	}
}

// Apply server-side-applies each document in docs (one Kubernetes object per
// document) and records them under policyID as wholly wizard-owned. Blank
// documents (from a trailing "---" separator) are skipped.
func (a *Applier) Apply(ctx context.Context, policyID string, docs [][]byte) ([]AppliedResource, error) {
	return a.applyDocs(ctx, policyID, docs, a.applied)
}

// ApplyPatch server-side-applies docs the same way Apply does, but tracks
// them separately: these are partial manifests against a shared object the
// wizard must never delete. Use RevertPatch, not Revert, to undo it.
func (a *Applier) ApplyPatch(ctx context.Context, policyID string, docs [][]byte) ([]AppliedResource, error) {
	return a.applyDocs(ctx, policyID, docs, a.patchTracked)
}

// applyDocs server-side-applies each document in docs and records it under
// policyID in tracked, the generic applied map for Apply or the separate
// patchTracked map for ApplyPatch. Each resource is recorded as soon as it
// succeeds, not after the whole batch completes, so a later document's
// failure still leaves the earlier ones discoverable by
// Revert/RevertPatch/IsApplied/ResetAll.
func (a *Applier) applyDocs(ctx context.Context, policyID string, docs [][]byte, tracked map[string][]AppliedResource) ([]AppliedResource, error) {
	var resources []AppliedResource

	for _, doc := range docs {
		obj := &unstructured.Unstructured{}
		if err := yaml.Unmarshal(doc, &obj.Object); err != nil {
			return resources, fmt.Errorf("parsing manifest: %w", err)
		}
		if len(obj.Object) == 0 {
			continue
		}

		gvk := obj.GroupVersionKind()
		mapping, err := a.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return resources, fmt.Errorf("resolving REST mapping for %s: %w", gvk, err)
		}

		ri := a.resourceInterface(mapping, obj.GetNamespace())
		applied, err := ri.Apply(ctx, obj.GetName(), obj, metav1.ApplyOptions{FieldManager: fieldManager, Force: true})
		if apierrors.IsNotFound(err) {
			// A real API server upserts on server-side apply, but client-go's
			// fake dynamic client (used in tests) only patches existing
			// objects; fall back to a plain create for the first apply.
			applied, err = ri.Create(ctx, obj, metav1.CreateOptions{FieldManager: fieldManager})
		}
		if err != nil {
			return resources, fmt.Errorf("applying %s %q in namespace %q: %w", gvk.Kind, obj.GetName(), obj.GetNamespace(), err)
		}

		resource := AppliedResource{
			GVR:       mapping.Resource,
			Namespace: applied.GetNamespace(),
			Name:      applied.GetName(),
		}
		resources = append(resources, resource)

		a.mu.Lock()
		tracked[policyID] = append(tracked[policyID], resource)
		a.mu.Unlock()
	}

	return resources, nil
}

// Revert deletes every resource previously applied under policyID via Apply,
// in reverse order, and forgets them. A policyID with nothing applied is a
// no-op. Already-absent resources are not an error. Never touches resources
// tracked via ApplyPatch.
func (a *Applier) Revert(ctx context.Context, policyID string) error {
	a.mu.Lock()
	resources := a.applied[policyID]
	delete(a.applied, policyID)
	a.mu.Unlock()

	var errs []error
	for i := len(resources) - 1; i >= 0; i-- {
		r := resources[i]
		ri := a.resourceInterfaceFor(r.GVR, r.Namespace)
		if err := ri.Delete(ctx, r.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("deleting %s %q: %w", r.GVR.Resource, r.Name, err))
		}
	}
	return errors.Join(errs...)
}

// RevertPatch undoes a previous ApplyPatch by server-side-applying
// emptyDocs - the same objects with the previously-set fields omitted - under
// the wizard's field manager, which releases its ownership of those fields
// instead of deleting the (shared) object. It never deletes anything: that
// is the entire point of tracking ApplyPatch separately from Apply.
func (a *Applier) RevertPatch(ctx context.Context, policyID string, emptyDocs [][]byte) error {
	a.mu.Lock()
	delete(a.patchTracked, policyID)
	a.mu.Unlock()

	// Applied into a throwaway map: the "empty" documents replace this
	// policy's ownership record, they don't need their own tracking entry.
	if _, err := a.applyDocs(ctx, policyID, emptyDocs, map[string][]AppliedResource{}); err != nil {
		return err
	}
	return nil
}

// ResetAll reverts every policy that currently has Apply-tracked resources,
// returning the cluster to its pre-provisioned baseline. It does not revert
// ApplyPatch-tracked policies - callers that also use ApplyPatch (the API
// layer, which knows each patch policy's revert manifest) are responsible
// for reverting those themselves; see Task 14/17.
func (a *Applier) ResetAll(ctx context.Context) error {
	a.mu.Lock()
	ids := make([]string, 0, len(a.applied))
	for id := range a.applied {
		ids = append(ids, id)
	}
	a.mu.Unlock()

	var errs []error
	for _, id := range ids {
		if err := a.Revert(ctx, id); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// IsApplied reports whether policyID currently has any resources applied,
// via either Apply or ApplyPatch.
func (a *Applier) IsApplied(policyID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.applied[policyID]) > 0 || len(a.patchTracked[policyID]) > 0
}

// Get fetches the current state of a single resource. Callers that need to
// extend an existing list field (RateLimitConfig descriptors,
// EnterpriseAgentgatewayBudget budgets) without clobbering entries another
// caller already added use this to read the object, modify it in Go, and
// re-Apply the whole merged object - see virtual-key lifecycle in the api
// package. Returns the underlying apierrors.IsNotFound-detectable error
// unwrapped, so callers can distinguish "doesn't exist" from other failures.
func (a *Applier) Get(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error) {
	ri := a.resourceInterfaceFor(gvr, namespace)
	return ri.Get(ctx, name, metav1.GetOptions{})
}

// WaitForDeploymentReady polls the named apps/v1 Deployment until its
// readyReplicas matches its desired replica count (spec.replicas, defaulting
// to 1 when unset; a Deployment explicitly scaled to 0 replicas is ready
// immediately, since there's nothing to wait for), so callers can confirm a
// just-applied Deployment is actually serving traffic before declaring an
// operation done. Returns the context's error if ctx is cancelled first, a
// distinct "not found" error if the Deployment doesn't exist (this is always
// called right after Apply has already succeeded on it, so a NotFound here
// means something else is wrong, not that it hasn't shown up yet), or a
// timeout error naming the deployment once timeout elapses without it
// becoming ready.
func (a *Applier) WaitForDeploymentReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for {
		ready, err := a.deploymentIsReady(ctx, namespace, name)
		if err != nil && apierrors.IsNotFound(err) {
			return fmt.Errorf("deployment %q in namespace %q not found", name, namespace)
		}
		if err == nil && ready {
			return nil
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("timed out after %s waiting for deployment %q in namespace %q to become ready", timeout, name, namespace)
		}

		// Sleep for whichever is shorter so a timeout shorter than one poll
		// interval doesn't overshoot the deadline by waiting out the full
		// interval anyway.
		wait := deploymentPollInterval
		if remaining < wait {
			wait = remaining
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}

// deploymentIsReady reports whether namespace/name's Deployment currently has
// as many ready replicas as it desires. A Deployment explicitly scaled to 0
// replicas is reported ready immediately. A non-nil error means the status
// couldn't be read; the caller distinguishes a NotFound error (the Deployment
// doesn't exist) from any other read failure, which is treated as not-ready
// and retried.
func (a *Applier) deploymentIsReady(ctx context.Context, namespace, name string) (bool, error) {
	obj, err := a.dynamic.Resource(deploymentGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("getting deployment %q in namespace %q: %w", name, namespace, err)
	}

	replicas, found, err := unstructured.NestedInt64(obj.Object, "spec", "replicas")
	if err != nil {
		return false, fmt.Errorf("reading spec.replicas for deployment %q: %w", name, err)
	}
	if !found {
		replicas = 1
	}
	if replicas == 0 {
		return true, nil
	}

	readyReplicas, _, err := unstructured.NestedInt64(obj.Object, "status", "readyReplicas")
	if err != nil {
		return false, fmt.Errorf("reading status.readyReplicas for deployment %q: %w", name, err)
	}

	return readyReplicas > 0 && readyReplicas == replicas, nil
}

// resourceInterface routes namespaced resources to namespace (the object's
// own metadata.namespace); the API server has no way to infer a namespaced
// resource's namespace from the request body alone.
func (a *Applier) resourceInterface(mapping *meta.RESTMapping, namespace string) dynamic.ResourceInterface {
	if mapping.Scope.Name() != meta.RESTScopeNameNamespace {
		namespace = ""
	}
	return a.resourceInterfaceFor(mapping.Resource, namespace)
}

func (a *Applier) resourceInterfaceFor(gvr schema.GroupVersionResource, namespace string) dynamic.ResourceInterface {
	if namespace != "" {
		return a.dynamic.Resource(gvr).Namespace(namespace)
	}
	return a.dynamic.Resource(gvr)
}
