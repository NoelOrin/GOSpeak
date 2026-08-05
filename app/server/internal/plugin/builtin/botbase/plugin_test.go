package botbase

import "testing"

func TestBotbase_RejectsBadConfig(t *testing.T) {
	p := &Plugin{cfg: Config{
		LLMProviders: []LLMProviderConfig{{Name: "keep"}},
	}}
	p.applyConfig(map[string]any{"llm_providers": "not-an-array"})
	if len(p.cfg.LLMProviders) != 1 || p.cfg.LLMProviders[0].Name != "keep" {
		t.Fatalf("malformed config must be rejected and must not replace current config, got %+v", p.cfg)
	}
}
