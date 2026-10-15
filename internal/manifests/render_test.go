package manifests

import (
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestRenderSplitsDocumentsAndSubstitutesParams(t *testing.T) {
	docs, err := Render("testfixture/backend.yaml.tmpl", Params{Namespace: "agentgateway-system"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(docs))
	}
	if !strings.Contains(string(docs[0]), "namespace: agentgateway-system") {
		t.Fatalf("expected namespace substitution in doc 0, got: %s", docs[0])
	}
	if !strings.Contains(string(docs[1]), "test-fixture-2") {
		t.Fatalf("expected doc 1 to be the second ConfigMap, got: %s", docs[1])
	}
}

func TestRenderErrorsOnUnknownPath(t *testing.T) {
	if _, err := Render("does/not/exist.yaml.tmpl", Params{}); err == nil {
		t.Fatalf("expected an error for an unknown template path")
	}
}

func TestAccountsRBACTemplateRendersValidPolicy(t *testing.T) {
	docs, err := Render("auth/accounts-rbac.yaml.tmpl", Params{Namespace: "agentgateway-system"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	var parsed struct {
		Kind string `yaml:"kind"`
		Spec struct {
			TargetRefs []struct {
				Kind string `yaml:"kind"`
				Name string `yaml:"name"`
			} `yaml:"targetRefs"`
			Backend struct {
				MCP struct {
					Authorization struct {
						Action string `yaml:"action"`
						Policy struct {
							MatchExpressions []string `yaml:"matchExpressions"`
						} `yaml:"policy"`
					} `yaml:"authorization"`
				} `yaml:"mcp"`
			} `yaml:"backend"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(docs[0], &parsed); err != nil {
		t.Fatalf("unmarshaling rendered doc: %v", err)
	}
	if parsed.Kind != "EnterpriseAgentgatewayPolicy" {
		t.Fatalf("expected kind EnterpriseAgentgatewayPolicy, got %q", parsed.Kind)
	}
	if parsed.Spec.TargetRefs[0].Name != "core-banking-standard-backend" {
		t.Fatalf("expected targetRef core-banking-standard-backend, got %q", parsed.Spec.TargetRefs[0].Name)
	}
	exprs := parsed.Spec.Backend.MCP.Authorization.Policy.MatchExpressions
	if len(exprs) != 3 || exprs[2] != `mcp.tool.name == "get_account_detail" && jwt.is_org_admin == "true"` {
		t.Fatalf("unexpected matchExpressions: %v", exprs)
	}
}

func TestSettlementRBACTemplateRendersValidPolicy(t *testing.T) {
	docs, err := Render("auth/settlement-rbac.yaml.tmpl", Params{Namespace: "agentgateway-system"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	var parsed struct {
		Kind string `yaml:"kind"`
		Spec struct {
			TargetRefs []struct {
				Name string `yaml:"name"`
			} `yaml:"targetRefs"`
			Backend struct {
				MCP struct {
					Authorization struct {
						Policy struct {
							MatchExpressions []string `yaml:"matchExpressions"`
						} `yaml:"policy"`
					} `yaml:"authorization"`
				} `yaml:"mcp"`
			} `yaml:"backend"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(docs[0], &parsed); err != nil {
		t.Fatalf("unmarshaling rendered doc: %v", err)
	}
	if parsed.Spec.TargetRefs[0].Name != "core-banking-delegation-backend" {
		t.Fatalf("expected targetRef core-banking-delegation-backend, got %q", parsed.Spec.TargetRefs[0].Name)
	}
	exprs := parsed.Spec.Backend.MCP.Authorization.Policy.MatchExpressions
	want := `mcp.tool.name == "transfer" && jwt.act.act.sub == "system:serviceaccount:agentgateway-system:transfer-orchestrator-agent"`
	if len(exprs) != 1 || exprs[0] != want {
		t.Fatalf("unexpected matchExpressions: %v", exprs)
	}
}
