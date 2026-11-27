// Package config resolves the wizard's runtime configuration from environment
// variables and reports which required values are missing, so the UI can show
// a setup banner instead of failing opaquely on first use.
package config

import "os"

const (
	defaultStockMCPImage        = "australia-southeast1-docker.pkg.dev/field-engineering-apac/kasunt/stock-server-mcp:0.1.1"
	defaultCurrencyMCPImage     = "australia-southeast1-docker.pkg.dev/field-engineering-apac/kasunt/currency-server-mcp:0.1.1"
	defaultExtMCPGuardrailImage = "australia-southeast1-docker.pkg.dev/field-engineering-apac/kasunt/ext-mcp-guardrail:0.1.2"
	// defaultSTSHost/defaultSTSPort are Solo's enterprise-agentgateway Helm
	// chart's own in-cluster STS service name and port, confirmed against
	// agentgateway-field-kit's config/environments/aws-dev.yaml
	// (spec.internal.stsIssuer) - internal-only, never a public demo
	// hostname like GatewayHost/KeycloakHost, so they only need defaults,
	// never a "missing" check.
	defaultSTSHost = "enterprise-agentgateway.agentgateway-system.svc.cluster.local"
	defaultSTSPort = "7777"
	// defaultAgentGatewayInternalURL is agentgateway's own in-cluster Service
	// DNS name, matching field-kit's config/environments/aws-dev.yaml
	// (spec.internal.agentgateway) - the wizard pod runs in the same
	// cluster/namespace, so it reaches the ops-agents containers through the
	// same gateway a browser would use externally.
	defaultAgentGatewayInternalURL = "http://agentgateway-gw.agentgateway-system.svc.cluster.local:8080"
	// defaultTenantsGatewayInternalURL is the dedicated tenants Gateway's own
	// in-cluster Service DNS name (Gateway "agentgateway-tenants",
	// agentgateway-field-kit's demo-wizard usecase) - internal-only, like
	// defaultAgentGatewayInternalURL, so it only needs a default, never a
	// "missing" check.
	defaultTenantsGatewayInternalURL = "http://agentgateway-tenants.agentgateway-system.svc.cluster.local:8080"
	// defaultTenantAcmeClientSecret/defaultTenantGlobexClientSecret/
	// defaultTenantInitechClientSecret match the fixed client-credentials
	// secrets agentgateway-field-kit's keycloak addon seeds for each tenant
	// org realm - fixed, already-public-in-this-repo demo values, not
	// secrets requiring setup, so they only need defaults, never a
	// "missing" check.
	defaultTenantAcmeClientSecret    = "acme-team-eng-secret"
	defaultTenantGlobexClientSecret  = "globex-team-sales-secret"
	defaultTenantInitechClientSecret = "initech-team-dev-secret"
)

// Config holds the wizard's runtime configuration.
type Config struct {
	GatewayHost                    string
	KeycloakHost                   string
	OpenAIKey                      string
	StockMCPImage                  string
	CurrencyMCPImage               string
	ExtMCPGuardrailImage           string
	OAuthTokenExchangeClientSecret string
	DemoUserPassword               string
	STSHost                        string
	STSPort                        string
	AccountsAgentURL               string
	TransferOrchestratorAgentURL   string
	StatementsAgentURL             string
	TenantsGatewayInternalURL      string
	TenantAcmeClientSecret         string
	TenantGlobexClientSecret       string
	TenantInitechClientSecret      string
}

// Load reads configuration from environment variables. It never errors:
// missing values are left empty and surfaced later via Missing.
// StockMCPImage/CurrencyMCPImage/ExtMCPGuardrailImage are never "missing" - they default to
// Solo's internal demo images and are only overridden if a cluster can't
// pull from that private registry.
func Load() *Config {
	return &Config{
		GatewayHost:                    os.Getenv("AGW_HOST"),
		KeycloakHost:                   os.Getenv("KEYCLOAK_HOST"),
		OpenAIKey:                      os.Getenv("OPENAI_API_KEY"),
		StockMCPImage:                  envOrDefault("STOCK_SERVER_MCP_IMAGE", defaultStockMCPImage),
		CurrencyMCPImage:               envOrDefault("CURRENCY_SERVER_MCP_IMAGE", defaultCurrencyMCPImage),
		ExtMCPGuardrailImage:           envOrDefault("EXT_MCP_GUARDRAIL_IMAGE", defaultExtMCPGuardrailImage),
		OAuthTokenExchangeClientSecret: os.Getenv("OAUTH_TOKEN_EXCHANGE_CLIENT_SECRET"),
		DemoUserPassword:               os.Getenv("AGW_DEMO_WIZARD_USER_PASSWORD"),
		STSHost:                        envOrDefault("STS_HOST", defaultSTSHost),
		STSPort:                        envOrDefault("STS_PORT", defaultSTSPort),
		AccountsAgentURL:               envOrDefault("ACCOUNTS_AGENT_URL", defaultAgentGatewayInternalURL+"/accounts-agent/run"),
		TransferOrchestratorAgentURL:   envOrDefault("TRANSFER_ORCHESTRATOR_AGENT_URL", defaultAgentGatewayInternalURL+"/transfer-orchestrator-agent/run"),
		StatementsAgentURL:             envOrDefault("STATEMENTS_AGENT_URL", defaultAgentGatewayInternalURL+"/statements-agent/run"),
		TenantsGatewayInternalURL:      envOrDefault("TENANTS_GATEWAY_INTERNAL_URL", defaultTenantsGatewayInternalURL),
		TenantAcmeClientSecret:         envOrDefault("TENANT_ACME_CLIENT_SECRET", defaultTenantAcmeClientSecret),
		TenantGlobexClientSecret:       envOrDefault("TENANT_GLOBEX_CLIENT_SECRET", defaultTenantGlobexClientSecret),
		TenantInitechClientSecret:      envOrDefault("TENANT_INITECH_CLIENT_SECRET", defaultTenantInitechClientSecret),
	}
}

// Missing returns the names of required environment variables that are
// unset, in a fixed, stable order.
func (c *Config) Missing() []string {
	missing := []string{}
	if c.GatewayHost == "" {
		missing = append(missing, "AGW_HOST")
	}
	if c.KeycloakHost == "" {
		missing = append(missing, "KEYCLOAK_HOST")
	}
	if c.OpenAIKey == "" {
		missing = append(missing, "OPENAI_API_KEY")
	}
	if c.OAuthTokenExchangeClientSecret == "" {
		missing = append(missing, "OAUTH_TOKEN_EXCHANGE_CLIENT_SECRET")
	}
	if c.DemoUserPassword == "" {
		missing = append(missing, "AGW_DEMO_WIZARD_USER_PASSWORD")
	}
	return missing
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
