// Package config resolves the wizard's runtime configuration from environment
// variables and reports which required values are missing, so the UI can show
// a setup banner instead of failing opaquely on first use.
package config

import "os"

const (
	defaultStockMCPImage    = "australia-southeast1-docker.pkg.dev/field-engineering-apac/kasunt/stock-server-mcp:0.1.1"
	defaultCurrencyMCPImage = "australia-southeast1-docker.pkg.dev/field-engineering-apac/kasunt/currency-server-mcp:0.1.1"
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
)

// Config holds the wizard's runtime configuration.
type Config struct {
	GatewayHost                    string
	KeycloakHost                   string
	OpenAIKey                      string
	StockMCPImage                  string
	CurrencyMCPImage               string
	OAuthTokenExchangeClientSecret string
	DemoUserPassword               string
	STSHost                        string
	STSPort                        string
	RepoAgentURL                   string
	ReleaseManagerAgentURL         string
	GitHubAgentURL                 string
}

// Load reads configuration from environment variables. It never errors:
// missing values are left empty and surfaced later via Missing.
// StockMCPImage/CurrencyMCPImage are never "missing" - they default to
// Solo's internal demo images and are only overridden if a cluster can't
// pull from that private registry.
func Load() *Config {
	return &Config{
		GatewayHost:                    os.Getenv("AGW_HOST"),
		KeycloakHost:                   os.Getenv("KEYCLOAK_HOST"),
		OpenAIKey:                      os.Getenv("OPENAI_API_KEY"),
		StockMCPImage:                  envOrDefault("STOCK_SERVER_MCP_IMAGE", defaultStockMCPImage),
		CurrencyMCPImage:               envOrDefault("CURRENCY_SERVER_MCP_IMAGE", defaultCurrencyMCPImage),
		OAuthTokenExchangeClientSecret: os.Getenv("OAUTH_TOKEN_EXCHANGE_CLIENT_SECRET"),
		DemoUserPassword:               os.Getenv("DEMO_USER_PASSWORD"),
		STSHost:                        envOrDefault("STS_HOST", defaultSTSHost),
		STSPort:                        envOrDefault("STS_PORT", defaultSTSPort),
		RepoAgentURL:                   envOrDefault("REPO_AGENT_URL", defaultAgentGatewayInternalURL+"/repo-agent/run"),
		ReleaseManagerAgentURL:         envOrDefault("RELEASE_MANAGER_AGENT_URL", defaultAgentGatewayInternalURL+"/release-manager-agent/run"),
		GitHubAgentURL:                 envOrDefault("GITHUB_AGENT_URL", defaultAgentGatewayInternalURL+"/github-agent/run"),
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
		missing = append(missing, "DEMO_USER_PASSWORD")
	}
	return missing
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
