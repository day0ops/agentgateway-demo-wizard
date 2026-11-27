package gateway

import (
	"testing"

	"github.com/day0ops/agentgateway-demo-wizard/internal/scenarios"
)

// TestRegistryPresetIdentitiesMatchClientMaps guards against a regression
// already caught once in review: a RequestPreset using a tenant/demo
// identity string that doesn't match this package's auth maps, which would
// make the preset's Identity unresolvable at demo time. Lives here, not in
// package scenarios, because scenarios must not import gateway - this test
// imports scenarios instead, so it can check both maps' real values rather
// than a copy that could itself drift.
func TestRegistryPresetIdentitiesMatchClientMaps(t *testing.T) {
	for _, pillar := range scenarios.Registry() {
		for _, step := range pillar.Steps {
			for _, preset := range step.Presets {
				if preset.Identity == "" || preset.Identity == "anonymous" {
					continue
				}

				switch preset.Gateway {
				case "tenants":
					if _, ok := tenantIdentities[preset.Identity]; !ok {
						t.Errorf("pillar %q step %q preset %q: identity %q is not in tenantIdentities",
							pillar.ID, step.ID, preset.ID, preset.Identity)
					}
				case "":
					if _, ok := demoUsers[preset.Identity]; !ok {
						t.Errorf("pillar %q step %q preset %q: identity %q is not in demoUsers",
							pillar.ID, step.ID, preset.ID, preset.Identity)
					}
				default:
					t.Errorf("pillar %q step %q preset %q: unknown Gateway %q",
						pillar.ID, step.ID, preset.ID, preset.Gateway)
				}
			}
		}
	}
}
