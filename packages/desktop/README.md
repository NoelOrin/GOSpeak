# @gospeak/desktop

GOSpeak 桌面端的**能力 SDK**——一端对接 TypeScript，一端对接原生宿主。

## 它是什么

GOSpeak 桌面端由两部分组成：

| 路径 | 角色 | 语言 |
| --- | --- | --- |
| `packages/desktop` | 桌面平台特殊能力的 **TS SDK**（本包） | TypeScript |
| `app/desktop` | 桌面端**本体**：Go + MyGo 宿主，内嵌 Web 前端（mygo 从 Go 生成 TS 客户端） | Go |

本包解决的问题：**让被内嵌的 React 前端（`@gospeak/web`）能够调用原生桌面能力**——
窗口控制、系统托盘、原生通知、开机自启、深链、文件对话框、自动更新等——
而同一套代码在纯浏览器里也能跑（自动降级到 Web 等价实现）。

## 桥接模型

MyGo 在加载每个页面时，向页面注入运行时 `window.mygo`（类型来自 `mygo-runtime`）：

```ts
window.mygo = {
  call<T>(method: string, ...args: unknown[]): Promise<T>; // 调用绑定的 Go 方法
  on(event: string, listener: (...args: unknown[]) => void): () => void; // 订阅宿主事件
  isMyGo(): boolean;       // 是否在 MyGo 窗口里（而非 dev server 的浏览器标签页）
  runtime(): { platform; windowId; version };
};
```

`app/desktop` 通过 `mygo.Bind(Window{}, Tray{}, …)` 绑定的 Go 服务，会被 `mygo generate`
生成一套类型化 TS 客户端（如 `Overlay.show()`、`Window.setAlwaysOnTop(v)`）。本包**直接基于
`window.mygo` 的 `call` / `on`** 封装能力，而不是自己造一个桥：

- 方法名契约：`<Struct>.<Method>`，与 Go 侧**逐字一致**（首字母都大写），即 Go 的 `Overlay.PushState`
  → `"Overlay.PushState"`、`ServerConfig.GetBaseURL` → `"ServerConfig.GetBaseURL"`。用 `bridge.ts` 的
  `command("Overlay", "PushState")` 构造，不要手写字符串；`scripts/check-desktop-commands.mjs` 会在 CI
  双向校验 Go 绑定与 SDK 调用，命令名写错会直接失败而不是运行时静默 reject。
- 参数契约：**位置参数**（MyGo 按 JSON 解码进 Go 方法签名）。
- 事件名契约：`mygo.Broadcast(event, ...args)`（宿主侧）/ `mygo.on(event, listener)`（前端侧）。
- `isDesktop()` 等价于 `window.mygo?.isMyGo()`；纯浏览器里 `window.mygo` 不存在，`isMyGo()` 为 false。

```ts
// packages/desktop/src/capabilities/overlay.ts（节选）
const native: Overlay = {
  // 位置参数对齐 Go 的 `func (Overlay) SetEnabled(v bool)`
  setEnabled: (v) => bridge.invoke(command("Overlay", "SetEnabled"), v),
  // 订阅宿主经 mygo.Broadcast 发出的事件
  onState: (handler) => bridge.listen("overlay:state", (s) => handler(s as OverlaySnapshot)),
};
```

> 因为本包直接用 `window.mygo`，**不需要**在 React 渲染前注入自定义桥；只要页面跑在 MyGo 窗口里，
> `isDesktop()` 即为真。若后续使用 `mygo generate` 的产物（类型化客户端），它底层同样走 `mygo-runtime`，
> 与本包并存无冲突。

### 平台隔离

- `isDesktop()` 由 `window.mygo.isMyGo()` 决定——纯 Web 构建（dev server 标签页）里它永远是 false。
- 因此 `@gospeak/web` 的「桌面端」设置 tab 只在桌面端注册；覆盖层路由 `/overlay` 在浏览器里打开时
  只显示提示文案，不渲染 HUD、不调用任何原生 API。
- 覆盖层是一个**独立的 WebView 窗口**，与主窗口各自拥有独立的 JS 上下文与 store 实例。为避免
  覆盖层重新加入语音房（`DUPLICATE_IDENTITY`/双 join），覆盖层**不连接 socket / SFU**：
  主窗口把房间成员与发言状态接力（`overlay.pushState` → 宿主 `Broadcast` → 覆盖层 `onState`）
  给覆盖层，覆盖层只做展示。这条接力链只在桌面端生效，浏览器里两端都是 no-op。
- 所有能力方法在浏览器里都是安全的 no-op / Web 等价实现，不会抛错或崩溃。

## 用法

