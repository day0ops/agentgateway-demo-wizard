package scenarios

import (
	"encoding/json"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestPolicyJSONHidesViewRefsButShowsReadOnly(t *testing.T) {
	p := Policy{
		ID:       "routing-fallback",
		Title:    "Priority-tiered failover",
		ReadOnly: true,
		ViewRefs: []ViewRef{{
			GVR:  schema.GroupVersionResource{Group: "enterpriseagentgateway.solo.io", Version: "v1alpha1", Resource: "enterpriseagentgatewaybackends"},
			Name: "routing-fallback",
		}},
	}

	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded["readOnly"] != true {
		t.Fatalf("expected readOnly:true in JSON, got: %s", raw)
	}
	if _, present := decoded["viewRefs"]; present {
		t.Fatalf("expected viewRefs to be hidden from JSON, got: %s", raw)
	}
	if _, present := decoded["ViewRefs"]; present {
		t.Fatalf("expected ViewRefs to be hidden from JSON, got: %s", raw)
	}
}

func TestReadOnlyPolicyRendersNothing(t *testing.T) {
	p := Policy{ID: "routing-fallback", ReadOnly: true}
	docs, err := p.Render(ManifestParams{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if docs != nil {
		t.Fatalf("expected a read-only policy with no ManifestPath to render nothing, got: %v", docs)
	}
}
