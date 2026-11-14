import { defineConfig } from "mygo-cli";

// MyGo project config for the GOSpeak desktop client.
// Docs: https://mygo.egoist.dev
//
// 连接模型（重要）：桌面端自带前端、不依赖后端托管页面 ——
//   - release：宿主在 127.0.0.1:<port> 起本地回环反向代理（app/desktop/proxyserver.go），
//     窗口加载该地址：代理同源托管本目录复制进来的前端产物，并把 /api /ws /uploads
//     转发到用户配置的服务器地址（持久化于用户数据目录）。页面/接口/WS 全程同源，无跨域。
//   - dev：设 GOSPEAK_DESKTOP_DEV=1 后窗口直接加载下方 devUrl（Vite 自带 /api /ws
//     代理到本地后端，同样同源）。与 app/desktop/windows.go 的 devWindowURL 保持一致。
//
// frontendDist 仍由 `mygo build` 打包（满足 CLI 要求并作为 mygo:// 兜底资源），
// 但 release 窗口实际由回环代理托管前端，因此构建后需要把产物同步到
// app/desktop/webui/dist（go:embed），见下方 buildCommand。
export default defineConfig({
	// Human-readable app name.
	name: "GOSpeak",
	// Release version: names the .app / dmg / installer artifacts. Keep in sync
	// with .github/versions/desktop.yaml (release CI overrides via -ldflags too).
	version: "1.0.2",
	// Reverse-DNS bundle identifier used for the packaged app / auto-update.
	identifier: "dev.gospeak.desktop",
	// 深链协议：gospeak://invite/d/<CODE> 由打包注册（CFBundleURLTypes / 安装器
	// 注册表 / .desktop MimeType），未打包运行时由 app/desktop/deeplink.go 的
	// registerDeepLink 兜底注册。
	urlSchemes: ["gospeak"],
	// Dev server of the web frontend (@gospeak/web, vite port 3000). Start it
	// first with `pnpm dev:web`; launch mygo with GOSPEAK_DESKTOP_DEV=1.
	devUrl: "http://localhost:3000",
	// Build the web frontend in DESKTOP target (conditional-compilation
	// constants: __GOSPEAK_DESKTOP__=true, desktop branches included) and sync
	// it into the Go embed directory consumed by proxyserver.go
	// (//go:embed all:webui/dist). Web 构建不带该变量即自动剔除桌面分支。
	//
	// 最后一步为每个文本资源生成 .gz 边车（gzip -9，保留原文件）：宿主直接
	// 发送预压缩字节，启动时既不压缩也不读写临时缓冲，首屏 CPU 归零（见
	// proxyserver.go 的 assetCache）；缺少 .gz 时宿主仍会运行时压缩兜底。
	buildCommand:
		"GOSPEAK_TARGET=desktop pnpm --filter @gospeak/web build && " +
		"rm -rf webui/dist && mkdir -p webui/dist && cp -R ../web/dist/. webui/dist/ && " +
		"find webui/dist -type f \\( -name '*.js' -o -name '*.css' -o -name '*.html' " +
		"-o -name '*.json' -o -name '*.svg' -o -name '*.txt' -o -name '*.xml' " +
		"-o -name '*.map' -o -name '*.wasm' \\) -exec gzip -9 -k -f {} \\;",
	frontendDist: "webui/dist",
	// macOS 麦克风/摄像头用途说明必须写进 Info.plist：没有它，系统会直接拒绝
	// getUserMedia 的请求（界面上表现为"读取音频设备失败"），而不是弹权限框。
	// 之前是打包后手工用 PlistBuddy 注入的，重新打包就会丢，所以落到配置里。
	macos: {
		infoPlist: {
			NSMicrophoneUsageDescription: "GOSpeak 需要使用麦克风进行游戏语音通话。",
			NSCameraUsageDescription: "GOSpeak 需要使用摄像头进行视频通话。",
		},
	},
});
