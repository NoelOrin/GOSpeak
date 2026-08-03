package botbase

import (
	"strings"
	"testing"
)

func TestSecretValueRoundTrip(t *testing.T) {
	enc, err := encryptSecretValue("sk-secret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if !strings.HasPrefix(enc, secretPrefix) {
		t.Fatalf("expected encrypted prefix, got %s", enc)
	}
	if got := decryptSecretValue(enc); got != "sk-secret" {
		t.Fatalf("decrypt = %q, want sk-secret", got)
	}
}

func TestValidateConfigEncryptsAPIKeys(t *testing.T) {
	p := New()
	raw := map[string]any{
		"llm_providers": []any{
			map[string]any{
				"name":     "openai",
				"protocol": "openai-compatible",
				"api_key":  "secret",
			},
		},
	}
	norm, err := p.ValidateConfig(raw)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	providers, ok := norm["llm_providers"].([]any)
	if !ok || len(providers) != 1 {
		t.Fatalf("unexpected normalized providers: %#v", norm["llm_providers"])
	}
	provider := providers[0].(map[string]any)
	key, _ := provider["api_key"].(string)
	if key == "secret" || !strings.HasPrefix(key, secretPrefix) {
		t.Fatalf("api_key must be encrypted at rest, got %q", key)
	}

	// ValidateConfig 不得原地改写调用方传入的配置。
	rawProvider := raw["llm_providers"].([]any)[0].(map[string]any)
	if rawProvider["api_key"] != "secret" {
		t.Fatalf("ValidateConfig mutated caller config: %v", rawProvider["api_key"])
	}

	p.applyConfig(norm)
	if len(p.cfg.LLMProviders) != 1 || p.cfg.LLMProviders[0].APIKey != "secret" {
		t.Fatalf("in-memory config must keep plaintext api_key, got %+v", p.cfg.LLMProviders)
	}

	// applyConfig 不得把 norm 原地解密，否则 SaveConfig 会落库明文。
	provider = providers[0].(map[string]any)
	if provider["api_key"] != key {
		t.Fatalf("applyConfig mutated encrypted config: %v", provider["api_key"])
	}
}
