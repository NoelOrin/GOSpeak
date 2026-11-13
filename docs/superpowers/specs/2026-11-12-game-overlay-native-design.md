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
| 渲染方式 | mygo `ui` 原生绘制（`WindowOptions.Content`） | 零 JS 运行时、省一份前端解析 |
| 目标平台 | macOS + Windows（Linux 按 mygo 能力自动降级） | |
| 全屏检测 | **删除** | `Panel: true` 本身就解决了"盖全屏"，不需要知道前台是不是全屏 |
| 默认显示策略 | `auto` = 离开 GOSpeak 就显示 | 事件驱动、三平台原生、语义比"前台有全屏应用"更准 |
| 策略数量 | `auto` / `always` / `manual` 三选 | 用户要求「策略作为可选择的」；`always` 对双屏用户有用（游戏在另一块屏） |
| HUD 配色 | **跟随系统主题**，描影反向翻转 | 用户要求；浅色模式下深色字配白光晕，避免压在深色游戏上看不见 |
| HUD 动效 | **不做** | HUD 是余光看的，动效是干扰 |
| 房间名 | **不显示** | 已在房里，是噪音，且多一行多挡画面 |
| 头像 | 原生异步加载 | 用户要求 |
| 防录屏 | 默认**关闭**，可选开关 | 用户要求：开了自己截不到图 |
| 窗口关闭 | **不可关闭**（无标题栏、`DisableClose`、Escape 拦截） | 覆盖层不是普通窗口，只有「关掉功能」和「隐藏」，没有「关掉窗口」 |

### 为什么用「离开 GOSpeak」替代「前台有全屏应用」

`mygo.App.IsActive()` 三平台都是原生实现，无需 cgo：

| 平台 | 实现 |
|---|---|
| macOS | `NSApp.isActive` |
| Windows | `GetForegroundWindow()` 是否属于本进程窗口 |
| Linux | 是否有本进程窗口持有键盘焦点 |

配合 `OnDidBecomeActive` / `OnDidResignActive` 事件，策略是**事件驱动**的，不需要轮询。

语义从"你在全屏玩游戏"放宽到"你不在 GOSpeak 里"——你在刷网页、看视频、写代码时想瞄一眼谁在说话，HUD 同样该在。全屏检测只在你恰好全屏时才显示，反而更窄。

