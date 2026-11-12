# 桌面端自定义标题栏 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 桌面宿主改用「隐藏系统标题栏、保留系统窗口按钮」，页面自己画一条 28px 标题栏（与 Web 构建 Header 同高），三平台一致；Web 构建完全不变。

**架构：** Go 侧 `WindowOptions.TitleBarStyle = mygo.TitleBarHidden` + `TitleBarHeight = 28`，页面通过 MyGo 注入的 `--mygo-titlebar-*` CSS 变量给系统按钮让位；React 侧在 root route 挂 `DesktopTitleBar`（编译期 token 裁剪），展示组件 `AppTitleBar` 纯 props、可单测。

**技术栈：** Go 1.26 + MyGo 0.8.3、React 19 + TanStack Router、Tailwind 4、Vitest 4。

---

## 文件结构

| 文件 | 职责 |
| --- | --- |
| 创建 `app/web/src/components/titleBar/routeTitle.ts` | 纯函数：路由匹配 + 路径 → 标题（含 `SEGMENT_TITLES`） |
| 创建 `app/web/src/components/titleBar/routeTitle.test.ts` | 纯函数单测 |
| 创建 `app/web/src/components/titleBar/useRouteTitle.ts` | `useMatches` / `useLocation` → `resolveRouteTitle` |
| 创建 `app/web/src/components/titleBar/appTitleBar.tsx` | 展示组件：字标、居中标题、连接指示、主题切换 |
| 创建 `app/web/src/components/titleBar/appTitleBar.test.tsx` | 展示组件单测 |
| 创建 `app/web/src/components/titleBar/desktopTitleBar.tsx` | 宿主门控（`__GOSPEAK_DESKTOP__`）+ 原生外观同步 |
| 创建 `app/web/src/components/titleBar/desktopTitleBar.test.tsx` | 门控与渲染单测 |
| 修改 `app/web/src/pages/__root.tsx` | 桌面构建下包一层「标题栏 + Outlet」，Web 构建保持原样 |
| 修改 `app/web/src/layouts/layout.tsx` | 桌面构建不再渲染旧 `Header` |
| 修改 `app/web/src/layouts/common/header.tsx` | 标题逻辑改用 `useRouteTitle()` |
| 修改 `app/web/src/pages/login.tsx`、`register.tsx`、`guest.tsx`、`invite/d/$code.tsx` | 视口高度 → 容器高度 |
| 修改 `app/web/src/assets/styles/global.css` | 非活动窗口的标题栏降级色 |
| 修改 `app/desktop/windows.go` | 主窗口 `TitleBarHidden` + 固定标题 |
| 修改 `app/desktop/main.go` | dev / release 统一走 `openMainWindow()` |
| 修改 `app/desktop/window.go` | 新增 `SetAppearance` |
| 修改 `packages/desktop/src/capabilities/window.ts` | `setAppearance` 能力 |
| 修改 `packages/desktop/README.md`、`AGENTS.md` | 能力表与规范同步 |

---

### 任务 1：路由标题纯函数

**文件：**
- 创建：`app/web/src/components/titleBar/routeTitle.ts`
- 测试：`app/web/src/components/titleBar/routeTitle.test.ts`

- [ ] **步骤 1：编写失败的测试**

```ts
import { describe, expect, it } from "vitest";
import { resolveRouteTitle } from "./routeTitle";

const match = (title?: string) => ({ staticData: title ? { title } : undefined });

describe("resolveRouteTitle", () => {
	it("取路由链上最近一级的 staticData.title", () => {
		expect(
			resolveRouteTitle([match("管理"), match("域管理")], "/manage/domains"),
		).toBe("域管理");
	});

	it("staticData 缺失时回退到路径段映射", () => {
		expect(resolveRouteTitle([], "/manage/users")).toBe("用户管理");
	});

	it("动态段不作为标题，回退到父级路径段", () => {
		expect(resolveRouteTitle([], "/manage/domains/3f2a9c1e-0000-4000-8000-000000000000")).toBe(
			"域管理",
		);
	});

	it("完全未知的路径回退到 GOSpeak", () => {
		expect(resolveRouteTitle([], "/unknown-path")).toBe("GOSpeak");
		expect(resolveRouteTitle([], "/")).toBe("GOSpeak");
	});
});
```

