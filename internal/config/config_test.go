package config

import "testing"

func TestLoadAndMissing(t *testing.T) {
	t.Setenv("AGW_HOST", "agentgateway.demo.example.com")
	t.Setenv("KEYCLOAK_HOST", "")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("OAUTH_TOKEN_EXCHANGE_CLIENT_SECRET", "")
	t.Setenv("AGW_DEMO_WIZARD_USER_PASSWORD", "")

	cfg := Load()

	if cfg.GatewayHost != "agentgateway.demo.example.com" {
		t.Fatalf("unexpected GatewayHost: %q", cfg.GatewayHost)
	}

	missing := cfg.Missing()
	want := []string{"KEYCLOAK_HOST", "OAUTH_TOKEN_EXCHANGE_CLIENT_SECRET", "AGW_DEMO_WIZARD_USER_PASSWORD"}
	if len(missing) != len(want) {
		t.Fatalf("expected %v, got %v", want, missing)
	}
	for i, name := range want {
		if missing[i] != name {
			t.Fatalf("expected %v, got %v", want, missing)
		}
	}
}

func TestLoadDefaultsMCPImagesWhenUnset(t *testing.T) {
	t.Setenv("STOCK_SERVER_MCP_IMAGE", "")
	t.Setenv("CURRENCY_SERVER_MCP_IMAGE", "")
	t.Setenv("EXT_MCP_GUARDRAIL_IMAGE", "")

	cfg := Load()

	if cfg.StockMCPImage != defaultStockMCPImage {
		t.Fatalf("expected the default stock MCP image, got %q", cfg.StockMCPImage)
	}
	if cfg.CurrencyMCPImage != defaultCurrencyMCPImage {
		t.Fatalf("expected the default currency MCP image, got %q", cfg.CurrencyMCPImage)
	}
	if cfg.ExtMCPGuardrailImage != defaultExtMCPGuardrailImage {
		t.Fatalf("expected the default ext-mcp-guardrail image, got %q", cfg.ExtMCPGuardrailImage)
	}
	if len(cfg.Missing()) == 0 {
		t.Fatalf("expected other required env vars to still be reported missing in this test")
	}
}

func TestLoadRespectsMCPImageOverride(t *testing.T) {
	t.Setenv("STOCK_SERVER_MCP_IMAGE", "my-registry.example.com/stock-server-mcp:custom")
	t.Setenv("EXT_MCP_GUARDRAIL_IMAGE", "my-registry.example.com/ext-mcp-guardrail:custom")

	cfg := Load()

	if cfg.StockMCPImage != "my-registry.example.com/stock-server-mcp:custom" {
		t.Fatalf("expected the overridden image, got %q", cfg.StockMCPImage)
	}
	if cfg.ExtMCPGuardrailImage != "my-registry.example.com/ext-mcp-guardrail:custom" {
		t.Fatalf("expected the overridden ext-mcp-guardrail image, got %q", cfg.ExtMCPGuardrailImage)
	}
}

func TestMissingAllConfigured(t *testing.T) {
	t.Setenv("AGW_HOST", "agentgateway.demo.example.com")
	t.Setenv("KEYCLOAK_HOST", "keycloak.demo.example.com")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("OAUTH_TOKEN_EXCHANGE_CLIENT_SECRET", "sk-test-exchange-secret")
	t.Setenv("AGW_DEMO_WIZARD_USER_PASSWORD", "test-demo-password")

	cfg := Load()
	missing := cfg.Missing()

	if missing == nil {
		t.Fatalf("expected non-nil empty slice, got nil slice")
	}
	if len(missing) != 0 {
		t.Fatalf("expected 0 missing vars, got %d: %v", len(missing), missing)
	}
}

func TestLoadDefaultsSTSHostWhenUnset(t *testing.T) {
	t.Setenv("STS_HOST", "")

	cfg := Load()

	if cfg.STSHost != defaultSTSHost {
		t.Fatalf("expected the default STS host, got %q", cfg.STSHost)
	}
}

func TestLoadRespectsSTSHostOverride(t *testing.T) {
	t.Setenv("STS_HOST", "sts.other-namespace.svc.cluster.local")

	cfg := Load()

	if cfg.STSHost != "sts.other-namespace.svc.cluster.local" {
		t.Fatalf("expected the overridden STS host, got %q", cfg.STSHost)
	}
}

