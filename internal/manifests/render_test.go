package manifests

import (
	"strings"
	"testing"
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