- [ ] **步骤 2：运行测试验证失败**

运行：`pnpm --filter @gospeak/web test -- src/components/titleBar/routeTitle.test.ts`
预期：FAIL，`Failed to resolve import "./routeTitle"`。

- [ ] **步骤 3：编写最少实现代码**

```ts
/** 只取 TanStack Router 匹配结果里用得到的部分，staticData 在各页面自行声明。 */
export interface RouteMatchLike {
	staticData?: unknown;
}

// 兜底映射：只在路由未配置 staticData.title、且路径段不是动态参数时使用
export const SEGMENT_TITLES: Record<string, string> = {
	discover: "发现域",
	chat: "聊天",
	link: "分享链接",
	profile: "个人资料",
	guest: "访客访问",
	login: "登录",
	register: "注册",
	manage: "管理",
	domain: "语音域",
	domains: "域管理",
	users: "用户管理",
	roles: "角色权限",
	mute: "禁言",
	ban: "封禁",
	sfu: "SFU 管理",
	storage: "存储",
	email: "邮箱",
	oauth: "OAuth",
	monitor: "服务监控",
};

export function resolveRouteTitle(
	matches: readonly RouteMatchLike[],
	pathname: string,
): string {
	for (let i = matches.length - 1; i >= 0; i--) {
		const data = matches[i].staticData as { title?: string } | undefined;
		if (data?.title) return data.title;
	}
	const segments = pathname.split("/").filter(Boolean);
	const last = segments.at(-1);
	// 动态段（UUID / 数字 ID）不作为标题，回退到父级路径段
	const fallback = last && /^[0-9a-f-]{8,}$/i.test(last) ? segments.at(-2) : last;
	return (fallback ? SEGMENT_TITLES[fallback] : undefined) ?? "GOSpeak";
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`pnpm --filter @gospeak/web test -- src/components/titleBar/routeTitle.test.ts`
预期：PASS（4 个用例）。

- [ ] **步骤 5：Commit**

```bash
git add app/web/src/components/titleBar/routeTitle.ts app/web/src/components/titleBar/routeTitle.test.ts
git commit -m "feat(web): extract route title resolver for the desktop title bar"
```

---

### 任务 2：标题栏展示组件

**文件：**
- 创建：`app/web/src/components/titleBar/appTitleBar.tsx`
- 测试：`app/web/src/components/titleBar/appTitleBar.test.tsx`

- [ ] **步骤 1：编写失败的测试**

```tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";
import { useThemeStore } from "@/stores/themeStore";
import AppTitleBar from "./appTitleBar";

describe("AppTitleBar", () => {
	beforeEach(() => {
		useThemeStore.setState({ isDark: false });
	});

	it("渲染字标与标题", () => {
		render(<AppTitleBar title="域管理" />);
		expect(screen.getByText("域管理")).toBeTruthy();
		expect(screen.getByText("Speak")).toBeTruthy();
	});

	it("整条栏是窗口拖拽区", () => {
		const { container } = render(<AppTitleBar title="聊天" />);
		const bar = container.firstElementChild as HTMLElement;
		expect(bar.className).toContain("[--app-region:drag]");
		expect(bar.className).toContain("app-titlebar");
		expect(bar.style.paddingLeft).toBe("var(--mygo-titlebar-inset-left, 0px)");
		expect(bar.style.paddingRight).toBe("var(--mygo-titlebar-inset-right, 0px)");
	});

	it("主题按钮切换深色模式", async () => {
		render(<AppTitleBar title="聊天" />);
		await userEvent.click(screen.getByRole("button", { name: "切换到深色" }));
		expect(useThemeStore.getState().isDark).toBe(true);
	});

	it("未登录时不渲染连接指示", () => {
		render(<AppTitleBar title="登录" />);
		expect(screen.queryByLabelText("服务器已连接")).toBeNull();
	});
});
```

- [ ] **步骤 2：运行测试验证失败**

运行：`pnpm --filter @gospeak/web test -- src/components/titleBar/appTitleBar.test.tsx`
预期：FAIL，`Failed to resolve import "./appTitleBar"`。

- [ ] **步骤 3：编写最少实现代码**

```tsx
import { Moon, Sun } from "lucide-react";
import ConnectionIndicator from "@/layouts/common/connectionIndicator";
import { toggleDark, useThemeStore } from "@/stores/themeStore";

