package genprotocol

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGolden(t *testing.T) {
	pkgDir := filepath.Join("..", "protocol")
	files, err := Generate(pkgDir)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, name := range []string{"events.ts", "payloads.ts", "envelope.ts"} {
		want, err := os.ReadFile(filepath.Join("testdata", "golden", name))
		if err != nil {
			t.Fatalf("read golden %s: %v", name, err)
		}
		if files[name] != string(want) {
			t.Errorf("%s drifted from golden; run go run ./cmd/gen-protocol -update-golden and commit", name)
		}
	}
}

func TestEventsHaveNoInternal(t *testing.T) {
	files, err := Generate(filepath.Join("..", "protocol"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(files["events.ts"], `"connection"`) || strings.Contains(files["events.ts"], `"disconnect"`) {
		t.Error("internal events must not be emitted to TS")
	}
}
