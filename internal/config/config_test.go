package config

import "testing"

func TestLoadAndMissing(t *testing.T) {
	t.Setenv("AGW_HOST", "agentgateway.demo.example.com")
	t.Setenv("KEYCLOAK_HOST", "")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("OAUTH_TOKEN_EXCHANGE_CLIENT_SECRET", "")
	t.Setenv("DEMO_USER_PASSWORD", "")

	cfg := Load()

	if cfg.GatewayHost != "agentgateway.demo.example.com" {
		t.Fatalf("unexpected GatewayHost: %q", cfg.GatewayHost)
	}

	missing := cfg.Missing()
	want := []string{"KEYCLOAK_HOST", "OAUTH_TOKEN_EXCHANGE_CLIENT_SECRET", "DEMO_USER_PASSWORD"}
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

	cfg := Load()

	if cfg.StockMCPImage != defaultStockMCPImage {
		t.Fatalf("expected the default stock MCP image, got %q", cfg.StockMCPImage)
	}
	if cfg.CurrencyMCPImage != defaultCurrencyMCPImage {
		t.Fatalf("expected the default currency MCP image, got %q", cfg.CurrencyMCPImage)
	}
	if len(cfg.Missing()) == 0 {
		t.Fatalf("expected other required env vars to still be reported missing in this test")
	}
}

func TestLoadRespectsMCPImageOverride(t *testing.T) {
	t.Setenv("STOCK_SERVER_MCP_IMAGE", "my-registry.example.com/stock-server-mcp:custom")

	cfg := Load()

	if cfg.StockMCPImage != "my-registry.example.com/stock-server-mcp:custom" {
		t.Fatalf("expected the overridden image, got %q", cfg.StockMCPImage)
	}
}

func TestMissingAllConfigured(t *testing.T) {
	t.Setenv("AGW_HOST", "agentgateway.demo.example.com")
	t.Setenv("KEYCLOAK_HOST", "keycloak.demo.example.com")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("OAUTH_TOKEN_EXCHANGE_CLIENT_SECRET", "sk-test-exchange-secret")
	t.Setenv("DEMO_USER_PASSWORD", "test-demo-password")

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
