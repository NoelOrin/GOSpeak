# 桌面端自定义标题栏（各平台统一）

**状态：** 已批准（2026-11-08）

## 背景

`app/desktop` 是 MyGo（Go + 系统 WebView）写的桌面宿主，窗口目前仍是系统默认边框：macOS 是一条灰色标题栏 + 红绿灯，Windows 是系统标题栏 + 系统按钮，Linux 由 GTK 装饰。页面内容从系统标题栏下方开始，与应用内的视觉语言（`--background` / `--border` / 40px 频道栏）割裂。

目标：**页面自己画标题栏，窗口按钮交给系统**，三平台的栏高、排版、配色、交互完全一致，看起来就是应用的一部分。

## 决策

窗口按钮保留系统原生实现（`TitleBarStyle: mygo.TitleBarHidden`），不自绘。

理由：

- macOS 红绿灯、Windows 11 最小化/最大化/关闭（含贴靠布局与悬停反馈）、Linux GTK 按钮的位置与外观由桌面规范决定，自绘会立刻显得"不像这个系统的应用"。
- 保留系统阴影、圆角、Windows 顶部缩放边、双击标题栏缩放（macOS 遵循 `AppleActionOnDoubleClick`）。
- 自绘需要自己维护最大化/还原状态图标、全屏、键盘可达性与三套平台分支，回归面大。

代价：三个平台的按钮**字形**不同。这是系统规范而非风格割裂；栏本身（高度、内边距、背景、分隔线、标题排版）三平台完全一致，且与 Web 构建的 `Header` 同高同排版。

## 方案

### 1. 窗口层（`app/desktop`）

`app/desktop/windows.go` 的主窗口改为：

```go
mygo.NewWindow(mygo.WindowOptions{
    Title:          "GOSpeak",
    URL:            base + "/",
    Page:           mygo.PageOptions{TrustedOrigins: originOf(base)},
    TitleBarStyle:  mygo.TitleBarHidden, // 隐藏标题栏，保留系统窗口按钮
    TitleBarHeight: titleBarHeight,      // 28，与 Web 构建的 Header 同高
    // macOS：三灯垂直居中于 28px 栏内（按钮高 14 → 14 + 2×7 = 28）
    TrafficLightPosition: &mygo.Point{X: 20, Y: 7},
    // 首帧不白闪，取值对齐 theme.css 的 --background
    BackgroundColor: "light-dark(#f8f8f8, #17181b)",
    MinWidth:        960,
    MinHeight:       600,
})
```

- 新增常量 `titleBarHeight = 28`，与 Web 构建 `Header` 的实际高度（`px-2 py-0.5` + `py-1 text-xs` 按钮 = 28px）一致；前端 CSS 变量兜底同为 28px。
- 三平台都保留系统窗口按钮；Windows / Linux 按 `TitleBarHeight` 铺满按钮区，macOS 按 `TrafficLightPosition` 反算出 `--mygo-titlebar-height: 28px`。
- `main.go` 的 dev 分支不再单独建窗，统一走 `openMainWindow()`（dev / release 只差 `appPageBase()`）。

**原生窗口标题固定为 `GOSpeak`，不跟随路由。** 任务栏、Mission Control、Alt+Tab 上只显示 `GOSpeak`：

```go
mainWindow.Page().OnPageTitleUpdated(func(e *mygo.TitleEvent) {
    e.PreventDefault() // 页面 <title> 不再改写原生窗口标题
})
```

页面内的路由标题只出现在应用标题栏里；浏览器构建的标签页标题行为不变。

**原生外观跟随应用主题。** 新增绑定方法：

```go
func (Window) SetAppearance(ctx context.Context, theme string) // "light" | "dark" | "system"
```

实现里 `mygo.Theme.SetSource(...)` 让原生菜单、对话框、macOS 红绿灯跟随，`w.SetWindowButtonsTheme(...)` 让 Windows 标题按钮在深色栏上改用浅色字形。不调用时保持系统外观。

### 2. 页面层（`app/web`）

新增 `app/web/src/components/titleBar/`：

| 文件 | 职责 |
| --- | --- |
| `useRouteTitle.ts` | 从路由 `staticData.title` + 路径段兜底推导标题（从 `layouts/common/header.tsx` 抽出，两处共用） |
| `appTitleBar.tsx` | 标题栏组件 |

结构：

```
┌ padding-left: var(--mygo-titlebar-inset-left) ─────────────────────────────┐
│  [GOSpeak 标记]      标题（绝对居中）      [连接指示] [主题切换]             │
└ padding-right: var(--mygo-titlebar-inset-right) ────────────────────────────┘
   height: var(--mygo-titlebar-height, 28px)   bg-background   border-b border-border
```

