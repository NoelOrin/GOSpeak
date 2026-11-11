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
| `proxyserver.go` | 本地回环反向代理：托管内嵌前端（gzip 边车 / 强缓存 / ETag）+ 转发 `/api` `/livekit` `/ws` 到配置的服务器 |
| `server_config.go` | 后端地址持久化（`server-config.json`，**权威**）；切换时清 Cookie/会话缓存并重载窗口。页面侧 `localStorage` 只是同步镜像，见 `packages/desktop` 的「服务器地址：双持久化」 |
| `overlay.go` | 游戏覆盖层窗口：按需创建、全局快捷键、显隐策略、成员状态接力 |
| `startup.go` | 启动耗时追踪（`GOSPEAK_STARTUP_DEBUG=1`，见「启动耗时」） |
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

### 自动更新

`updater.go` 基于 MyGo 内建的签名更新模块（`mygo.Updater`）：前端设置页的
「软件更新」区块调用 `Updater.Check` / `Updater.Install`，宿主从更新源拉取
`update-<target>.json` 清单，下载、ed25519 签名校验后原子替换应用本体并重启
（进度经 `updater.progress` 事件推给前端）。

默认未启用（`mygo.Updater.Enabled()` 为 false）：此时 Check 返回
`supported=false`，前端回退为「前往发布页」按钮。启用步骤：

1. `mygo keygen` 生成密钥对（私钥 `mygo-update.key` **不要提交**）。
2. `mygo.config.ts` 加 `updates: { publicKey, github: "NoelOrin/GOSpeak",
   tagPrefix: "desktop-v" }` —— 与桌面发布链的 tag 约定一致，发布即自带
   更新清单。
3. CI 构建时通过 `MYGO_UPDATER_PRIVATE_KEY` 环境变量（或 config 的
   `updates.privateKey`）提供私钥，`mygo build` 才会签名产物并生成清单。

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

## 性能设计要点

- **静态资源一次读入内存**（`proxyserver.go`）：`webui/dist` 只 ~2 MB，进程启动后在
  后台把所有文本资源读进内存；页面请求只是内存拷贝 + `ServeContent`。
- **构建期预压缩**：`mygo.config.ts` 的 `buildCommand` 最后用 `gzip -9 -k` 为每个文本
  产物生成 `.gz` 边车，宿主直接发送预压缩字节 —— 启动时**不再有任何压缩工作**
  （实测 warm 从 ~50ms 降到 ~7ms）。缺少 `.gz` 时（`go run` / `mygo dev` / 测试）仍会
  运行时压缩兜底。首屏引用的 35 个资源：882 KiB → 318 KiB。
- **缓存策略**：`assets/` `chunks/` `entries/` 是带 hash 的内容寻址产物 → `immutable`
  长缓存；`index.html` → `no-cache`（否则升级后仍指向旧 chunk 名）；其余非 hash 文件
  短缓存。每个响应带 ETag，重新加载走 304 —— 实测二次启动时 WebKit 直接命中磁盘缓存，
  35 个资源请求全部消失。
- **反向代理连接池**：专用 `Transport`（每主机 16 条空闲连接、90s idle、禁用
  `HTTP(S)_PROXY`），避免默认的 2 条空闲连接在 API 突发时反复握手，也避免用户机器上
  的环境代理劫持「回环→自建服务器」的流量。上游不可达时返回统一 JSON 错误（6002）。
- **覆盖层窗口按需创建**（`overlay.go`）：默认关闭时不再预建第二个 WebView
  （否则启动就要多解析/驻留一整份前端 JS）。开启开关、快捷键或策略显示时才创建，
  创建回调会补上期间挂起的"显示"请求。
- **状态接力节流**（`app/web/src/hooks/useOverlayRelay.ts`）：覆盖层关闭时主窗口
  完全不推送；开启后按 100ms 尾部合并推送（原为逐帧），并按 JSON key 去重。
- **配置落盘防抖**：不透明度滑块这类高频变更不再每次同步写 `overlay-config.json`，
  500ms 静默期后写一次，退出时 flush。

### 启动耗时（本机 macOS/arm64，`GOSPEAK_STARTUP_DEBUG=1` 实测，多次运行取中位）

| 阶段 | 耗时 |
| --- | --- |
| 进程启动 → 服务绑定 + 深链注册 | ~13ms |
| mygo/Cocoa 初始化，到 `WhenReady` | ~125ms |
| 回环代理就绪 + 资源缓存预热 | ~8ms（`.gz` 边车）/ ~50ms（运行时压缩兜底） |
| `NewWindow`（WebKit 视图创建） | ~110–170ms |
| WebView 内容进程起来并发出首个请求 | ~215–440ms |
| 首个请求 → React 挂载（页面内计时） | ~225ms |
| **合计：进程启动 → 首屏渲染** | **~0.7–1.0s（中位 ~0.8s）** |

结论：**大头是 mygo/WebKit 的框架与系统开销（约 0.6s），Go 侧自有代码只占 ~250ms，
且几乎都发生在窗口已经显示之后**（窗口在 ~240ms 就画出页面背景色）。继续优化只能动
前端首屏 JS，而不是宿主。

已经用实测否定掉的三个「看起来能省」的方向（别再试了）：

- **隐藏窗口先加载再显示**：首字节仍是 888ms vs 显示中的 899ms —— 那 ~500ms 是 WebKit
  内容进程启动，跟窗口是否可见无关。
- **推迟麦克风权限到首帧之后**：页面内挂载时间 239ms → 238ms，无变化（`getUserMedia`
  是异步的，不阻塞渲染）。
- **推迟 Dock 图标 / 托盘创建**：它们在 ~300ms 完成、首屏在 ~800ms，本来就不在关键路径上。

**留在关键路径上的只有一件事**：启动即创建覆盖层窗口。实测把它改回「启动即建」会让
`overlay subsystem initialised` 从 ~170ms 推迟到 300–580ms（多占 150–300ms 主线程）。
所以覆盖层严格按需创建（见上）。

`GOSPEAK_STARTUP_DEBUG=1` 会打印上述时间线；配合 `GOSPEAK_STARTUP_EXIT_MS=4000` 可以
无人值守跑一次测量：

```bash
cd app/desktop && go build -o /tmp/gospeak . \
  && GOSPEAK_STARTUP_DEBUG=1 GOSPEAK_STARTUP_EXIT_MS=4000 /tmp/gospeak
```

## 接入新能力

1. 在对应的 `*.go` 里给结构体加一个导出方法（普通参数 + 返回值/error）。
2. 在 `main.go` 里 `mygo.Bind` 该结构体（已绑定的类型无需重复）。
3. 在 `packages/desktop/src/capabilities/` 里加/改对应的能力封装，命令名用
   `command("<Struct>", "<Method>")` 构造（与 Go 侧逐字一致）。
4. 跑 `node scripts/check-desktop-commands.mjs` 确认命令名对得上。

> 注意：命令名写错时 MyGo 只会返回 "method X is not bound" 的 rejected Promise，
> 而多数能力封装会静默吞掉错误——所以必须靠上面的校验脚本兜底。
