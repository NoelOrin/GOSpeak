package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"GOSpeak/internal/genprotocol"
)

func main() {
	var updateGolden bool
	flag.BoolVar(&updateGolden, "update-golden", false, "overwrite testdata/golden instead of app/web/src/protocol/generated")
	flag.Parse()

	// go run 的 cwd 是 app/server（模块根），路径以此为基准
	pkgDir := filepath.Join("internal", "protocol")
	outDir := filepath.Join("..", "web", "src", "protocol", "generated")
	if updateGolden {
		outDir = filepath.Join("internal", "genprotocol", "testdata", "golden")
	}
	files, err := genprotocol.Generate(pkgDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gen-protocol: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "gen-protocol: %v\n", err)
		os.Exit(1)
	}
	for _, name := range []string{"events.ts", "payloads.ts", "envelope.ts"} {
		if err := os.WriteFile(filepath.Join(outDir, name), []byte(files[name]), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "gen-protocol: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("gen-protocol: wrote %d files to %s\n", len(files), outDir)
}
