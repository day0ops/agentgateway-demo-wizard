package scenarios

import (
	"strings"
	"testing"
)

var wantAuthStepIDs = []string{
	"auth-questions",
	"auth-login",
	"auth-accounts-impersonation",
	"auth-accounts-rbac",
	"auth-transfer-chain",
	"auth-settlement-rbac",
	"auth-statements-interactive",
	"auth-statements-brokered",
}

func TestAuthPillarStepIDsInOrder(t *testing.T) {
	auth := authPillar()
	if auth.ID != "auth" {
		t.Fatalf("expected pillar ID 'auth', got %q", auth.ID)
	}
	if auth.Title != "Who's allowed to do what" {
		t.Fatalf("expected pillar Title to stay \"Who's allowed to do what\", got %q", auth.Title)
	}
	if len(auth.Steps) != len(wantAuthStepIDs) {
		t.Fatalf("expected %d auth steps, got %d", len(wantAuthStepIDs), len(auth.Steps))
	}
	for i, want := range wantAuthStepIDs {
		if auth.Steps[i].ID != want {
			t.Fatalf("expected step[%d].ID %q, got %q", i, want, auth.Steps[i].ID)
		}
	}
}

func TestAuthQuestionsIntroStepHasNoRequestOrAgent(t *testing.T) {
	intro := findStep(authPillar().Steps, "auth-questions")

	if len(intro.Presets) != 0 {
		t.Fatalf("expected the intro step to have no presets, got %d", len(intro.Presets))
	}
	if len(intro.Policies) != 0 {
		t.Fatalf("expected the intro step to have no policies, got %d", len(intro.Policies))
	}
	if intro.OpsAgent != nil {
		t.Fatalf("expected the intro step to have no OpsAgent")
	}
	if intro.LoginDemo {
		t.Fatalf("expected the intro step to not be the login demo")
	}
	if intro.Explanation == "" || intro.Diagram == "" {
		t.Fatalf("expected the intro step to carry an Explanation and a Diagram")
	}
}

func TestAuthLoginStepKeepsLoginDemo(t *testing.T) {
	login := findStep(authPillar().Steps, "auth-login")

	if !login.LoginDemo {
		t.Fatalf("expected the login step to set LoginDemo")
	}
	if len(login.Policies) != 0 {
		t.Fatalf("expected the login step to have no policies, got %d", len(login.Policies))
	}
	if len(login.Presets) != 0 {
		t.Fatalf("expected the login step to have no presets, got %d", len(login.Presets))
	}
	if login.OpsAgent != nil {
		t.Fatalf("expected the login step to have no OpsAgent")
	}
	if login.Diagram == "" {
		t.Fatalf("expected the login step to keep its ext-auth sequence diagram")
	}
}

func TestAuthAgentStepsDriveTheRightOpsAgent(t *testing.T) {
	wantAgent := map[string]string{
		"auth-accounts-impersonation": "accounts",
		"auth-accounts-rbac":          "accounts",
		"auth-transfer-chain":         "transfer-orchestrator",
		"auth-settlement-rbac":        "transfer-orchestrator",
		"auth-statements-interactive": "statements",
		"auth-statements-brokered":    "statements",
	}

	auth := authPillar()
	for id, want := range wantAgent {
		step := findStep(auth.Steps, id)
		if step.OpsAgent == nil {
			t.Fatalf("step %q: expected OpsAgent to be set", id)
		}
		if step.OpsAgent.Agent != want {
			t.Fatalf("step %q: OpsAgent.Agent = %q, want %q", id, step.OpsAgent.Agent, want)
		}
		if step.OpsAgent.Agent != "accounts" && step.OpsAgent.Agent != "transfer-orchestrator" && step.OpsAgent.Agent != "statements" {
			t.Fatalf("step %q: OpsAgent.Agent %q is not one of the valid opsAgentURL cases", id, step.OpsAgent.Agent)
		}
		if step.OpsAgent.DefaultPrompt == "" {
			t.Fatalf("step %q: expected a non-empty DefaultPrompt", id)
		}
	}
}

// TestStatementsStepsSetModeToSelectTheRightProvider enforces that the two
// Statements Agent steps carry the Mode the agent needs to pick the right
// upstream (Stripe only ever supports Brokered, so Interactive targets
// PayPal instead - see the agent's build_agent() in ops-agents).
func TestStatementsStepsSetModeToSelectTheRightProvider(t *testing.T) {
	wantMode := map[string]string{
		"auth-statements-interactive": "interactive",
		"auth-statements-brokered":    "brokered",
	}

	auth := authPillar()
	for id, want := range wantMode {
		step := findStep(auth.Steps, id)
		if step.OpsAgent == nil {
			t.Fatalf("step %q: expected OpsAgent to be set", id)
		}
		if step.OpsAgent.Mode != want {
			t.Fatalf("step %q: OpsAgent.Mode = %q, want %q", id, step.OpsAgent.Mode, want)
		}
	}
}

