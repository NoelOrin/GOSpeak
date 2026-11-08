# GOSpeak Desktop（`app/desktop`）

GOSpeak 桌面端的**本体**：一个用 [MyGo](https://github.com/egoist/mygo)（纯 Go 桌面框架）写的原生宿主程序，内嵌现有的 `@gospeak/web` 前端。

## 为什么是 MyGo

- 纯 Go、无 cgo，编译出几 MB 的单文件二进制，跨平台（macOS / Linux / Windows）。
- 前端用任意 Web 工具（我们就是 React 19 的 `@gospeak/web`），跑在系统 WebView 里。
- 关键能力：**从 Go 代码生成类型化的 TypeScript 客户端**——前端调用 Go 服务时两端类型对齐。
- 自带桌面 API（窗口、菜单、托盘、通知、全局快捷键、深链、对话框、自动更新）。

## 架构

```
┌─────────────────────────────────────────────────────────────┐
│  app/desktop  (Go + MyGo 宿主)                                │
│   main.go 绑定 Go 服务 → mygo 生成 TS 客户端                   │
│   window.go / tray.go / notification.go / autostart.go /      │
│   deeplink.go / dialog.go / updater.go  ←  原生能力实现        │
│        │  WebView 嵌入                                        │
│        ▼                                                      │
│  @gospeak/web (React 19 前端，dev: localhost:3000)            │
│        │  通过 mygo 生成的客户端调用 Go 服务                   │
│        ▼                                                      │
│  @gospeak/desktop (TS 能力 SDK + 浏览器兜底)  ←  packages/desktop│
└─────────────────────────────────────────────────────────────┘
```

- `app/desktop`（本目录）= 桌面端**本体** + Go 侧的原生能力实现。
- `packages/desktop` = TypeScript 侧的**能力 SDK**：包装 mygo 生成的客户端，并给浏览器环境提供兜底实现，让同一套前端代码在桌面端和纯浏览器里都能跑。

## 目录

| 文件 | 作用 |
| --- | --- |
| `go.mod` | Go module（`gospeak-desktop`），依赖 `github.com/egoist/mygo` |
| `main.go` | 入口：`mygo.Bind(...)` 注册服务 + 创建内嵌前端的窗口 |
| `window.go` / `tray.go` / `notification.go` / `autostart.go` / `deeplink.go` / `dialog.go` / `updater.go` | 桌面能力服务的 Go 实现（mygo 绑定后生成 TS 客户端） |
| `deeplink.go` | `gospeak://invite/d/<CODE>` 深链：注册协议、归一化为应用内路径、冷启动暂存与运行中广播 |
| `mygo.config.ts` | MyGo 项目配置（应用名、identifier、devUrl 指向 `@gospeak/web`） |
| `package.json` | 通过 `mygo-cli` 提供 `mygo dev` / `mygo build` |
| `resources/icon.png` | 打包用应用图标（1024×1024），MyGo 生成 .icns / .ico / .desktop 图标 |
| `assets/` | go:embed 的运行时图标：`icon.png`（Dock/窗口兜底）、`tray.png`（单色托盘模板图） |
| `assets.go` | 读取内嵌图标并应用到 Dock / 窗口 / 托盘 |
| `proxyport.go` | 回环代理的稳定端口选择与持久化（`proxy-port.json`） |
| `lifecycle.go` | 关窗进托盘、Dock 激活、单实例聚焦与退出标记 |

## 前提

- **Go 1.27+**（MyGo 要求）。
- **Bun 或 npm**（前端工具链；本仓库用 pnpm，MyGo CLI 通过 `mygo-cli` 提供 `mygo` 二进制）。
- 先装依赖：`go mod download` 拉取 `github.com/egoist/mygo`；`pnpm install` 装 `mygo-cli`。

## 开发

```bash
# 1) 起前端（另一个终端）
pnpm dev:web                  # @gospeak/web @ localhost:3000

# 2) 起桌面宿主（WebView 指向上面的 dev server，带热重载）
pnpm --filter @gospeak/desktop-app dev
```

## 打包

`mygo build` 会先跑 `mygo.config.ts` 的 `buildCommand`（按桌面目标构建前端并同步进
`webui/dist`），再产出各平台安装包：

```bash
pnpm --filter @gospeak/desktop-app build
```

| 平台 | 产物 |
|------|------|
| macOS | `dist/darwin-<arch>/GOSpeak.app` + `gospeak-<ver>-<arch>.dmg`（拖入 Applications） |
| Linux | `dist/linux-<arch>/gospeak_<ver>_<arch>.deb`、`.tar.gz`、`install.sh`、`PKGBUILD` |
| Windows | `dist/windows-<arch>/gospeak-<ver>-<arch>-setup.exe`（NSIS 安装包） |

产物落在 `dist/` 下（已被 gitignore）。只想出一个平台：

```bash
cd app/desktop && ./node_modules/.bin/mygo build -platform darwin/arm64
```

### 版本号注入

`mygo build` 用自己生成的 ldflags，直接传 `-ldflags` 会被覆盖。用 `MYGO_LDFLAGS`
环境变量，它会拼在 mygo 自己的 flags 之后：

```bash
MYGO_LDFLAGS="-X 'main.version=1.2.3' -X 'main.commit=abc1234'" \
  ./node_modules/.bin/mygo build
```

发布链（`.github/workflows/release.yml`）就是这么注入 tag 与 commit 的。

### 常见坑

- **DMG 只能在 macOS 上构建**：`buildDMG` 依赖 `hdiutil` / `ditto` / `diskutil`。
  所以发布链把 macOS 单独放在 `macos-latest` 作业，Linux/Windows 在 `ubuntu-latest`。
- **macOS 必须发 `.app`/`.dmg`，不能发裸二进制**：裸 Mach-O 没有 Info.plist，
  也就没有 `NSMicrophoneUsageDescription`，系统会直接拒绝 `getUserMedia`，
  用户看到的报错是「读取音频设备失败」。用途说明写在 `mygo.config.ts` 的
  `macos.infoPlist` 里。
- **Windows 安装包需要 NSIS**：mygo 只在 Windows 上自动下载 `makensis`，在 Linux/macOS
  上必须预装（Debian/Ubuntu `apt install nsis`，macOS `brew install nsis`）。
  缺失时 mygo 只打一条 warning 就继续，产物里只有裸 exe 而构建仍报成功 ——
  发布链因此加了显式的产物断言。
- 产物是 ad-hoc 签名，未公证。要消除 Gatekeeper 提示需配置
  `macos.signingIdentity`（Developer ID）与 `macos.notarize`。

## 接入新能力

1. 在对应的 `*.go` 里给结构体加一个导出方法（普通参数 + 返回值/error）。
2. 在 `main.go` 里 `mygo.Bind` 该结构体（已绑定的类型无需重复）。
3. 在 `packages/desktop/src/capabilities/` 里加/改对应的能力封装，命令名用
   `command("<Struct>", "<Method>")` 构造（与 Go 侧逐字一致）。
4. 跑 `node scripts/check-desktop-commands.mjs` 确认命令名对得上。

> 注意：命令名写错时 MyGo 只会返回 "method X is not bound" 的 rejected Promise，
> 而多数能力封装会静默吞掉错误——所以必须靠上面的校验脚本兜底。
