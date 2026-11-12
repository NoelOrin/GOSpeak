# 游戏内覆盖层：原生渲染重做

**状态：** 待评审（2026-11-12）

**分支：** `feature/game-overlay-native-ui`

## 背景

覆盖层现在是一条完整链路：Go 宿主建第二个 WebView（`app/desktop/overlay.go`，631 行）→ 加载前端 `/overlay` 路由（`app/web/src/pages/overlay.tsx`）→ 主窗口通过 `useOverlayRelay` 把房间状态推给宿主 → 宿主 `Broadcast` 给覆盖层窗口渲染。

链路是通的，但**它不是原生覆盖层**。对照 mygo v0.8.3 的 API 逐条核实后，有五处硬伤：

1. **盖不住全屏游戏。** 窗口只用了 `AlwaysOnTop`，没设 `Panel: true`。mygo 的 panel 才是"浮在所有 App 之上、跟着切 Space、能盖别人全屏 App"的窗口类型（macOS `NSPanel` + `CanJoinAllSpaces|FullScreenAuxiliary`，Windows `WS_EX_TOOLWINDOW|WS_EX_TOPMOST`）。当前实现在 macOS 上根本盖不住全屏游戏。
2. **`isFullscreenAppInFront()` 在 macOS 恒返回 `false`。** 为保持 cgo-free，CoreGraphics 实现被删掉了（`overlay_fullscreen_other.go`），于是 `auto` 策略在 mac 上永远不会自动显示。
3. **鼠标穿透是假的。** `SetIgnoreMouseEvents` 全仓库零调用。页面用 `pointer-events-none` 只让 DOM 不响应，**窗口本身仍吃掉那块区域的点击**——锁定状态下点 HUD 位置，游戏收不到。
4. **没有防录屏。** `SetContentProtection` 零调用，直播/录屏会把 HUD 一起录进去。
5. **显示时可能抢焦点。** 用的是 `Show()`，Windows 上它会 `SetForegroundWindow`。

另外，为一个只显示成员名的 HUD 常驻第二份前端 JS 运行时（多解析一整份 React 应用），代价与收益不匹配。

目标：**用 mygo 原生 UI（`github.com/egoist/mygo/ui`）在 GPU 上直接绘制 HUD，去掉第二个 WebView**，同时把窗口行为改成真正的原生覆盖层。

## 决策

| 决策 | 选择 | 理由 |
|---|---|---|
| 渲染方式 | mygo `ui` 原生绘制（`WindowOptions.Content`） | 零 JS 运行时、可做逐帧动效、省一份前端解析 |
| 目标平台 | macOS + Windows（Linux 按 mygo 能力自动降级） | |
| 全屏检测 | **删除** | `Panel: true` 本身就解决了"盖全屏"，不需要知道前台是不是全屏 |
| 默认显示策略 | `auto` = 离开 GOSpeak 就显示 | 事件驱动、三平台原生、语义比"前台有全屏应用"更准 |
| 头像 | 原生异步加载 | 用户要求 |
| 防录屏 | 默认**关闭**，可选开关 | 用户要求：开了自己截不到图 |

### 为什么用「离开 GOSpeak」替代「前台有全屏应用」

`mygo.App.IsActive()` 三平台都是原生实现，无需 cgo：

| 平台 | 实现 |
|---|---|
| macOS | `NSApp.isActive` |
| Windows | `GetForegroundWindow()` 是否属于本进程窗口 |
| Linux | 是否有本进程窗口持有键盘焦点 |

配合 `OnDidBecomeActive` / `OnDidResignActive` 事件，策略是**事件驱动**的，不需要轮询。

语义从"你在全屏玩游戏"放宽到"你不在 GOSpeak 里"——你在刷网页、看视频、写代码时想瞄一眼谁在说话，HUD 同样该在。全屏检测只在你恰好全屏时才显示，反而更窄。

