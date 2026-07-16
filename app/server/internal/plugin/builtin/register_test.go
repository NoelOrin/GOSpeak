package builtin

import (
	"testing"

	"GOSpeak/internal/plugin"
)

func TestRegisterAll_IncludesBotBase(t *testing.T) {
	host := plugin.NewHost(nil, nil, nil)
	reg := plugin.NewRegistry(host)
	if err := RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	p, ok := reg.Get("bot-base")
	if !ok {
		t.Fatal("bot-base not registered")
	}
	if p.Meta().Name != "bot-base" {
		t.Fatalf("name=%s", p.Meta().Name)
	}
	sum := EmbeddedSummary()
	if len(sum) == 0 || sum[0]["embedded"] != true {
		t.Fatalf("embedded summary=%v", sum)
	}
}