```ts
import { createDesktop, isDesktop, showNotification } from "@gospeak/desktop";

const desktop = createDesktop();

if (desktop.isDesktop) {
  await desktop.window.setAlwaysOnTop(true);
  desktop.tray.setMenu([
    { id: "open", label: "打开 GOSpeak" },
    { separator: true },
    { id: "quit", label: "退出" },
  ]);
  desktop.tray.onMenuClick((id) => console.log("tray click", id));
}

showNotification({ title: "有人邀请你加入语音房", body: "点击加入" }).onClick(() => {
  // 仅在桌面端会触发
});
```

## 能力清单

| 能力 | 工厂 | 原生命令前缀 | 浏览器兜底 |
| --- | --- | --- | --- |
| 窗口控制 | `createWindowControls()` | `Window.*` | `document.title` / `window.close` |
| 窗口外观 | `createWindowControls().setAppearance()` | `Window.SetAppearance` | 无操作 |
| 系统托盘 | `createSystemTray()` | `Tray.*` | 无操作 |
| 原生通知 | `showNotification()` | `Notification.*` | Web Notifications API |
| 开机自启 | `createAutoStart()` | `AutoStart.*` | 无操作 |
| 深链 | `onDeepLink()` | `DeepLink.*` | 无操作 |
| 文件对话框 | `openFileDialog()` 等 | `Dialog.*` | `<input type=file>` / `confirm` |
| 自动更新 | `createUpdater()` | `Updater.*` | 无操作 |

## 条件编译（构建期裁剪）

`src/conditional.ts` 提供一套构建期条件编译工具：桌面独享模块在 Web 构建中
**不进包、不产出 chunk**，同时保留运行时检测兼容（dev server 被宿主打开的场景）。

### 机制

Vite 通过 `define` 注入两个编译期常量（由 `GOSPEAK_TARGET` 环境变量驱动，默认 `web`）：

| Token | 类型 | 值 |
| --- | --- | --- |
| `__GOSPEAK_TARGET__` | `"web" \| "desktop"` | 构建目标 |
| `__GOSPEAK_DESKTOP__` | `boolean` 字面量 | 是否桌面构建 |

未注入 token 的环境（tsc / vitest / Node 直跑源码）按 `web` 兜底，因此本包在任何
上下文中都安全。

### 工具函数

| 导出 | 用途 |
| --- | --- |
| `BUILD_TARGET` / `DESKTOP_BUILD` / `WEB_BUILD` | 编译期常量（非打包环境同样可用） |
| `inDesktop()` | 综合判断：桌面构建恒 true，否则回退运行时 `isDesktop()`。UI 条件渲染用这个 |
| `assertDesktop(feature)` | 开发期守卫：Web 构建调用桌面独享能力时抛可读错误 |
| `ifDesktop(d, w)` / `desktopOnly(d, fallback?)` / `webOnly(w, fallback?)` | 分支取值；死分支**不执行**（运行时安全） |
| `importDesktop(loader)` / `importWeb(loader)` | 动态导入守卫；非目标构建不执行 loader，返回 `null` |

### 两种模式怎么选

- **模块级裁剪**（希望桌面独享代码不进 Web 包）：在调用处直接写 token 的 `if`，
  死分支会被打包器常量折叠后整体剔除：

  ```ts
  // 静态模块：desktop.tsx 在 Web 构建中被摇树剔除
  export const TABS = __GOSPEAK_DESKTOP__ ? [...baseTabs, desktopTab] : baseTabs;

  // 动态导入：Web 构建中 import() 位于死分支，chunk 不会产出
  const mod = __GOSPEAK_DESKTOP__ ? await import("./overlay-page") : null;
  ```

- **运行时兼容**（Web 构建仍可能被宿主打开，如 dev）：用 `inDesktop()`，

  ```ts
  if (inDesktop()) {
    const base = await createServerConfig().getBaseUrl();
  }
  ```

  能力 SDK 内部（`capabilities/*` 的浏览器降级）继续使用运行时 `isDesktop()`，不受影响。

### 构建 / 开发命令

```bash
pnpm --filter @gospeak/web build            # Web 构建（桌面独享模块剔除）
pnpm --filter @gospeak/web build:desktop    # 桌面构建（GOSPEAK_TARGET=desktop）
pnpm --filter @gospeak/web dev:desktop      # 桌面目标 dev server
GOSPEAK_TARGET=desktop pnpm --filter @gospeak/desktop test   # 桌面目标跑测试
```

新增能力时：在 `src/capabilities/` 下加一个模块（接口 + native + webFallback），
在 `src/index.ts` 导出，并同步在 `app/desktop` 的对应 Go 服务里实现对应命令（先 `mygo generate` 生成 TS 客户端）。