> **物理限制（需在文档中向用户说明）**：**独占全屏（DirectX exclusive fullscreen）任何窗口都盖不住**，这是 OS 层面限制，Discord 同样盖不住。无边框全屏（当前绝大多数游戏）没有问题。

## 方案

### 1. 窗口层（`app/desktop/overlay.go`）

```go
mygo.NewWindow(mygo.WindowOptions{
    Title:      "GOSpeak Overlay",
    Panel:      true,   // 关键：浮在所有 App 之上、跟 Space、盖别人全屏、不进 Alt+Tab/Mission Control
    Hidden:     true,
    Frameless:  true,   // 无标题栏 → 没有系统关闭按钮
    DisableClose: true, // 声明不可关闭（但 Escape 仍需 OnClose 拦截，见下）
    Transparent: true,
    DisableShadow: true,
    DisableResize: true,
    AlwaysOnTop: true,  // Panel 已隐含，显式保留以对齐语义
    SkipTaskbar: true,  // Panel 已隐含
    Width:  200,
    Height: 44,         // 由内容计算，见下
    Content: ui.View(hud.view),
    StateKey: "overlay", // 记住用户拖到的位置
})
```

与现状的差异：

| 项目 | 现在 | 改成 |
|---|---|---|
| 窗口类型 | 普通窗口 + `AlwaysOnTop` | `Panel: true` |
| 内容 | `URL: base + "/overlay"` | `Content: ui.View(...)` |
| 显示 | `w.Show()` | `w.ShowInactive()` |
| 锁定 | 页面 `pointer-events-none`（假） | `SetIgnoreMouseEvents(true/false)` |
| 拖拽 | 页面 `--app-region: drag` | 元素 `.DragWindow()` |
| 防录屏 | 无 | `SetContentProtection(bool)`，默认 false |

**覆盖层没有"关闭"这个动作。** 它不是普通窗口，用户不该能把它关掉——关闭意味着"覆盖层没了"，但语义上只有"关掉覆盖层功能"（设置页的总开关）和"隐藏"（快捷键）。所以：

- `Frameless: true` + `Panel: true` → 没有标题栏、没有红绿灯、没有系统关闭按钮，本来就没有关闭入口。
- `DisableClose: true` → 声明式地表达"这个窗口不可关闭"。
- **但 `DisableClose` 拦不住 Escape。** 已核实 mygo 的 `windowHandler.ShouldClose()`（`window.go:1981`）**完全不检查 `Closable`**——它只 fire `OnClose` 然后看有没有被 `PreventDefault`。而 AppKit 的 `NSPanel` 会把 Escape 转成关闭请求（mygo 的 `cancelOperation:` 实现为空，让命令沿 responder chain 传到 window）。所以必须显式拦截：

```go
w.OnClose(func(e *mygo.CloseEvent) {
    e.PreventDefault()
    w.Hide()   // 覆盖层从不销毁，只隐藏
})
```

这不是防御性代码，是**唯一**能阻止 Escape 关掉覆盖层的手段。

**`ShowInactive` 是必须的，不是偏好。** 三个理由，任意一条都足以否决 `Show()`：

1. **Windows**：`Show()` 会 `SetForegroundWindow` 抢前台。而 `auto` 策略依赖"前台是否属于本进程"——用 `Show()` 会让覆盖层自己把 App 变成前台，触发自己隐藏，形成闪烁循环。
2. **macOS**：mygo 的 Panel 用 `Show()` 会 `makeKeyAndOrderFront`，**抢走游戏的键盘焦点**（mygo 自己的 e2e 测试 `TestPanel` 明确断言了 "the panel focused"）。`ShowInactive` 走 `orderFrontRegardless`，不成为 key window。
3. **Linux**：`ShowInactive` 走 `gtkWidgetShow` 而不激活。

顺带的好处：`ShowInactive` 下 Panel 不是 key window，收不到 Escape，正常使用中根本触发不到关闭路径；只有**解锁拖拽后点击 HUD**才会成为 key window。但拦截仍然必须有，因为那条路径真实存在。

**删除的文件：**