> **全屏覆盖的真实边界（必须在文档中向用户说明）**
>
> Windows 上「全屏」分三种，能不能盖住取决于 DWM 是否在合成：
>
> | 模式 | DWM | Panel 能盖住 |
> |---|---|---|
> | 无边框窗口 | 合成 | 能 |
> | FSO（全屏优化） | **合成** | **能** |
> | 真 FSE（legacy flip） | 合成被挂起 | 不能 |
>
> 按微软 [Demystifying Fullscreen Optimizations](https://devblogs.microsoft.com/directx/demystifying-full-screen-optimizations/)：FSO 下「游戏以为自己独占，实际跑在无边框窗口」，且「有 overlay 出现时 DWM 重新接管显示」。**FSO 从 Windows 10 起默认开启**，所以玩家在游戏里选「全屏」的绝大多数场景，Panel 是盖得住的。
>
> 盖不住的是：用户手动在 exe 属性里勾了「禁用全屏优化」、老游戏、或特意关闭 FSO 的竞技游戏。此时连 Windows 音量条和 Game Bar 都盖不上去（[PresentMon #212](https://github.com/GameTechDev/PresentMon/issues/212)）。
>
> **Discord / Steam 能盖住真 FSE，是因为它们注入 DLL 并 hook `IDXGISwapChain::Present`**，不是靠窗口置顶。本项目不做注入：技术栈冲突（纯 Go 无 cgo vs C++/MinHook），且有反作弊封号风险（Vanguard 尤其严格），对游戏语音产品而言性价比为负。
>
> **应对：不检测、不折腾，只引导。** 设置页加一行「若覆盖层未显示，请把游戏显示模式改为无边框窗口」。macOS 无此问题——全屏 App 各占一个 Space，`Panel` 的 `CanJoinAllSpaces | FullScreenAuxiliary` 就是为此设计的。

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
│ ◉  🎤 阿伟         │  说话中：头像 Success 环 + 名字加粗
│ ◯  🎤 小林  (你)   │  自己：名字后加 (你)
│ ◯  🔇 Kate         │  静音：麦克风划掉
└────────────────────┘
```

**跟随系统主题。** `c.Theme()` 由 mygo 提供，它监听系统的 `effectiveAppearance`，变化时自动重建帧（`ui/runtime.go` 的 `themeChanged`），**不需要自己写任何检测代码**。视图里只读 `c.Theme()` 决定配色。

| 元素 | 深色模式 | 浅色模式 |
|---|---|---|
| 名字（说话中） | `#f4f4f5` 加粗 | `#18181b` 加粗 |
| 名字（其他） | `#f4f4f5` 70% | `#18181b` 70% |
| **文字描影** | `(0, 1, 4, rgba(0,0,0,0.9))` 黑晕 | `(0, 1, 4, rgba(255,255,255,0.9))` 白晕 |
| 说话高亮 / 头像环 | `Theme.Success`（`#22c55e`） | `Theme.Success`（`#16a34a`） |
| 静音图标 | 白 45% | 黑 45% |
| 头像无图 fallback | `Theme.Accent` → `Theme.Success` 渐变 | 同左 |
| 窗口背景 | 完全透明 | 完全透明 |

> **为什么描影要反向翻转**：HUD 没有卡片背景，直接浮在游戏画面上。浅色模式下 `Theme.Text` 是近黑色（`#18181b`），压在深色游戏上会看不清。深色字配白色光晕、浅色字配黑色光晕——这是字幕的标准做法，既跟随了主题，又不拿可读性去赌游戏画面的明暗。

其余规格：

- **无卡片背景**（不做半透明底，不做毛玻璃）
- **麦克风图标用内嵌 SVG**：`ui.MustParseSVG(...)` 常量 + `ui.Icon(c, mic)`。`ui.Icon` 跟随祖先的 `TextColor`（已核实），所以静音/未静音只改 `TextColor`，不需要两套图标。SVG 路径取自 lucide-react 的 `mic` / `mic-off`（与 Web 端图标同源），24×24 viewBox、`stroke="currentColor"`。
- 头像 16px 圆形：有图显示图，无图/加载中/失败显示**首字 + 渐变圆**（与 Web 端 `components/common/avatar.tsx` 的 fallback 一致）
- 行高 22，内边距 4，窗口宽 200（**基准值，实际按缩放系数放大，见下**）
- **高度随人数变化**：`height = 8 + min(n, 12) × 22`（基准值），成员变化时调 `win.SetContentSize`
- 超过 12 人：显示前 11 行 + 一行 `+N`（穿透窗口里滚动没有意义，不实现滚动）
- 排序：说话人排前，其余保持原序（沿用现有逻辑）
- 成员为空时窗口隐藏（而不是画一个空窗口）

**明确不做**：说话状态动效（HUD 是余光看的，动效是干扰）、房间名（已在房里，是噪音）、卡片背景。

### 3a. 屏幕自适应缩放

HUD 的尺寸是 DIPs，在高分屏上不会模糊（mygo 原生 UI 按 `backingScaleFactor` 以物理像素渲染）。但**逻辑分辨率不同**：1080p 屏的逻辑宽度是 1920，4K 屏在 100% 缩放下是 3840——同样 12px 的字，在 4K 屏上占的视觉比例只有一半，小到看不见。

**用 `Context.SetZoom(z)` 缩放**，不手算每个尺寸：

> 已核实 `ui/zoom.go` 的语义：内容按 `1/z` 布局、放大 `z` 倍渲染，**文字、图标、矢量、图片都按屏幕分辨率重绘，保持矢量清晰**，不是把画面拉大的位图。`SetZoom` 取值 clamp 到 `[0.25, 5]`。

**关键尺寸换算**（已核实 `ui/zoom.go` 的注释）：

> 「`Window.SetContentSize` 和 `Window.ContentSize` 是**窗口的 DIPs**，是内容 DIPs 的 z 倍。」

所以窗口尺寸必须是 `hudHeight(n) * z`，而视图内部仍用基准值布局。这是最容易写错的地方。

**z 的取值**：默认按覆盖层窗口所在显示器的**逻辑分辨率**自动决定，用户可在设置页手动覆盖。

| 逻辑宽度 | 自动 z |
|---|---|
| ≤ 1920 | 1.0 |
| ≤ 2560 | 1.25 |
| ≤ 3440 | 1.35 |
| > 3440 | 1.5 |

显示器逻辑宽度取 `mygo.Screen.DisplayNearestPoint(windowPos).Bounds.Width`。已核实 macOS 的 `NSScreen.frame` 返回的是**逻辑点（DIPs）**而非物理像素，所以 `Bounds.Width` 直接可用，不需要再除以 `ScaleFactor`。

**手动覆盖**：`OverlayConfig` 加 `scale float64` 字段，`0` 表示自动（默认）。设置页给一个「HUD 大小」下拉（自动 / 80% / 100% / 125% / 150%）。用户改动时立即调 `SetZoom` 并重算窗口尺寸。

**显示器变化时重算**：拖动 HUD 到另一块屏，逻辑分辨率变了，`z` 要跟着变。用 `mygo.Screen.OnDisplaysChanged(...)`（已核实存在）重新求值并 `SetZoom` + `SetContentSize`。

> **为什么窗口尺寸要手动维护**：已核实 `fit` 仅在 `opts.Content == nil && opts.FitContent` 时挂载，且 `connectFit` 只对 flyout 调用。原生 UI 窗口不会自动撑高，必须自己 `SetContentSize`。

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
- 加载失败缓存为 nil，**不重试**（同一人反复进出会反复请求坏图；等窗口重建或切服务器时清空再试）
- **不预缩放**：源图可能是 512×512 而显示只要 16×16，但引入缩放代码的收益不抵复杂度（实际同时在线通常 <10 人，约 10MB）。真出内存问题再优化。
- **切换服务器必须清空缓存**：`ServerConfig.SetBaseURL` 里调 `overlayAvatarCache.clear()`。否则旧服务器的 `/uploads/...` 在代理已指向新服务器后会 404 或返回别人的头像——这是改成本地缓存后新引入的耦合点，容易漏。

### 5. 设置页（`app/web/src/components/modal/settingModal/tab_item/desktop.tsx`）

- 显示策略改为三选：`自动（离开 GOSpeak 时显示）` / `始终显示` / `手动`
- 新增「防录屏」开关（默认关），描述说明"开启后直播/录屏/截图看不到覆盖层。仅 macOS / Windows 生效"
- 覆盖层一节底部加一行提示：「若覆盖层未显示，请把游戏的显示模式改为**无边框窗口**（Windows 独占全屏无法被任何置顶窗口覆盖）」
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
- 新增「全屏覆盖的边界」一节：FSO 可覆盖 / 真 FSE 不可覆盖、为什么不做注入、引导用户切无边框

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
| 改 | `app/desktop/lifecycle.go`（托盘菜单加「锁定/解锁覆盖层位置」） |
| 改 | `app/desktop/windows.go`（`reloadAllWindows` 删掉 `w.Page()` 分支——原生窗口 `Page()` 恒 nil） |
| 改 | `app/desktop/server_config.go`（`SetBaseURL` 调 `overlayAvatarCache.clear()`） |
| 改 | `app/desktop/proxyserver_test.go` |
| 改 | `app/desktop/README.md` |
| 改 | `packages/desktop/src/capabilities/overlay.ts` |
| 改 | `packages/desktop/src/index.ts`（若需导出新类型） |
| 改 | `app/web/src/components/modal/settingModal/tab_item/desktop.tsx` |
| 改 | `app/web/src/hooks/useOverlayRelay.ts` |
| 改 | `app/web/src/main.tsx` |
| 改 | `app/web/src/layouts/layout.tsx` |
| 重生成 | `app/web/src/routeTree.gen.ts` |

## 实施顺序

**关键约束：步骤 1 和 2 必须连续完成，中间不可发布。**

窗口一旦改成 `Panel: true` + `ShowInactive` + `SetIgnoreMouseEvents`，旧 WebView HUD 就会变成「能看见但完全不能操作」的状态（不接键盘、锁定时不收鼠标）。feature 分支上无所谓，但不能在两步之间切出去。

分五步，每步都能独立验证：

| 步骤 | 内容 | 完成后可验证 |
|---|---|---|
| **1. 窗口层** | `Panel: true`、`ShowInactive`、`OnClose` 拦截、`SetIgnoreMouseEvents`、删全屏检测与 ticker；**暂时保留 `URL: /overlay`** | 窗口行为全部生效，HUD 仍是旧 WebView 版：盖全屏、穿透、不抢焦点、Escape 不关闭 |
| **2. 原生视图** | `overlay_view.go` + 把 `URL` 换成 `Content: ui.View(...)`；删 `pages/overlay.tsx`、`overlayStore.ts` | HUD 变成原生绘制，主题跟随生效 |
| **3. 策略** | 三模式 + `App.IsActive` 事件接线 | 切 App 自动显隐 |
| **4. 头像** | `overlay_avatar.go` + 缓存 + 切服务器清缓存 | 头像显示，切服务器刷新 |
| **5. 收尾** | 设置页、托盘解锁项、SDK 类型、文档、测试 | 完整 |

**为什么第 1 步不直接换 `Content`**：把「窗口行为」和「渲染方式」的变更分开，出问题时能立刻判断是窗口配置错了还是视图画错了。窗口层有 5 个平台相关的坑（Panel、ShowInactive、OnClose、穿透、ContentProtection），视图层是纯逻辑——混在一起会让定位成本翻倍。

## 验证

**自动化：**

```bash
cd app/desktop && go vet ./... && go test ./...
pnpm typecheck && pnpm lint
pnpm test:web
```

视图层可**完全无头测试**：`ui.NewTester(view, 200, h)` + 断言 `Texts()` / `Find()` / 窗口高度。头像用 `httptest`。策略求值是纯函数，表驱动。**窗口行为无法自动化**，只能手动验。

**手动（macOS + Windows 各一遍）：**

1. `pnpm dev:web` + `GOSPEAK_DESKTOP_DEV=1 pnpm --filter @gospeak/desktop-app dev`
2. 设置页开启覆盖层，策略选 `自动`
3. 进入语音房 → 切到别的 App → HUD 出现
4. 切回 GOSpeak → HUD 自动消失
5. 开着 HUD 进一个**无边框全屏**游戏/视频 → HUD 仍可见（macOS 验证 Space 切换）
6. 锁定状态下点 HUD 位置 → 游戏收到点击（穿透生效）
7. 快捷键解锁 → 拖动 HUD → 位置重启后保留
8. **按 Escape → HUD 不消失**（验证 `OnClose` 拦截）
9. 成员说话 → 高亮变化；静音 → 图标变化
10. 3 人 vs 15 人 → 窗口高度正确，15 人显示 `+4`
11. 系统切深色/浅色 → HUD 配色与描影跟着翻转
12. 切换服务器 → 头像刷新，不残留旧服务器图片
13. 开启防录屏 → 系统截图/录屏中 HUD 消失
14. 关掉覆盖层 → 任务管理器/活动监视器确认没有第二个 WebView 进程

## 风险

| 风险 | 缓解 |
|---|---|
| 原生 UI 在 CJK 下的字形与排版与 Web 端有差异 | 用 `ui.NewTester` 的 `Texts()` 断言文本；手动目视对比 |
| `Panel` 在 Linux Wayland 无 layer-shell 时降级为普通窗口 | 文档说明；Linux 不在主要目标内 |
| 头像 HTTP 请求在弱网下堆积 | 5s 超时 + 失败缓存为 nil 不重试 |
| 删除 `/overlay` 路由影响外部深链 | 已核实只有 `proxyserver_test.go` 引用；深链只注册 `gospeak://invite/d/<CODE>` |
| `SetIgnoreMouseEvents` 在锁定态下，用户可能忘记如何解锁 | 托盘菜单加「锁定/解锁覆盖层位置」项（`app/desktop/lifecycle.go` 的 `trayMenu()`），不只依赖热键 |
| `SetContentProtection` 在 Linux 是空实现 | 设置页描述里标注"仅 macOS / Windows 生效"；Linux 上开关保留但无效，不报错 |
