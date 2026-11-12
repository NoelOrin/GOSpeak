# 桌面端外壳：应用图标、窗口尺寸记忆、托盘行为与登录态持久化

**状态：** 已批准（2026-11-08）

## 背景

`app/desktop` 是 MyGo（Go + 系统 WebView）写的桌面宿主，内嵌 `@gospeak/web` 构建产物，通过本地回环反向代理（`app/desktop/proxyserver.go`）让页面、REST、WebSocket 全部同源。

当前存在四类问题：

1. 打包产物没有应用图标，Dock / 任务栏 / 安装器显示 MyGo 默认图。
2. 窗口位置与尺寸不记忆，每次启动都回到默认大小与居中位置。
3. 没有托盘行为：关窗即退出进程，无法常驻后台。
4. 「服务器地址记不住、反复登录」——由三个独立缺陷叠加造成。

## 根因分析

### 根因 A：桌面桥接命令名与 MyGo 注册键不一致（最主要）

MyGo 用 `mygo.Bind(Window{})` 注册服务后，命令键是 `"<StructName>.<MethodName>"`，即 `"Window.Minimize"`、`"ServerConfig.GetBaseURL"`、`"Overlay.PushState"`（见 `mygo generate` 的产物 `app/desktop/mygo.ts`）。

而 `packages/desktop/src/capabilities/*.ts` 里调用的是 `"window.minimize"`、`"server.getBaseURL"`、`"overlay.pushState"`。MyGo 找不到方法时返回 `method X is not bound` 的 rejected Promise，SDK 侧大多 `.catch(() => {})` 静默吞掉。

后果：服务器地址读写、窗口控制、托盘菜单、覆盖层、通知全部静默失效。用户在登录页填写服务器地址后 `setBaseUrl` 失败，宿主既没有持久化也没有切换代理目标，下次启动仍是空地址 —— 表现为「服务器地址记不住」。

### 根因 B：回环端口每次随机，localStorage 随 origin 丢失

`startProxyServer()` 用 `net.Listen("tcp", "127.0.0.1:0")`，端口每次启动都变。WebView 的 `localStorage` 按 origin（含端口）隔离，于是每次启动 `user` 缓存、`gospeak_session_expires_at`、主题偏好全部落到新的空 storage —— 表现为「反复登录」。

### 根因 C：会话判定过严 + 换服务器不清凭据

- `userStore.ensureSession` 在任意异常下都 `clearAuth()`，网络抖动或服务端未启动也会清掉本地会话。
- Cookie 存在回环 origin 上，切换后端服务器时没有清理，会串号。

## 方案

### 1. 修复桥接契约

- 在 `packages/desktop/src/bridge.ts` 增加 `command(service, method)` 构造函数：`command("Window", "Minimize") === "Window.Minimize"`。
- 所有 `capabilities/*.ts` 改用该函数，命令名与 Go 结构体/方法名逐字一致。
- 新增仓库级校验脚本 `scripts/check-desktop-commands.mjs`：
  - 解析 `app/desktop/*.go`，收集 `main.go` 中 `mygo.Bind(...)` 绑定类型的导出方法；
  - 解析 `packages/desktop/src/capabilities/*.ts`，收集 `command("X", "Y")`；
  - 双向比对，任何一侧多出或缺失即失败。
- 修正 `packages/desktop/README.md` 与 `AGENTS.md` 里错误的命令名示例（`<struct 小写>.<Method 首字母小写>` → `<Struct>.<Method>`）。

### 2. 稳定的回环 origin

- `startProxyServer` 改为优先绑定上次成功使用的端口，失败则按固定区间顺延（默认 47000–47099），全被占用才回落到系统随机端口。
- 端口区间选在 47000 段而非更高：macOS / Windows 的临时端口段从 49152 起，再往上会和系统随机分配的端口频繁冲突；47000 既比低段端口高一大截，又仍在临时端口段以下。
- 选定端口写入用户数据目录 `proxy-port.json`，下次启动优先复用。
- 端口是 localhost 本地服务，不做鉴权强化；但代理只监听 `127.0.0.1`。

