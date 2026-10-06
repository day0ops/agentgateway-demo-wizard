package scenarios

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRegistryHasIntroAndWrap(t *testing.T) {
	reg := Registry()

	if len(reg) != 7 {
		t.Fatalf("expected 7 pillars (intro, routing, auth, guardrails, cost, agent, wrap), got %d", len(reg))
	}
	if reg[0].ID != "intro" {
		t.Fatalf("expected first pillar id 'intro', got %q", reg[0].ID)
	}
	if reg[len(reg)-1].ID != "wrap" {
		t.Fatalf("expected last pillar id 'wrap', got %q", reg[len(reg)-1].ID)
	}
	if len(reg[0].Steps) == 0 || reg[0].Steps[0].Explanation == "" {
		t.Fatalf("expected the intro pillar's first step to have explanation text")
	}
}

// TestRegistryStepsNeverNilSlices guards against the exact bug that crashed
// the wizard on load: a step whose literal never sets Policies/Presets is
// left at Go's zero value (nil), which encoding/json marshals as JSON null.
// The frontend calls .map() on both fields unconditionally, so a null
// crashes the app. Every step, not just the ones currently affected, must
// hold this invariant so a future step can't regress it silently.
func TestRegistryStepsNeverNilSlices(t *testing.T) {
	reg := Registry()

	for _, pillar := range reg {
		for _, step := range pillar.Steps {
			if step.Policies == nil {
				t.Errorf("pillar %q step %q: Policies is nil, want non-nil (empty) slice", pillar.ID, step.ID)
			}
			if step.Presets == nil {
				t.Errorf("pillar %q step %q: Presets is nil, want non-nil (empty) slice", pillar.ID, step.ID)
			}
		}
	}
}

// TestRegistryJSONNeverContainsNullPoliciesOrPresets asserts the actual
// condition that crashed the browser: the marshaled JSON string must never
// contain "policies":null or "presets":null, since that's precisely what the
// frontend receives and calls .map() on.
func TestRegistryJSONNeverContainsNullPoliciesOrPresets(t *testing.T) {
	reg := Registry()

	data, err := json.Marshal(reg)
	if err != nil {
		t.Fatalf("json.Marshal(Registry()): %v", err)
	}

	got := string(data)
	if strings.Contains(got, `"policies":null`) {
		t.Errorf("marshaled registry JSON contains \"policies\":null, which crashes the frontend's .map() call:\n%s", got)
	}
	if strings.Contains(got, `"presets":null`) {
		t.Errorf("marshaled registry JSON contains \"presets\":null, which crashes the frontend's .map() call:\n%s", got)
	}
}
