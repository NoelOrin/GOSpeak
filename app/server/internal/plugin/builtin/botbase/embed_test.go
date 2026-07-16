package botbase

import (
	"testing"
)

func TestHasEmbeddedAssets(t *testing.T) {
	if !HasEmbeddedAssets() {
		t.Fatal("expected bot-base assets to be embedded")
	}
}

func TestLoadEmbeddedDefaultConfig(t *testing.T) {
	cfg, err := loadEmbeddedDefaultConfig()
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}
	if len(cfg.LLMProviders) == 0 {
		t.Fatal("expected default llm providers")
	}
	foundGemini := false
	for _, p := range cfg.LLMProviders {
		if p.Protocol == "gemini-response" {
			foundGemini = true
		}
	}
	if !foundGemini {
		t.Fatal("expected gemini-response provider in embedded defaults")
	}
}

func TestLoadEmbeddedManifest(t *testing.T) {
	m, err := loadEmbeddedManifest()
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if m["name"] != "bot-base" {
		t.Fatalf("name=%v", m["name"])
	}
	if m["embedded"] != true {
		t.Fatalf("embedded=%v", m["embedded"])
	}
}

func TestNewUsesEmbeddedDefaults(t *testing.T) {
	p := New()
	if p == nil {
		t.Fatal("nil plugin")
	}
	meta := p.Meta()
	if meta.Name != Name {
		t.Fatalf("meta name=%s", meta.Name)
	}
	if meta.Kind != "builtin" {
		t.Fatalf("kind=%s", meta.Kind)
	}
}
