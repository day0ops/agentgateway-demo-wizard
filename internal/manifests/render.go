// Package manifests renders the wizard's own embedded agentgateway CRD
// templates into applyable YAML documents. Every step's Configure-pane
// policies point at a template under templates/, authored against the real
// enterprise agentgateway CRD shapes.
package manifests

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed templates
var templatesFS embed.FS

// Params are the template variables available to every manifest template.
type Params struct {
	Namespace                      string
	GatewayName                    string
	GatewayNS                      string
	GatewayHost                    string
	KeycloakHost                   string
	OpenAIKey                      string
	StockMCPImage                  string
	CurrencyMCPImage               string
	OAuthTokenExchangeClientSecret string
}

// Render executes the template at path (relative to templates/) with params
// and splits the result into individual YAML documents on "---" separators.
// Blank documents are dropped.
func Render(path string, params Params) ([][]byte, error) {
	raw, err := templatesFS.ReadFile("templates/" + path)
	if err != nil {
		return nil, fmt.Errorf("reading template %q: %w", path, err)
	}

	tmpl, err := template.New(path).Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("parsing template %q: %w", path, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return nil, fmt.Errorf("executing template %q: %w", path, err)
	}

	var docs [][]byte
	for _, doc := range strings.Split(buf.String(), "\n---\n") {
		if strings.TrimSpace(doc) == "" {
			continue
		}
		docs = append(docs, []byte(doc))
	}
	return docs, nil
}
