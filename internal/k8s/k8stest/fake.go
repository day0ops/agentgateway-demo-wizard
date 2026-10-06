// Package k8stest provides test doubles for exercising internal/k8s.Applier
// against client-go's fake dynamic client. Nothing here is imported by the
// wizard's production code; it exists purely to be imported from test files
// (including other packages' tests), which is why it is its own package
// rather than living in a _test.go file.
package k8stest

import (
	"context"
	"encoding/json"

	jsonpatch "gopkg.in/evanphx/json-patch.v4"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// WithApplyFallback wraps dyn so Resource(...).Namespace(...).Apply(...)
// also works against an object that already exists.
//
// client-go's non-managed-fields fake dynamic client (dynamicfake's plain
// ObjectTracker) simulates the Apply verb by Go-reflecting over the live
// object's type to look up each JSON field's patch metadata, which only
// works for typed API structs. Every object that client stores is an
// *unstructured.Unstructured, which has no such fields, so a second Apply
// against an object that already exists always errors ("unable to find api
// field in struct Unstructured..."). A real API server never produces that
// error - every real apiserver rejection is a structured
// apierrors.APIStatus - so this is purely a fake-client limitation, never a
// production code path.
//
// internal/k8s.Applier's ApplyPatch/RevertPatch is the first feature to
// Apply the same object twice (once to set a field, again to release it),
// exactly the scenario the plain fake tracker can't simulate. This wrapper
// recovers only from that specific, unmistakable failure mode by merging
// client-side (RFC 7396 JSON merge patch) and writing the result back with
// Update, so tests can still exercise the Applier's own tracking and
// never-delete behavior against a pre-existing object.
func WithApplyFallback(dyn dynamic.Interface) dynamic.Interface {
	return applyFallbackDynamicClient{dyn}
}

type applyFallbackDynamicClient struct {
	dynamic.Interface
}

func (c applyFallbackDynamicClient) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return applyFallbackNamespaceableResource{c.Interface.Resource(gvr)}
}

type applyFallbackNamespaceableResource struct {
	dynamic.NamespaceableResourceInterface
}

func (r applyFallbackNamespaceableResource) Namespace(ns string) dynamic.ResourceInterface {
	return applyFallbackResource{r.NamespaceableResourceInterface.Namespace(ns)}
}

type applyFallbackResource struct {
	dynamic.ResourceInterface
}

func (r applyFallbackResource) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options metav1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	applied, err := r.ResourceInterface.Apply(ctx, name, obj, options, subresources...)
	if err == nil || apierrors.IsNotFound(err) {
		return applied, err
	}

	live, getErr := r.Get(ctx, name, metav1.GetOptions{})
	if getErr != nil {
		return nil, err
	}
	liveJSON, marshalErr := json.Marshal(live)
	if marshalErr != nil {
		return nil, err
	}
	patchJSON, marshalErr := json.Marshal(obj)
	if marshalErr != nil {
		return nil, err
	}
	mergedJSON, mergeErr := jsonpatch.MergePatch(liveJSON, patchJSON)
	if mergeErr != nil {
		return nil, err
	}

	merged := &unstructured.Unstructured{}
	if unmarshalErr := merged.UnmarshalJSON(mergedJSON); unmarshalErr != nil {
		return nil, err
	}
	return r.Update(ctx, merged, metav1.UpdateOptions{FieldManager: options.FieldManager})
}
