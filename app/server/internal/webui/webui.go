package webui

import (
	"embed"
	"io/fs"
	"net/http"
)

// dist 在构建时由前端产物填充；仓库内保留 EMBED_PLACEHOLDER 保证 go:embed 合法。
//
//go:embed all:dist
var embedded embed.FS

// HasAssets 报告嵌入式前端是否可用（至少包含 index.html）。
func HasAssets() bool {
	_, err := embedded.Open("dist/index.html")
	return err == nil
}

// FS 返回以 dist 根目录为根的 http.FileSystem。
// 若未嵌入有效前端，返回 nil。
func FS() http.FileSystem {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil
	}
	if !HasAssets() {
		return nil
	}
	return http.FS(sub)
}

// FileSystem 返回以 dist 根目录为根的 fs.FS，供自定义路由使用。
func FileSystem() (fs.FS, bool) {
	if !HasAssets() {
		return nil, false
	}
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil, false
	}
	return sub, true
}