- 挂在 **root route**（`app/web/src/pages/__root.tsx`），因此登录 / 注册 / 访客 / 邀请 / 应用内全部覆盖。
- `/overlay`（浮窗）与纯浏览器构建不渲染：`__GOSPEAK_DESKTOP__` 是编译期常量，Web 构建里该分支恒为 `false`，组件返回 `null`，DOM 与基线截图不变。桌面能力 SDK 沿用 `useOverlayRelay` 的既有做法静态引用（Web 包本就包含它）。
- 左右内边距直接吃 `--mygo-titlebar-inset-left/right`：Windows 的按钮区、macOS 红绿灯区、Linux 按桌面按钮布局、全屏时归零，全部自动让位，**不需要任何平台分支**。
- 整条栏 `--app-region: drag` 拖拽窗口；栏内的 `button` / `input` / `a` 由 MyGo 自动排除在拖拽区之外，保持可点；双击缩放由宿主处理。
- 全屏时 `--mygo-titlebar-height` 变为 `0px`，栏随之收起（系统按钮同时隐藏），退出全屏自动恢复。
- 窗口失焦时整条栏文字降为 `text-faint-foreground`（`:root[data-mygo-window-inactive] .app-titlebar`），与原生窗口的非活动态一致。
- 内容：左端 `GOSpeak` 字标；中间当前路由标题；右端连接指示（仅已登录，复用 `ConnectionIndicator`）与浅/深色切换（复用 `themeStore`）。登录页只显示字标 + 标题 + 主题切换。
- 挂载时调用 `createDesktop().window.setAppearance(isDark ? "dark" : "light")`，主题切换时同步调用。

**布局调整（桌面构建）：**

- `layouts/layout.tsx` 的 `<Header />` 改为 `!__GOSPEAK_DESKTOP__ && <Header />`，桌面构建不再出现第二条顶栏；Web 构建行为完全不变。
- 高度链：root route 在桌面构建下包一层 `flex h-full min-h-0 flex-col`（标题栏 + `flex-1 min-h-0` 的 `Outlet`），因此需要把页面里的视口单位高度改为容器高度：
  - `pages/login.tsx`、`pages/register.tsx`：`min-h-screen` → `min-h-full`
  - `pages/guest.tsx`：`h-screen w-screen` → `h-full w-full`
  - `pages/invite/d/$code.tsx`：`min-h-dvh` → `min-h-full`
  - `components/common/errorFallback.tsx`、`pages/overlay.tsx` 的 `min-h-dvh`：保持不变（错误页与浮窗不在标题栏容器内）
  - Web 构建下 `#app` 仍是 100% 高，渲染结果不变。

### 3. 能力 SDK（`packages/desktop`）

`capabilities/window.ts` 的 `WindowControls` 增加：

```ts
setAppearance(theme: "light" | "dark" | "system"): Promise<void>;
```

原生实现 `command("Window", "SetAppearance")`，浏览器兜底为空操作。`scripts/check-desktop-commands.mjs` 会自动双向校验新增命令，无需改脚本。

## 数据与状态

无新增持久化状态。主题仍是 `localStorage`（`theme-is-dark`），标题栏只是把它推给宿主，让原生外观跟随。

## 错误处理

- `SetAppearance` 调用失败（非桌面环境 / 旧宿主）：`catch` 后忽略，页面主题照常生效。
- 宿主未提供 `--mygo-titlebar-*`（例如 Web 构建被宿主打开以外的场景）：CSS 兜底 `28px` / `0px`。
- 窗口按钮区宽度为 0（Linux 桌面不显示按钮、全屏）：标题栏自动占满，无空白。

## 测试

- `app/web/src/components/titleBar/useRouteTitle.test.ts`：路由标题优先、动态段回退、未知路径回退 `GOSpeak`。
- `app/web/src/components/titleBar/appTitleBar.test.tsx`：Web 构建下不渲染任何节点；`GOSPEAK_TARGET=desktop` 下渲染标题、字标与主题按钮。
- `scripts/check-desktop-commands.mjs`：覆盖 `Window.SetAppearance`。
- 手工验证（macOS 本机 + 桌面构建）：
  1. 28px 栏与 Web 构建的顶栏同高，与下方内容无缝衔接，红绿灯垂直居中、不被遮挡；
  2. 拖拽栏可移动窗口，双击可缩放，栏内按钮可点击；
  3. 深色模式下原生菜单/对话框与 Windows 按钮字形跟随；
  4. 全屏后标题栏收起、退出后恢复；
  5. 任务栏 / Mission Control 显示 `GOSpeak`，切换路由不变；
  6. 登录、注册、访客、邀请、应用内页面都有标题栏且无第二条顶栏。
- 回归：`pnpm typecheck`、`biome check`、`GOSPEAK_TARGET=desktop pnpm --filter @gospeak/web build`、`cd app/desktop && go build ./... && go vet ./...`。

## 不在本次范围

- 自绘窗口按钮（`Frameless` + `HideWindowButtons`）。
- Windows / Linux 上的实机视觉验证（本次在 macOS 上验证，其余平台依赖 MyGo 的既有实现与 CI 构建）。
- 覆盖层 `/overlay` 的外观调整（继续无边框、透明、无标题栏）。
- 窗口尺寸记忆（`StateKey`）等既有外壳能力。