interface AppTitleBarProps {
	title: string;
	/** 已登录页面才显示连接状态。 */
	showConnection?: boolean;
}

export default function AppTitleBar({
	title,
	showConnection = false,
}: AppTitleBarProps) {
	const isDark = useThemeStore((s) => s.isDark);

	return (
		<div
			className="app-titlebar bg-background text-foreground relative flex shrink-0 select-none items-center gap-2 overflow-hidden px-3 text-xs [--app-region:drag]"
			style={{
				height: "var(--mygo-titlebar-height, 28px)",
				paddingLeft: "var(--mygo-titlebar-inset-left, 0px)",
				paddingRight: "var(--mygo-titlebar-inset-right, 0px)",
				boxShadow: "inset 0 -1px 0 var(--border)",
			}}
		>
			<span className="shrink-0 font-semibold tracking-wide">
				GO<span className="text-primary">Speak</span>
			</span>
			<div className="pointer-events-none absolute inset-0 flex items-center justify-center">
				<span className="font-medium">{title}</span>
			</div>
			<div className="ml-auto flex shrink-0 items-center gap-1">
				{showConnection && <ConnectionIndicator />}
				<button
					type="button"
					className="hover:bg-muted flex cursor-pointer items-center gap-1 rounded-sm px-2 py-1 transition-colors"
					onClick={() => toggleDark()}
					title={isDark ? "切换到浅色" : "切换到深色"}
					aria-label={isDark ? "切换到浅色" : "切换到深色"}
				>
					<span className="text-muted-foreground">
						{isDark ? "深色" : "浅色"}
					</span>
					{isDark ? <Moon className="size-4" /> : <Sun className="size-4" />}
				</button>
			</div>
		</div>
	);
}
```

标题用绝对定位居中，与两侧内容宽度无关；系统按钮区由 `padding-left/right` 让出，标题永远落在可见区中央。

- [ ] **步骤 4：运行测试验证通过**

运行：`pnpm --filter @gospeak/web test -- src/components/titleBar/appTitleBar.test.tsx`
预期：PASS（4 个用例）。

- [ ] **步骤 5：Commit**

```bash
git add app/web/src/components/titleBar/appTitleBar.tsx app/web/src/components/titleBar/appTitleBar.test.tsx
git commit -m "feat(web): add presentational app title bar"
```

---

### 任务 3：标题栏宿主与根路由挂载

**文件：**
- 创建：`app/web/src/components/titleBar/useRouteTitle.ts`
- 创建：`app/web/src/components/titleBar/desktopTitleBar.tsx`
- 测试：`app/web/src/components/titleBar/desktopTitleBar.test.tsx`
- 修改：`app/web/src/pages/__root.tsx`

- [ ] **步骤 1：编写失败的测试**

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", () => ({
	useMatches: () => [{ staticData: { title: "域管理" } }],
	useLocation: () => ({ pathname: "/manage/domains" }),
}));

vi.mock("@gospeak/desktop", () => ({
	createDesktop: () => ({
		window: { setAppearance: () => Promise.resolve() },
	}),
}));

import DesktopTitleBar from "./desktopTitleBar";

describe("DesktopTitleBar", () => {
	it("桌面构建渲染标题栏；Web 构建不渲染任何节点", () => {
		const { container } = render(<DesktopTitleBar />);
		if (__GOSPEAK_DESKTOP__) {
			expect(screen.getByText("域管理")).toBeTruthy();
		} else {
			expect(container.firstElementChild).toBeNull();
		}
	});
});
```

