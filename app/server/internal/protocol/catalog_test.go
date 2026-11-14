package protocol

import "testing"

func TestCatalogIntegrity(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range Catalog {
		if e.Name == "" {
			t.Fatal("catalog entry with empty Name")
		}
		if seen[e.Name] {
			t.Fatalf("duplicate event name: %s", e.Name)
		}
		seen[e.Name] = true
		switch e.Dir {
		case DirClientToServer, DirServerToClient, DirBoth, DirInternal:
		default:
			t.Fatalf("event %s: invalid dir %q", e.Name, e.Dir)
		}
		if e.Payload != "" && !payloadStructs[e.Payload] {
			t.Fatalf("event %s: payload %q not defined in package", e.Name, e.Payload)
		}
	}
	// 事件名必须等于常量值（防 Catalog 手抄漂移）
	for _, e := range Catalog {
		if v, ok := eventConsts[e.Name]; ok && v != e.Name {
			t.Fatalf("event %s: constant value mismatch %q", e.Name, v)
		}
	}
	// 每个 Catalog 事件必须登记在 eventConsts（防漏登记）
	for _, e := range Catalog {
		if _, ok := eventConsts[e.Name]; !ok {
			t.Fatalf("event %s: not registered in eventConsts", e.Name)
		}
	}
}