// wantViewRefNames is the expected, ordered set of ViewRef names per
// read-only agent step - see the field-kit integration contract referenced
// from the GVR const docs in registry.go.
var wantViewRefNames = map[string][]string{
	"auth-accounts-impersonation": {"core-banking-standard"},
	"auth-accounts-rbac":          {"core-banking-standard", "mcp-tool-access-core-banking-standard-backend"},
	"auth-transfer-chain":         {"transfer-orchestrator-agent", "settlement-agent", "core-banking-delegation"},
	"auth-settlement-rbac":        {"core-banking-delegation", "mcp-tool-access-core-banking-delegation-backend"},
	"auth-statements-interactive": {"mcp-paypal", "elicitation-backend-paypal-mcp-backend"},
	"auth-statements-brokered":    {"mcp-stripe-brokered", "elicitation-backend-stripe-mcp-brokered-backend"},
}

// policyViewRefNames are the ViewRef names that point at an
// EnterpriseAgentgatewayPolicy rather than an HTTPRoute. These must match
// the *merged* resource name EnterpriseAgentgatewayPolicy objects actually
// get in the cluster (feature name + targetRef name - see
// FeatureManager.commit in agentgateway-field-kit's src/lib/feature.js),
// not whatever policyName the usecase config happens to set - every policy
// goes through that merge/rename pipeline regardless of contributor count.
var policyViewRefNames = map[string]bool{
	"mcp-tool-access-core-banking-standard-backend":   true,
	"mcp-tool-access-core-banking-delegation-backend": true,
	"elicitation-backend-paypal-mcp-backend":          true,
	"elicitation-backend-stripe-mcp-brokered-backend": true,
}

func TestAuthAgentStepsAreReadOnlyAndNeverApply(t *testing.T) {
	auth := authPillar()
	for id, want := range wantViewRefNames {
		step := findStep(auth.Steps, id)
		if len(step.Policies) != 1 {
			t.Fatalf("step %q: expected exactly 1 policy, got %d", id, len(step.Policies))
		}
		p := step.Policies[0]
		if !p.ReadOnly {
			t.Fatalf("step %q: expected policy to be ReadOnly", id)
		}
		if len(p.ViewRefs) != len(want) {
			t.Fatalf("step %q: expected %d ViewRefs, got %d", id, len(want), len(p.ViewRefs))
		}
		for i, name := range want {
			if p.ViewRefs[i].Name != name {
				t.Fatalf("step %q: ViewRefs[%d].Name = %q, want %q", id, i, p.ViewRefs[i].Name, name)
			}
			wantResource := "httproutes"
			if policyViewRefNames[name] {
				wantResource = "enterpriseagentgatewaypolicies"
			}
			if p.ViewRefs[i].GVR.Resource != wantResource {
				t.Fatalf("step %q: ViewRefs[%d] (%s).GVR.Resource = %q, want %q", id, i, name, p.ViewRefs[i].GVR.Resource, wantResource)
			}
		}

		docs, err := p.Render(ManifestParams{Namespace: "agentgateway-system"})
		if err != nil {
			t.Fatalf("step %q: Render errored: %v", id, err)
		}
		if docs != nil {
			t.Fatalf("step %q: expected a read-only policy to render nothing, got %v", id, docs)
		}
	}
}

// TestAuthExplanationsNameTheQuestionsTheyAnswer enforces the pillar's
// organizing principle: every step's Explanation must explicitly name which
// of the four framing questions (set up in auth-questions) it answers.
func TestAuthExplanationsNameTheQuestionsTheyAnswer(t *testing.T) {
	wantQuestions := map[string][]string{
		"auth-questions":              {"Question 1", "Question 2", "Question 3", "Question 4"},
		"auth-login":                  {"Question 1"},
		"auth-accounts-impersonation": {"Question 1", "Question 2", "Question 3"},
		"auth-accounts-rbac":          {"Question 4"},
		"auth-transfer-chain":         {"Question 1", "Question 2", "Question 3"},
		"auth-settlement-rbac":        {"Question 4"},
		"auth-statements-interactive": {"Question 4"},
		"auth-statements-brokered":    {"Question 4"},
	}

	auth := authPillar()
	for id, questions := range wantQuestions {
		step := findStep(auth.Steps, id)
		for _, q := range questions {
			if !strings.Contains(step.Explanation, q) {
				t.Fatalf("step %q: expected Explanation to name %q, got: %s", id, q, step.Explanation)
			}
		}
	}
}

func findStep(steps []Step, id string) Step {
	for _, s := range steps {
		if s.ID == id {
			return s
		}
	}
	return Step{}
}

func toStrings(docs [][]byte) []string {
	out := make([]string, len(docs))
	for i, d := range docs {
		out[i] = string(d)
	}
	return out
}
