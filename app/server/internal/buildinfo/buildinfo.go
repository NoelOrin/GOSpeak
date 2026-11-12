// Package buildinfo 承载构建时注入的版本信息。
//
// Dockerfile 与 .github/workflows/release.yml 通过 ldflags 注入：
//
//	-X 'GOSpeak/internal/buildinfo.Version=<ver>' -X 'GOSpeak/internal/buildinfo.Commit=<sha>'
//
// cmd（cobra version 命令）与 internal/service（系统更新检查）均从此处读取，
// 避免 internal 反向依赖入口包。
package buildinfo

var (
	// Version 默认 dev，构建时注入（以仓库顶层 VERSION / v* tag 为准）。
	Version = "dev"
	// Commit 默认 unknown，构建时注入。
	Commit = "unknown"
)