- `app/desktop/overlay_fullscreen_windows.go`（88 行 Win32 全屏检测）
- `app/desktop/overlay_fullscreen_other.go`（10 行恒 false 桩）
- `startOverlayPolicyLoop()` 及每秒 ticker

### 2. 显示策略（三模式）

`OverlayConfig.Policy` 字段保留，取值扩为三种：

| 值 | 行为 | 求值方式 |
|---|---|---|
| `auto`（默认） | `snap.Connected && !mygo.App.IsActive()` → 显示 | 事件驱动：`OnDidBecomeActive` / `OnDidResignActive` + 每次 `PushState` |
| `always` | `snap.Connected` → 显示 | 每次 `PushState` |
| `manual` | 只由快捷键 / 托盘控制 | 不自动求值 |

热键覆盖语义保留：`auto`/`always` 下快捷键的最后一次操作优先生效，直到底层条件翻转（沿用现有 `manualOverride` + `lastAutoWant` 机制，仅把 `want` 的计算换掉）。

`evaluateOverlayPolicy()` 不再被 ticker 调用，改为由以下三处触发：

1. `OnDidBecomeActive` / `OnDidResignActive`（策略从 `auto` 切走时注销）
2. `Overlay.PushState`（连接状态可能变化）
3. `SetEnabled` / `SetPolicy`

`App.IsActive()` 必须在主线程调用（mygo 内部 `onMainValue`），因此事件回调里直接调用是安全的。

### 3. HUD 视觉（`app/desktop/overlay_view.go`）

```
┌────────────────────┐
│ ◯  🎤 阿伟         │  说话中：头像 emerald 环 + 名字加粗纯白
│ ◯  🔇 小林  (你)   │  静音：麦克风划掉
│ ◯  🎤 Kate         │  未说话：white/70
└────────────────────┘
```

- **无卡片背景**，白字 + `TextShadow(0, 1, 4, RGBA(0,0,0,0.9))`，保证在任何游戏画面上可读（沿用现有思路，mygo `Element.TextShadow` 已确认支持）
- **麦克风图标用内嵌 SVG**：`ui.MustParseSVG(...)` 常量 + `ui.Icon(c, mic)`。`ui.Icon` 跟随祖先的 `TextColor`（已核实），所以静音/未静音只改 `TextColor`，不需要两套图标。SVG 路径取自 lucide-react 的 `mic` / `mic-off`（与 Web 端图标同源，视觉一致），24×24 viewBox、`stroke="currentColor"`。
- 头像 16px 圆形：有图显示图，无图/加载中/失败显示**首字 + 渐变圆**（与 Web 端 `components/common/avatar.tsx` 的 fallback 一致）
- 行高 22，内边距 4，窗口宽 200
- **高度随人数变化**：`height = padding*2 + min(n, 12)*22`，成员变化时调 `win.SetContentSize(200, h)`。人少时窗口就小，不挡游戏
- 超过 12 人：显示前 11 行 + 一行 `+N`（穿透窗口里滚动没有意义，不实现滚动）
- 排序：说话人排前，其余保持原序（沿用现有逻辑）
- 成员为空时窗口隐藏（而不是画一个空窗口）

> **为什么窗口尺寸要手动维护**：已核实 `fit` 仅在 `opts.Content == nil && opts.FitContent` 时挂载，且 `connectFit` 只对 flyout 调用。原生 UI 窗口不会自动撑高，必须自己 `SetContentSize`。

**说话状态动效**：用 `Element.AnimateColor` / `Animate` 做颜色与环宽的过渡（mygo 已提供），避免生硬跳变。动效本身不触发额外 `Invalidate`，由 mygo 的动画帧驱动。

### 4. 头像异步加载（`app/desktop/overlay_avatar.go`）

```go
type avatarCache struct {
    mu     sync.Mutex
    images map[string]*ui.Bitmap  // url -> 解码结果（nil 表示加载失败，避免重试风暴）
}
```