- [ ] **步骤 2：运行测试验证失败**

运行：`pnpm --filter @gospeak/web test -- src/components/titleBar/desktopTitleBar.test.tsx`
预期：FAIL，`Failed to resolve import "./desktopTitleBar"`。

- [ ] **步骤 3：编写最少实现代码**

`app/web/src/components/titleBar/useRouteTitle.ts`：

```ts
import { useLocation, useMatches } from "@tanstack/react-router";
import { resolveRouteTitle } from "./routeTitle";

export function useRouteTitle(): string {
	const matches = useMatches();
	const location = useLocation();
	return resolveRouteTitle(matches, location.pathname);
}
```

`app/web/src/components/titleBar/desktopTitleBar.tsx`：

```tsx
import { createDesktop } from "@gospeak/desktop";
import { useEffect } from "react";
import { useThemeStore } from "@/stores/themeStore";
import { useUserStore } from "@/stores/userStore";
import AppTitleBar from "./appTitleBar";
import { useRouteTitle } from "./useRouteTitle";

/**
 * 桌面构建的窗口标题栏。Web 构建中 `__GOSPEAK_DESKTOP__` 恒为 false，
 * 整个组件与 MyGo 变量依赖一并被摇树剔除。
 */
export default function DesktopTitleBar() {
	if (!__GOSPEAK_DESKTOP__) return null;
	return <DesktopTitleBarLive />;
}

function DesktopTitleBarLive() {
	const title = useRouteTitle();
	const isDark = useThemeStore((s) => s.isDark);
	const loggedIn = useUserStore((s) => !!s.user);

	useEffect(() => {
		void createDesktop()
			.window.setAppearance(isDark ? "dark" : "light")
			.catch(() => {});
	}, [isDark]);

	return <AppTitleBar title={title} showConnection={loggedIn} />;
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`pnpm --filter @gospeak/web test -- src/components/titleBar/desktopTitleBar.test.tsx`
预期：PASS。再跑桌面目标：`GOSPEAK_TARGET=desktop pnpm --filter @gospeak/web test -- src/components/titleBar/desktopTitleBar.test.tsx`，预期同样 PASS（走 `getByText("域管理")` 分支）。

- [ ] **步骤 5：挂到根路由**

修改 `app/web/src/pages/__root.tsx`：

```tsx
import { Outlet, createRootRouteWithContext } from "@tanstack/react-router";
import type { QueryClient } from "@tanstack/react-query";
import { RouteErrorComponent } from "@/components/common/errorBoundary";
import DesktopTitleBar from "@/components/titleBar/desktopTitleBar";

export interface RouterContext {
	queryClient: QueryClient;
}

function PlainRoot() {
	return <Outlet />;
}

function DesktopRoot() {
	return (
		<div className="flex h-full min-h-0 flex-col">
			<DesktopTitleBar />
			<div className="min-h-0 flex-1">
				<Outlet />
			</div>
		</div>
	);
}