### 3. 登录态持久化加固

- `ensureSession` 只在**确定的鉴权失败**（错误码 1001/1002/1003/1014/1015）时清会话；网络错误/超时/5xx 时保留缓存会话并放行（有缓存用户则返回 `true`）。
- 切换服务器地址时：宿主清空回环 origin 上的全部 Cookie，前端清空 `user` 与过期时间，然后重载页面。地址未变化时不清理。

### 4. 窗口尺寸记忆

- 主窗口使用 MyGo 原生 `WindowOptions.StateKey = "main"`：位置、尺寸、最大化、全屏写入 `window-state.json`，重启恢复；显示器不存在时 MyGo 自动回落到可见区域居中。
- 首次启动给合理默认尺寸（1200×800）与最小尺寸（960×640），避免出现不可用的窄窗。

### 5. 应用图标

- 新增 `app/desktop/resources/icon.png`（1024×1024，品牌彩色图）→ 打包后的 `.app`、Dock、Windows 可执行文件、Linux desktop entry 全部使用它。
- 新增 `app/desktop/assets/icon.png`（同源，go:embed）→ 未打包运行（`go run` / `mygo dev`）时通过 `App.Dock.SetIcon` 与 `Window.SetIcon` 兜底，保证 Dock / 任务栏图标正确。
- 新增 `app/desktop/assets/tray.png`（单色 + alpha 的模板图）→ 托盘图标，`IconIsTemplate: true`，macOS 菜单栏与 Windows 深色任务栏自动反色。

### 6. 托盘行为

- **关闭按钮 = 隐藏到托盘**，进程常驻；**最小化按钮 = 普通最小化**。
- 主窗口 `OnClose`：非退出流程中 `PreventDefault()` 并 `Hide()`。
- 注册 `App.OnWindowAllClosed` 空监听，阻止「最后一个窗口关闭即退出」。
- 托盘菜单：`打开 GOSpeak` / `隐藏到托盘` / `退出`。`退出` 设置退出标记后调用 `App.Quit()`，此时 `OnClose` 不再拦截。
- Dock 图标点击（`App.OnActivate`）与第二次启动（`RequestSingleInstanceLock` + `OnSecondInstance`）都聚焦并显示主窗口。

## 数据与状态

| 位置 | 内容 |
| --- | --- |
| 用户数据目录 `server-config.json` | 后端 base URL |
| 用户数据目录 `proxy-port.json` | 上次回环端口 |
| 用户数据目录 `window-state.json` | 主窗口与覆盖层的位置/尺寸/状态 |
| 用户数据目录 `overlay-config.json` | 覆盖层配置 |
| WebView 默认 session | 后端下发的 HttpOnly 会话 Cookie |
| `localStorage`（回环 origin） | `user`、`gospeak_session_expires_at` |

## 错误处理

- 端口绑定失败：依次尝试候选端口；全部失败则报错并让宿主以「未配置代理」状态启动（窗口显示 503 提示页）。
- 图标资源缺失：记录日志，托盘/窗口图标保持系统默认，不阻塞启动。
- 桥接命令缺失：由校验脚本在 CI 阶段拦截，而不是运行时静默失败。

## 测试

- `scripts/check-desktop-commands.mjs` 单元测试（Node 内置 test runner 或 vitest，与 `scripts/check-version.test.mjs` 同风格）。
- `app/web/src/stores/userStore.test.ts` 新增：网络错误保留会话、鉴权错误清除会话。
- Go：`app/desktop/proxyserver_test.go` 覆盖端口选择与 `proxy-port.json` 往返。
- 手工验证：打包运行后确认 Dock/任务栏图标、托盘图标深浅色、关窗进托盘、重启后窗口位置尺寸恢复、切换服务器后不再带旧会话。

## 不在本次范围

- 自动更新（`Updater` 仍为 stub）。
- Linux/Windows 的开机自启实现（当前仅 macOS）。
- 覆盖层窗口的尺寸记忆调整（已使用 `StateKey: "overlay"`，行为不变）。