- **URL 归一化**：
  - 相对路径（`/uploads/...`）→ `appPageBase() + path`。release 走回环代理、dev 走 vite，`/uploads` 都是同源转发（已核实 `proxyserver.go` 的 `mux.HandleFunc("/uploads/", s.handleProxied)`）
  - 绝对 `http(s)://`（S3 / R2）→ 直接用
  - 空串 → 不请求，直接首字 fallback
- 独立 `http.Client{Timeout: 5s}`，不复用代理的 Transport（避免头像拖慢 API 连接池）
- `ui.DecodeImage(data)` 同时支持位图与 SVG（已核实）
- goroutine 拉取，完成后 `win.Invalidate()` 重绘
- 缓存上限 64 条，超出整体重置（覆盖层成员规模远小于此，不需要 LRU）
- 加载失败缓存为 nil，不重试

### 5. 设置页（`app/web/src/components/modal/settingModal/tab_item/desktop.tsx`）

- 显示策略改为三选：`自动（离开 GOSpeak 时显示）` / `始终显示` / `手动`
- 新增「防录屏」开关（默认关），描述说明"开启后直播/录屏/截图看不到覆盖层。仅 macOS / Windows 生效"
- 不透明度、两个热键字段保持不变

### 6. 前端清理

删除覆盖层作为**独立页面**的一切：

| 文件 | 处理 |
|---|---|
| `app/web/src/pages/overlay.tsx` | 删 |
| `app/web/src/stores/overlayStore.ts` | 删 |
| `app/web/src/routeTree.gen.ts` | 重新生成（去掉 `/overlay`） |
| `app/web/src/main.tsx` | 删掉 `pathname !== "/overlay"` 的麦克风权限判断 |
| `app/web/src/hooks/useOverlayRelay.ts` | 删掉 `pathname === "/overlay"` 自推判断；其余保留 |
| `app/web/src/layouts/layout.tsx` | 注释更新（不再有"覆盖层路由不走此 Layout"） |

`useOverlayRelay` 本身**保留**——主窗口仍需把状态推给宿主，只是宿主不再转发给第二个窗口，而是直接驱动原生 HUD 重绘。

### 7. SDK 类型（`packages/desktop/src/capabilities/overlay.ts`）

- `OverlayPolicy` 加 `"always"`
- `pushState` 保留（主窗口 → 宿主，驱动原生 HUD 重绘）
- `getState` 保留（诊断 / 设置页显示当前快照）
- 删 `onState`（没有窗口订阅了）
- 新增 `setContentProtection(bool)`；**不提供 `getContentProtection()`**——已核实 mygo 只有 `SetContentProtection`，没有读取接口（`platform.Window` 接口里也只有 setter）。前端要显示开关状态，从 `getConfig()` 里带回（配置存在 Go 侧，不依赖窗口查询）。
- `OverlayConfig` 加 `contentProtection bool` 字段（JSON tag `contentProtection`），与 `enabled` / `opacity` 等同级持久化。
- `OverlaySnapshot` / `OverlayMember` 的 JSON tag 保持与 Go 侧一致（`avatar` 现在真的会被用到了）

### 8. 测试

| 文件 | 覆盖 |
|---|---|
| `app/desktop/overlay_view_test.go` | `ui.NewTester` 无头渲染：0 人 / 3 人 / 说话人高亮 / 静音图标 / 超 12 人的 `+N` / 窗口高度计算 |
| `app/desktop/overlay_avatar_test.go` | `httptest` 提供 PNG：解码成功、缓存命中不重复请求、失败回退 nil、相对 URL 归一化 |
| `app/desktop/overlay_test.go` | 策略求值表驱动：三模式 × (connected, appActive) 的期望显隐 |
| `app/desktop/proxyserver_test.go` | 把 `/overlay` 深链断言换成 `/domain/test` 等真实 SPA 路由 |

**CI 注意**：桌面端 CI job 只跑 `go vet`（`go test` 需要真实前端产物）。新增的测试不依赖 `webui/dist` 内容，可以在本地跑；是否把 `go test` 加进 CI 另议（现有注释说明了原因）。