export const Route = createRootRouteWithContext<RouterContext>()({
	// 桌面构建才加标题栏容器：Web 构建的 DOM 与基线截图保持一字不差。
	component: __GOSPEAK_DESKTOP__ ? DesktopRoot : PlainRoot,
	errorComponent: RouteErrorComponent,
});
```

- [ ] **步骤 6：Commit**

```bash
git add app/web/src/components/titleBar/useRouteTitle.ts app/web/src/components/titleBar/desktopTitleBar.tsx app/web/src/components/titleBar/desktopTitleBar.test.tsx app/web/src/pages/__root.tsx
git commit -m "feat(web): mount desktop title bar at the root route"
```

---

### 任务 4：去掉桌面构建的第二条顶栏并改高度链

**文件：**
- 修改：`app/web/src/layouts/layout.tsx:491-497`
- 修改：`app/web/src/layouts/common/header.tsx`
- 修改：`app/web/src/pages/login.tsx:157`
- 修改：`app/web/src/pages/register.tsx:172`
- 修改：`app/web/src/pages/guest.tsx:114`
- 修改：`app/web/src/pages/invite/d/$code.tsx:101`
- 修改：`app/web/src/assets/styles/global.css`

- [ ] **步骤 1：Layout 桌面构建不渲染旧 Header**

```tsx
// 桌面构建的顶栏由根路由的 DesktopTitleBar 提供，这里只服务 Web 构建；
// 移动端舞台自带标题栏，列表页用精简顶栏。
{!__GOSPEAK_DESKTOP__ && !isMobile && <Header />}
{!__GOSPEAK_DESKTOP__ && isMobile && !(isDomain && mobileStage) && <Header />}
```

- [ ] **步骤 2：Header 改用共享标题逻辑**

把 `SEGMENT_TITLES`、`useMatches`、`useLocation` 与推导代码替换为：

```tsx
import { useRouteTitle } from "@/components/titleBar/useRouteTitle";

const title = useRouteTitle();
```

删除 `header.tsx` 里已迁走的 `SEGMENT_TITLES`、`useLocation`/`useMatches` 导入与推导段，保留 `initTheme` 与主题按钮。

- [ ] **步骤 3：高度链改容器高度**

```diff
- <div className="relative mx-auto flex min-h-screen w-full max-w-7xl">
+ <div className="relative mx-auto flex min-h-full w-full max-w-7xl">
```
（`login.tsx`、`register.tsx` 各一处）

```diff
- <div className="bg-muted flex h-screen w-screen items-center justify-center">
+ <div className="bg-muted flex h-full w-full items-center justify-center">
```
（`guest.tsx`）

```diff
- <div className="bg-muted flex min-h-dvh w-full items-center justify-center px-4">
+ <div className="bg-muted flex min-h-full w-full items-center justify-center px-4">
```
（`invite/d/$code.tsx`）

- [ ] **步骤 4：非活动窗口降级色**

在 `app/web/src/assets/styles/global.css` 末尾追加：

```css
/* 桌面窗口失焦时标题栏跟随原生非活动态（MyGo 在 <html> 上打标记） */
:root[data-mygo-window-inactive] .app-titlebar {
	color: var(--faint-foreground);
}
```

- [ ] **步骤 5：验证**

运行：`pnpm --filter @gospeak/web typecheck && pnpm --filter @gospeak/web test -- src/components/titleBar`
预期：typecheck 通过，titleBar 用例全 PASS。

- [ ] **步骤 6：Commit**

```bash
git add app/web/src/layouts app/web/src/pages/login.tsx app/web/src/pages/register.tsx app/web/src/pages/guest.tsx "app/web/src/pages/invite/d/\$code.tsx" app/web/src/assets/styles/global.css
git commit -m "feat(web): fit auth and invite pages into the desktop title bar shell"
```

---

### 任务 5：桌面宿主隐藏系统标题栏

**文件：**
- 修改：`app/desktop/windows.go`
- 修改：`app/desktop/main.go:58-74`

- [ ] **步骤 1：主窗口改用隐藏标题栏**

`app/desktop/windows.go` 中新增常量并改写 `openMainWindow`：

```go
// titleBarHeight 是页面自绘标题栏的高度（与 appTitleBar.tsx 的兜底值一致）：
// Windows / Linux 的系统按钮按它铺满，macOS 由 TrafficLightPosition 反算出同样的高度。
const titleBarHeight = 28

