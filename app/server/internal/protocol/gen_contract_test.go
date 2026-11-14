package protocol

import (
	"os"
	"path/filepath"
	"testing"

	gen "GOSpeak/internal/genprotocol"
)

// TestGeneratedArtifactsFresh 重跑生成器与已提交产物逐字节比对：
// 改了 protocol 包忘记重新生成时，本地测试直接红，不等 CI。
func TestGeneratedArtifactsFresh(t *testing.T) {
	files, err := gen.Generate(".")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// 测试 cwd = app/server/internal/protocol，产物在仓库 app/web 下（上溯四级）
	outDir := filepath.Join("..", "..", "..", "..", "app", "web", "src", "protocol", "generated")
	for _, name := range []string{"events.ts", "payloads.ts", "envelope.ts"} {
		want, err := os.ReadFile(filepath.Join(outDir, name))
		if err != nil {
			t.Fatalf("read committed %s: %v（跑 pnpm gen:protocol）", name, err)
		}
		if files[name] != string(want) {
			t.Errorf("%s stale — 跑 pnpm gen:protocol 并提交产物", name)
		}
	}
}