### 9. 文档

- `app/desktop/README.md`：更新覆盖层一节（原生渲染、Panel、策略三模式、删除全屏检测的原因）
- 新增「独占全屏盖不住」的已知限制说明

## 文件变更总览

| 操作 | 文件 |
|---|---|
| 删 | `app/desktop/overlay_fullscreen_windows.go` |
| 删 | `app/desktop/overlay_fullscreen_other.go` |
| 删 | `app/web/src/pages/overlay.tsx` |
| 删 | `app/web/src/stores/overlayStore.ts` |
| 新 | `app/desktop/overlay_view.go` |
| 新 | `app/desktop/overlay_avatar.go` |
| 新 | `app/desktop/overlay_view_test.go` |
| 新 | `app/desktop/overlay_avatar_test.go` |
| 新 | `app/desktop/overlay_test.go` |
| 改 | `app/desktop/overlay.go` |
| 改 | `app/desktop/lifecycle.go`（托盘菜单加锁定/解锁项） |
| 改 | `app/desktop/proxyserver_test.go` |
| 改 | `app/desktop/README.md` |
| 改 | `packages/desktop/src/capabilities/overlay.ts` |
| 改 | `packages/desktop/src/index.ts`（若需导出新类型） |
| 改 | `app/web/src/components/modal/settingModal/tab_item/desktop.tsx` |
| 改 | `app/web/src/hooks/useOverlayRelay.ts` |
| 改 | `app/web/src/main.tsx` |
| 改 | `app/web/src/layouts/layout.tsx` |
| 重生成 | `app/web/src/routeTree.gen.ts` |

## 验证

**自动化：**

```bash
cd app/desktop && go vet ./... && go test ./...
pnpm typecheck && pnpm lint
pnpm test:web
```

**手动（macOS + Windows 各一遍）：**

1. `pnpm dev:web` + `GOSPEAK_DESKTOP_DEV=1 pnpm --filter @gospeak/desktop-app dev`
2. 设置页开启覆盖层，策略选 `自动`
3. 进入语音房 → 切到别的 App → HUD 出现
4. 切回 GOSpeak → HUD 自动消失
5. 开着 HUD 进一个**无边框全屏**游戏/视频 → HUD 仍可见（macOS 验证 Space 切换）
6. 锁定状态下点 HUD 位置 → 游戏收到点击（穿透生效）
7. 快捷键解锁 → 拖动 HUD → 位置重启后保留
8. 成员说话 → 高亮变化；静音 → 图标变化
9. 3 人 vs 15 人 → 窗口高度正确，15 人显示 `+4`
10. 开启防录屏 → 系统截图/录屏中 HUD 消失
11. 关掉覆盖层 → 任务管理器/活动监视器确认没有第二个 WebView 进程

## 风险

| 风险 | 缓解 |
|---|---|
| 原生 UI 在 CJK 下的字形与排版与 Web 端有差异 | 用 `ui.NewTester` 的 `Texts()` 断言文本；手动目视对比 |
| `Panel` 在 Linux Wayland 无 layer-shell 时降级为普通窗口 | 文档说明；Linux 不在主要目标内 |
| 头像 HTTP 请求在弱网下堆积 | 5s 超时 + 失败缓存为 nil 不重试 |
| 删除 `/overlay` 路由影响外部深链 | 已核实只有 `proxyserver_test.go` 引用；深链只注册 `gospeak://invite/d/<CODE>` |
| `SetIgnoreMouseEvents` 在锁定态下，用户可能忘记如何解锁 | 托盘菜单加「锁定/解锁覆盖层位置」项（`app/desktop/lifecycle.go` 的 `trayMenu()`），不只依赖热键 |
| `SetContentProtection` 在 Linux 是空实现 | 设置页描述里标注"仅 macOS / Windows 生效"；Linux 上开关保留但无效，不报错 |
