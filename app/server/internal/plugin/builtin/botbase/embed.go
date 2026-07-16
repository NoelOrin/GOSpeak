package botbase

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
)

// assets 在编译期嵌入二进制，发布产物无需额外携带 bot 基础插件文件。
//
//go:embed all:assets
var embeddedAssets embed.FS

// EmbeddedFS 返回 bot-base 内嵌资源（manifest / default_config 等）。
func EmbeddedFS() fs.FS {
	sub, err := fs.Sub(embeddedAssets, "assets")
	if err != nil {
		return embeddedAssets
	}
	return sub
}

func loadEmbeddedManifest() (map[string]any, error) {
	b, err := embeddedAssets.ReadFile("assets/manifest.json")
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse embedded manifest: %w", err)
	}
	return m, nil
}

func loadEmbeddedDefaultConfig() (Config, error) {
	b, err := embeddedAssets.ReadFile("assets/default_config.json")
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse embedded default config: %w", err)
	}
	return cfg, nil
}

// HasEmbeddedAssets 供打包/健康检查确认插件已嵌入。
func HasEmbeddedAssets() bool {
	_, err := embeddedAssets.Open("assets/manifest.json")
	return err == nil
}