func TestLoadDefaultsSTSPortWhenUnset(t *testing.T) {
	t.Setenv("STS_PORT", "")

	cfg := Load()

	if cfg.STSPort != defaultSTSPort {
		t.Fatalf("expected the default STS port, got %q", cfg.STSPort)
	}
}

func TestLoadRespectsSTSPortOverride(t *testing.T) {
	t.Setenv("STS_PORT", "9999")

	cfg := Load()

	if cfg.STSPort != "9999" {
		t.Fatalf("expected the overridden STS port, got %q", cfg.STSPort)
	}
}

func TestLoadDefaultsOpsAgentURLsWhenUnset(t *testing.T) {
	t.Setenv("ACCOUNTS_AGENT_URL", "")
	t.Setenv("TRANSFER_ORCHESTRATOR_AGENT_URL", "")
	t.Setenv("STATEMENTS_AGENT_URL", "")

	cfg := Load()

	if cfg.AccountsAgentURL != defaultAgentGatewayInternalURL+"/accounts-agent/run" {
		t.Fatalf("unexpected default AccountsAgentURL: %q", cfg.AccountsAgentURL)
	}
	if cfg.TransferOrchestratorAgentURL != defaultAgentGatewayInternalURL+"/transfer-orchestrator-agent/run" {
		t.Fatalf("unexpected default TransferOrchestratorAgentURL: %q", cfg.TransferOrchestratorAgentURL)
	}
	if cfg.StatementsAgentURL != defaultAgentGatewayInternalURL+"/statements-agent/run" {
		t.Fatalf("unexpected default StatementsAgentURL: %q", cfg.StatementsAgentURL)
	}
}

func TestLoadRespectsOpsAgentURLOverrides(t *testing.T) {
	t.Setenv("ACCOUNTS_AGENT_URL", "http://accounts-agent.other-namespace.svc.cluster.local:8080/run")

	cfg := Load()

	if cfg.AccountsAgentURL != "http://accounts-agent.other-namespace.svc.cluster.local:8080/run" {
		t.Fatalf("expected the overridden AccountsAgentURL, got %q", cfg.AccountsAgentURL)
	}
}

func TestLoadDefaultsTenantFieldsWhenUnset(t *testing.T) {
	t.Setenv("TENANTS_GATEWAY_INTERNAL_URL", "")
	t.Setenv("TENANT_ACME_CLIENT_SECRET", "")
	t.Setenv("TENANT_GLOBEX_CLIENT_SECRET", "")
	t.Setenv("TENANT_INITECH_CLIENT_SECRET", "")

	cfg := Load()

	if cfg.TenantsGatewayInternalURL != defaultTenantsGatewayInternalURL {
		t.Fatalf("expected the default tenants gateway URL, got %q", cfg.TenantsGatewayInternalURL)
	}
	if cfg.TenantAcmeClientSecret != defaultTenantAcmeClientSecret {
		t.Fatalf("expected the default acme client secret, got %q", cfg.TenantAcmeClientSecret)
	}
	if cfg.TenantGlobexClientSecret != defaultTenantGlobexClientSecret {
		t.Fatalf("expected the default globex client secret, got %q", cfg.TenantGlobexClientSecret)
	}
	if cfg.TenantInitechClientSecret != defaultTenantInitechClientSecret {
		t.Fatalf("expected the default initech client secret, got %q", cfg.TenantInitechClientSecret)
	}

	for _, name := range cfg.Missing() {
		switch name {
		case "TENANTS_GATEWAY_INTERNAL_URL", "TENANT_ACME_CLIENT_SECRET", "TENANT_GLOBEX_CLIENT_SECRET", "TENANT_INITECH_CLIENT_SECRET":
			t.Fatalf("expected %q to never appear in Missing(), got %v", name, cfg.Missing())
		}
	}
}

func TestLoadRespectsTenantFieldOverrides(t *testing.T) {
	t.Setenv("TENANTS_GATEWAY_INTERNAL_URL", "http://agentgateway-tenants.other-namespace.svc.cluster.local:8080")
	t.Setenv("TENANT_ACME_CLIENT_SECRET", "override-acme-secret")

	cfg := Load()

	if cfg.TenantsGatewayInternalURL != "http://agentgateway-tenants.other-namespace.svc.cluster.local:8080" {
		t.Fatalf("expected the overridden tenants gateway URL, got %q", cfg.TenantsGatewayInternalURL)
	}
	if cfg.TenantAcmeClientSecret != "override-acme-secret" {
		t.Fatalf("expected the overridden acme client secret, got %q", cfg.TenantAcmeClientSecret)
	}
}
