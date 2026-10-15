// Package scenarios is the single source of truth for the wizard's 10-step
// demo script: the explanation and diagram shown for each step, the policies
// it can apply/revert, and the request presets it can fire. The frontend is a
// pure renderer of this registry via GET /api/scenarios.
package scenarios

import (
	"github.com/day0ops/agentgateway-demo-wizard/internal/manifests"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Pillar is one of the demo's top-level sections, shown in the breadcrumb.
type Pillar struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Teaser is a one-line, benefit-oriented summary of what this pillar
	// demonstrates. Empty for the intro/wrap-up pillars, which aren't
	// features themselves - the welcome step lists every pillar that has one.
	Teaser string `json:"teaser"`
	Steps  []Step `json:"steps"`
}

// Step is one screen of the wizard within a Pillar.
type Step struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Explanation string          `json:"explanation"`
	Diagram     string          `json:"diagram"`
	Policies    []Policy        `json:"policies"`
	Presets     []RequestPreset `json:"presets"`
	AgentDemo   bool            `json:"agentDemo"`
	VirtualKeys bool            `json:"virtualKeys"`
	// LoginDemo marks the one step (auth pillar's "Real login") that
	// renders a dedicated login pane instead of Configure/Request panes -
	// there's no policy to apply and no request to drive, just agentgateway's
	// own ext-auth redirect to Keycloak's real hosted login page.
	LoginDemo bool `json:"loginDemo"`
	// OpsAgent marks a step that drives one of the ops-agents usecase's real
	// external agent containers (see internal/api/ops_agent_handler.go)
	// instead of a Configure+DriveRequest pane pair or this repo's own
	// built-in agent loop (AgentDemo).
	OpsAgent *OpsAgentDemo `json:"opsAgent,omitempty"`
	// DirectCall marks a step with an additional raw, non-delegated MCP tool
	// call button alongside its OpsAgent pane - see DirectCallDemo.
	DirectCall *DirectCallDemo `json:"directCall,omitempty"`
}

// OpsAgentDemo configures the pane for a step that calls a real external
// agent container through POST /secure/ops-agent/run.
type OpsAgentDemo struct {
	// Agent selects which container's /run endpoint this step calls - must
	// match one of the cases in Server.opsAgentURL: "accounts",
	// "transfer-orchestrator", or "statements".
	Agent string `json:"agent"`
	// Mode is forwarded to the agent as-is. The Statements Agent branches on
	// it ("interactive" targets PayPal, anything else targets Stripe via
	// Brokered); the other banking agents don't branch on mode and leave it
	// empty.
	Mode          string `json:"mode"`
	DefaultPrompt string `json:"defaultPrompt"`
}

// DirectCallDemo configures a step's optional "direct, non-delegated call"
// button (see internal/api/secure_direct_call_handler.go) - for steps whose
// normal OpsAgent call can never exercise the denied path a policy protects
// against (e.g. auth-settlement-rbac's Transfer Orchestrator always goes
// through the proper delegation chain by construction).
type DirectCallDemo struct {
	Title     string         `json:"title"`
	Path      string         `json:"path"`
	Tool      string         `json:"tool"`
	Arguments map[string]any `json:"arguments"`
}

// Policy is one clickops Configure-pane card: a named group of manifests the
// presenter can Apply and Revert.
//
// ManifestPath is for wholly wizard-owned resources: Apply/Revert via the
// generic, delete-based path (k8s.Applier.Apply/Revert).
//
// PatchManifestPath/PatchRevertManifestPath are both optional and used
// together, only by policies that must attach to a shared, pre-provisioned
// object (e.g. the Gateway's spec.infrastructure.parametersRef) rather than
// create a new one: PatchManifestPath is the partial manifest to apply,
// PatchRevertManifestPath is the "empty" version of the same fields that
// releases the wizard's ownership of them on Revert
// (k8s.Applier.ApplyPatch/RevertPatch). As of this plan, only the
// cost-attribution policy (Task 14) uses these.
type Policy struct {
	ID                      string    `json:"id"`
	Title                   string    `json:"title"`
	ManifestPath            string    `json:"-"`
	PatchManifestPath       string    `json:"-"`
	PatchRevertManifestPath string    `json:"-"`
	ReadOnly                bool      `json:"readOnly"`
	ViewRefs                []ViewRef `json:"-"`
}

// ViewRef names one pre-provisioned resource a ReadOnly Policy displays via
// GET /api/config/view. It is never applied or deleted by this repo - the
// resource is owned by whatever provisioned it (see
// docs/superpowers/specs/2026-10-08-demo-wizard-field-kit-integration-design.md
// in agentgateway-field-kit for the resources the routing pillar expects).
type ViewRef struct {
	GVR  schema.GroupVersionResource
	Name string
}

// ManifestParams are the template variables available to a policy's manifest.
type ManifestParams = manifests.Params

// Render renders this policy's manifest template into applyable YAML
// documents. Returns nil, nil if ManifestPath is empty.
func (p Policy) Render(params ManifestParams) ([][]byte, error) {
	if p.ManifestPath == "" {
		return nil, nil
	}
	return manifests.Render(p.ManifestPath, params)
}

// RenderPatch renders this policy's shared-object patch manifest. Returns
// nil, nil if PatchManifestPath is empty.
func (p Policy) RenderPatch(params ManifestParams) ([][]byte, error) {
	if p.PatchManifestPath == "" {
		return nil, nil
	}
	return manifests.Render(p.PatchManifestPath, params)
}

// RenderPatchRevert renders the manifest that releases this policy's
// shared-object patch. Returns nil, nil if PatchRevertManifestPath is empty.
func (p Policy) RenderPatchRevert(params ManifestParams) ([][]byte, error) {
	if p.PatchRevertManifestPath == "" {
		return nil, nil
	}
	return manifests.Render(p.PatchRevertManifestPath, params)
}

// RequestPreset is one clickops Drive-Request-pane button: a pre-filled,
// editable request the presenter can send.
type RequestPreset struct {
	ID       string            `json:"id"`
	Title    string            `json:"title"`
	Identity string            `json:"identity"`
	Method   string            `json:"method"`
	Path     string            `json:"path"`
	Headers  map[string]string `json:"headers"`
	Body     string            `json:"body"`
	Stream   bool              `json:"stream"`
}
