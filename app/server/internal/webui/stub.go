//go:build noembedweb

package webui

import (
	"io/fs"
	"net/http"
)

// HasAssets 在 noembedweb 构建标签下恒为 false。
// 开发模式不嵌入前端，前端由 Vite 独立托管。
func HasAssets() bool {
	return false
}

// FS 在 noembedweb 构建标签下返回 nil。
func FS() http.FileSystem {
	return nil
}

// FileSystem 在 noembedweb 构建标签下返回空。
func FileSystem() (fs.FS, bool) {
	return nil, false
}