func openMainWindow() {
	base := appPageBase()
	mainWindow = mygo.NewWindow(mygo.WindowOptions{
		// 任务栏 / Mission Control / Alt+Tab 只显示应用名，不跟随路由标题。
		Title: "GOSpeak",
		URL:   base + "/",
		// 隐藏系统标题栏、保留系统窗口按钮：页面画栏，按钮仍是原生的。
		TitleBarStyle:  mygo.TitleBarHidden,
		TitleBarHeight: titleBarHeight,
		// macOS：三灯在 28px 栏内垂直居中（按钮高 14 + 2×7）。
		TrafficLightPosition: &mygo.Point{X: 20, Y: 7},
		// 首帧底色对齐 theme.css 的 --background，避免加载时白闪。
		BackgroundColor: "light-dark(#f8f8f8, #17181b)",
		// 与旧默认窗口一致的下限，避免出现不可用的窄窗。
		MinWidth:  960,
		MinHeight: 600,
		// Lets the loopback page call bound Go methods (server config, overlay,
		// capabilities). Verify the field name against the mygo version in use.
		Page: mygo.PageOptions{TrustedOrigins: originOf(base)},
	})
	// 页面 <title> 不再改写原生窗口标题。
	if p := mainWindow.Page(); p != nil {
		p.OnPageTitleUpdated(func(e *mygo.TitleEvent) { e.PreventDefault() })
	}
}
```

- [ ] **步骤 2：dev / release 统一建窗**

`app/desktop/main.go` 的 `WhenReady` 改为：

```go
mygo.App.WhenReady(func() {
	// Load the persisted backend base URL before any window opens.
	loadServerConfig()

	if !isDevMode() {
		// Release: boot the loopback proxy and load the frontend from it,
		// so API/WS stay same-origin no matter which server is configured.
		// Dev loads the Vite dev server directly (HMR); its built-in proxy
		// keeps /api and /ws same-origin. No loopback proxy needed.
		if _, err := ensureProxyStarted(); err != nil {
			log.Printf("[GOSpeak] loopback proxy failed to start: %v", err)
		}
	}
	openMainWindow()
	...
```

- [ ] **步骤 3：验证编译**

运行：`cd app/desktop && go build ./... && go vet ./...`
预期：无输出、退出码 0。

- [ ] **步骤 4：Commit**

```bash
git add app/desktop/windows.go app/desktop/main.go
git commit -m "feat(desktop): hide the native title bar and keep system window buttons"
```

---

### 任务 6：原生外观跟随应用主题

**文件：**
- 修改：`app/desktop/window.go`
- 修改：`packages/desktop/src/capabilities/window.ts`
- 修改：`packages/desktop/README.md`

- [ ] **步骤 1：Go 侧新增 SetAppearance**

```go
// SetAppearance 让原生外观跟随应用主题：Windows 的标题按钮改用与页面一致的
// 明暗字形，macOS / Linux 的按钮与原生菜单、对话框跟随应用外观。
func (Window) SetAppearance(ctx context.Context, theme string) {
	w := mygo.CallerWindow(ctx)
	if w == nil {
		return
	}
	switch theme {
	case "light":
		w.SetWindowButtonsTheme(mygo.ThemeLight)
		mygo.Theme.SetSource(mygo.ThemeLight)
	case "dark":
		w.SetWindowButtonsTheme(mygo.ThemeDark)
		mygo.Theme.SetSource(mygo.ThemeDark)
	default:
		w.SetWindowButtonsTheme(mygo.ThemeSystem)
		mygo.Theme.SetSource(mygo.ThemeSystem)
	}
}
```

- [ ] **步骤 2：SDK 侧新增能力**

`packages/desktop/src/capabilities/window.ts`：

```ts
export type Appearance = "light" | "dark" | "system";

export interface WindowControls {
	minimize(): Promise<void>;
	maximize(): Promise<void>;
	unmaximize(): Promise<void>;
	isMaximized(): Promise<boolean>;
	setAlwaysOnTop(value: boolean): Promise<void>;
	setTitle(title: string): Promise<void>;
	/** 让原生窗口按钮与菜单跟随应用主题（桌面端有效，浏览器为空操作）。 */
	setAppearance(theme: Appearance): Promise<void>;
	close(): Promise<void>;
}

const webFallback: WindowControls = {
	// ...
	async setAppearance() {},
};

const native: WindowControls = {
	// ...
	setAppearance: (theme) =>
		bridge.invoke(command("Window", "SetAppearance"), theme),
};
```

- [ ] **步骤 3：校验命令契约**

运行：`node scripts/check-desktop-commands.mjs`
预期：`desktop bridge commands are in sync`，退出码 0（`Window.SetAppearance` 双向匹配）。

- [ ] **步骤 4：README 能力表补一行**

`packages/desktop/README.md` 能力清单表新增：

```md
| 窗口外观 | `createWindowControls().setAppearance()` | `Window.SetAppearance` | 无操作 |
```

- [ ] **步骤 5：Commit**

```bash
git add app/desktop/window.go packages/desktop/src/capabilities/window.ts packages/desktop/README.md
git commit -m "feat(desktop): sync native window appearance with the app theme"
```

---

### 任务 7：文档、全量验证与测试日志

**文件：**
- 修改：`AGENTS.md`（桌面能力边界一节）
- 创建：`agent_test_logs/desktop-custom-titlebar-<YYYY-MM-DD-HHmm>.md`

- [ ] **步骤 1：AGENTS.md 补规范**

在「Web / 桌面能力边界」小节追加：

```md
- 桌面主窗口使用 `TitleBarStyle: mygo.TitleBarHidden`：系统窗口按钮保留，标题栏由
  `app/web/src/components/titleBar/` 自绘。页面只能通过 `--mygo-titlebar-*` 变量给按钮
  让位、用 `--app-region: drag` 声明拖拽区，禁止按平台写分支。
- 原生窗口标题固定为 `GOSpeak`（`OnPageTitleUpdated` 阻止页面改写），路由标题只出现在应用内标题栏。
```

- [ ] **步骤 2：全量验证**

```bash
cd app/desktop && go build ./... && go vet ./... && cd ../..
node scripts/check-desktop-commands.mjs
pnpm typecheck
npx @biomejs/biome check app/web packages/desktop
GOSPEAK_TARGET=desktop pnpm --filter @gospeak/web build
pnpm --filter @gospeak/web test -- src/components/titleBar
GOSPEAK_TARGET=desktop pnpm --filter @gospeak/web test -- src/components/titleBar
pnpm --filter @gospeak/desktop test
```

预期：全部通过。Web 构建下 `DesktopTitleBar` 渲染 `null`，页面 DOM 与改动前一致（用 `visual-baseline` 的登录页截图对照）。

- [ ] **步骤 3：本机桌面目视验证**

```bash
pnpm --filter @gospeak/desktop-app build   # 或 GOSPEAK_TARGET=desktop 构建后 mygo dev
```

检查清单：28px 栏与 Web 顶栏同高、与内容无缝；红绿灯垂直居中不被遮挡；拖拽移动、双击缩放、按钮可点；深色模式原生外观跟随；全屏收起/恢复；任务栏与 Mission Control 只显示 `GOSpeak`；登录/注册/访客/邀请/应用内均有标题栏且无第二条顶栏。

- [ ] **步骤 4：写测试日志**

按 `AGENTS.md` §Test Logging 模板写 `agent_test_logs/desktop-custom-titlebar-<时间>.md`，记录命令、输出摘要与目视结论。

- [ ] **步骤 5：Commit**

```bash
git add AGENTS.md agent_test_logs
git commit -m "docs(desktop): document the custom title bar contract"
```
