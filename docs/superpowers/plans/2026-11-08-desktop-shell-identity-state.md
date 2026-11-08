# 桌面端外壳：图标、窗口记忆、托盘与登录态持久化 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 让 GOSpeak 桌面端有正确的应用/托盘图标、记住窗口大小与位置、关窗进托盘常驻，并修好服务器地址与登录态的持久化。

**架构：** 分两条线。Go 宿主（`app/desktop`）负责图标资源、稳定回环端口、窗口 `StateKey`、托盘与关窗拦截；TS SDK（`packages/desktop`）负责修正桥接命令名契约，Web 端（`app/web`）负责会话判定与换服务器时的凭据清理。桥接命令名由仓库脚本在 CI 中双向校验，避免再次静默失效。

**技术栈：** Go 1.27 + MyGo 0.8.3、React 19 + TypeScript、Vitest 4、Node `node:test`、Biome。

---

## 文件结构

| 文件 | 职责 |
| --- | --- |
| `app/desktop/resources/icon.png` | 新建。1024×1024 品牌彩色图标，MyGo 打包时生成 .icns / .ico / .desktop 图标 |
| `app/desktop/assets/icon.png` | 新建。同一张图，`go:embed` 供未打包运行时设置 Dock / 窗口图标 |
| `app/desktop/assets/tray.png` | 新建。32×32 单色 + alpha 模板图，托盘用 |
| `app/desktop/assets.go` | 新建。`go:embed` 资源与 `applyAppIcon()` / `trayIcon()` |
| `app/desktop/proxyport.go` | 新建。端口候选、持久化、选择逻辑（纯函数，便于单测） |
| `app/desktop/proxyport_test.go` | 新建。端口选择与 `proxy-port.json` 往返测试 |
| `app/desktop/proxyserver.go` | 修改。`startProxyServer` 改用 `chooseProxyPort()` |
| `app/desktop/windows.go` | 修改。主窗口默认尺寸、最小尺寸、`StateKey` |
| `app/desktop/tray.go` | 修改。模板图标、托盘菜单、`showMainWindow()` |
| `app/desktop/lifecycle.go` | 新建。关窗进托盘、`OnWindowAllClosed`、Dock 激活、单实例聚焦、退出标记 |
| `app/desktop/main.go` | 修改。绑定新服务、启动顺序（单实例锁、图标、托盘、生命周期） |
| `app/desktop/server_config.go` | 修改。换服务器时清 Cookie 并通知前端清会话 |
| `packages/desktop/src/bridge.ts` | 修改。新增 `command(service, method)` |
| `packages/desktop/src/capabilities/*.ts` | 修改。全部改用 `command(...)` |
| `scripts/check-desktop-commands.mjs` | 新建。双向校验 Go 绑定方法 ↔ TS 命令名 |
| `scripts/check-desktop-commands.test.mjs` | 新建。校验脚本自身的测试 |
| `app/web/src/stores/userStore.ts` | 修改。区分网络错误与鉴权错误 |
| `app/web/src/stores/userStore.test.ts` | 修改。补网络错误保留会话的用例 |
| `app/web/src/api/authTransport.ts` | 修改。新增 `isAuthFailure()` 判定 |
| `packages/desktop/README.md` | 修改。修正命令名契约文档 |
| `AGENTS.md` | 修改。修正命令名契约描述 |

---

## 任务 1：桥接命令名契约与校验脚本

**文件：**
- 创建：`scripts/check-desktop-commands.mjs`、`scripts/check-desktop-commands.test.mjs`
- 修改：`packages/desktop/src/bridge.ts`
- 测试：`scripts/check-desktop-commands.test.mjs`

- [ ] **步骤 1：编写失败的测试**

创建 `scripts/check-desktop-commands.test.mjs`：

```js
import assert from "node:assert/strict";
import test from "node:test";

import { checkCommands, extractGoBindings, extractTsCommands } from "./check-desktop-commands.mjs";

const goSource = `
package main

func main() {
	mygo.Bind(Window{})
	mygo.Bind(ServerConfig{})
}
`;

const windowGo = `
package main

func (Window) Minimize(ctx context.Context) {}
func (Window) IsMaximized(ctx context.Context) bool { return false }
`;

const serverGo = `
package main

func (ServerConfig) GetBaseURL() string { return "" }
func (ServerConfig) SetBaseURL(newURL string) {}
`;

const tsSource = `
const native = {
  minimize: () => bridge.invoke(command("Window", "Minimize")),
  getBaseUrl: () => bridge.invoke(command("ServerConfig", "GetBaseURL")),
};
`;

test("extracts bound Go services and their exported methods", () => {
	const bindings = extractGoBindings([
		{ path: "main.go", source: goSource },
		{ path: "window.go", source: windowGo },
		{ path: "server_config.go", source: serverGo },
	]);

	assert.deepEqual([...bindings].sort(), [
		"ServerConfig.GetBaseURL",
		"ServerConfig.SetBaseURL",
		"Window.IsMaximized",
		"Window.Minimize",
	]);
});

test("extracts command names from the TypeScript SDK", () => {
	const commands = extractTsCommands([{ path: "capabilities/window.ts", source: tsSource }]);

	assert.deepEqual([...commands].sort(), [
		"ServerConfig.GetBaseURL",
		"Window.Minimize",
	]);
});

test("reports commands the host does not bind", () => {
	const result = checkCommands({
		bindings: new Set(["Window.Minimize"]),
		commands: new Set(["Window.Minimize", "ServerConfig.GetBaseURL"]),
	});

	assert.deepEqual(result.missing, ["ServerConfig.GetBaseURL"]);
	assert.deepEqual(result.unused, []);
});

test("reports bound methods the SDK never calls, as a warning", () => {
	const result = checkCommands({
		bindings: new Set(["Window.Minimize", "DeepLink.Open"]),
		commands: new Set(["Window.Minimize"]),
	});

	assert.deepEqual(result.missing, []);
	assert.deepEqual(result.unused, ["DeepLink.Open"]);
	// 反向缺失只是提示：有些方法由宿主自身或操作系统调用，SDK 不经过。
	assert.equal(result.ok, true);
});

test("a missing command fails the check even when nothing else is wrong", () => {
	const result = checkCommands({
		bindings: new Set(["Window.Minimize"]),
		commands: new Set(["window.minimize"]),
	});

	assert.deepEqual(result.missing, ["window.minimize"]);
	assert.equal(result.ok, false);
});

test("ignores Go methods that are not exported or not on bound types", () => {
	const bindings = extractGoBindings([
		{ path: "main.go", source: goSource },
		{
			path: "window.go",
			source: `
package main

func (Window) Minimize(ctx context.Context) {}
func (window) hidden(ctx context.Context) {}
func (proxyServer) serveDist() {}
`,
		},
	]);

	assert.deepEqual([...bindings], ["Window.Minimize"]);
});
```

- [ ] **步骤 2：运行测试验证失败**

运行：`node --test scripts/check-desktop-commands.test.mjs`
预期：FAIL，报错 `Cannot find module './check-desktop-commands.mjs'`

- [ ] **步骤 3：编写最少实现代码**

创建 `scripts/check-desktop-commands.mjs`：

```js
#!/usr/bin/env node
// 校验桌面桥接契约：MyGo 以 "<Struct>.<Method>" 注册绑定方法（见
// `mygo generate` 的产物 app/desktop/mygo.ts），TS SDK 必须逐字使用同一串。
// 任一侧多出或缺失都会在运行时静默失败（Promise rejected），因此在 CI 拦截。
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { pathToFileURL } from "node:url";

const DESKTOP_DIR = "app/desktop";
const TS_DIR = "packages/desktop/src/capabilities";

/** 从 Go 源码里取出 `mygo.Bind(X{})` 绑定的类型名。 */
export function extractBoundTypes(source) {
	const types = new Set();
	for (const m of source.matchAll(/mygo\.Bind\(\s*&?([A-Z]\w*)\s*\{\s*\}\s*\)/g)) {
		types.add(m[1]);
	}
	return types;
}

/** 从一组 Go 源文件里取出 `<Type>.<Method>` 形式的绑定方法全集。 */
export function extractGoBindings(files) {
	const bound = new Set();
	for (const file of files) {
		for (const t of extractBoundTypes(file.source)) bound.add(t);
	}
	const methods = new Set();
	for (const file of files) {
		for (const m of file.source.matchAll(/^func\s+\(([A-Z]\w*)\)\s+([A-Z]\w*)\s*\(/gm)) {
			const [, recv, method] = m;
			if (!bound.has(recv)) continue;
			methods.add(`${recv}.${method}`);
		}
	}
	return methods;
}

/** 从 TS SDK 里取出 `command("X", "Y")` 的命令名全集。 */
export function extractTsCommands(files) {
	const commands = new Set();
	for (const file of files) {
		for (const m of file.source.matchAll(/command\(\s*"([A-Z]\w*)"\s*,\s*"([A-Z]\w*)"\s*\)/g)) {
			commands.add(`${m[1]}.${m[2]}`);
		}
	}
	return commands;
}

/** 双向比对，返回两侧的差集。只有 missing 会让校验失败。 */
export function checkCommands({ bindings, commands }) {
	const missing = [...commands].filter((c) => !bindings.has(c)).sort();
	const unused = [...bindings].filter((b) => !commands.has(b)).sort();
	return { missing, unused, ok: missing.length === 0 };
}

function goFiles(dir) {
	return readdirSync(dir)
		.filter((f) => f.endsWith(".go") && !f.endsWith("_test.go"))
		.map((f) => ({ path: join(dir, f), source: readFileSync(join(dir, f), "utf8") }));
}

function tsFiles(dir) {
	return readdirSync(dir)
		.filter((f) => f.endsWith(".ts"))
		.map((f) => ({ path: join(dir, f), source: readFileSync(join(dir, f), "utf8") }));
}

export function checkRepo(root = ".") {
	const bindings = extractGoBindings(goFiles(join(root, DESKTOP_DIR)));
	const commands = extractTsCommands(tsFiles(join(root, TS_DIR)));
	return { bindings, commands, ...checkCommands({ bindings, commands }) };
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
	const { missing, unused } = checkRepo(".");
	for (const b of unused) {
		// 只是提示：有些绑定方法由宿主自身调用（托盘、生命周期），SDK 不经过。
		console.warn(`::warning::宿主绑定了但 SDK 未调用: ${b}`);
	}
	for (const c of missing) {
		console.error(`::error::SDK 调用了宿主未绑定的方法: ${c}`);
	}
	if (missing.length > 0) process.exit(1);
	console.log("desktop bridge commands are in sync");
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`node --test scripts/check-desktop-commands.test.mjs`
预期：PASS，5 个测试全绿

- [ ] **步骤 5：在 `bridge.ts` 增加 `command()`**

修改 `packages/desktop/src/bridge.ts`，在 `DesktopBridge` 接口之后加入：

```ts
/**
 * Builds the command key MyGo registers for a bound service method.
 *
 * MyGo binds `mygo.Bind(Window{})` as `"Window"` and each exported method as
 * `"<Struct>.<Method>"` — `"Window.Minimize"`, `"ServerConfig.GetBaseURL"`.
 * Call this instead of writing the string by hand: a mismatch rejects at
 * runtime with "method X is not bound" and is easy to miss, because most
 * capability methods swallow their errors. `scripts/check-desktop-commands.mjs`
 * verifies every call site against the Go side in CI.
 */
export function command(service: string, method: string): string {
	return `${service}.${method}`;
}
```

- [ ] **步骤 6：把 `capabilities/*.ts` 全部改用 `command()`**

对 `packages/desktop/src/capabilities/` 下每个文件，把 `bridge.invoke("x.y", ...)` 改为 `bridge.invoke(command("X", "Y"), ...)`，命令名以 `app/desktop/mygo.ts` 为准：

```ts
// window.ts
import { bridge, command } from "../bridge";

const native: WindowControls = {
	minimize: () => bridge.invoke(command("Window", "Minimize")),
	maximize: () => bridge.invoke(command("Window", "Maximize")),
	unmaximize: () => bridge.invoke(command("Window", "Unmaximize")),
	isMaximized: () => bridge.invoke<boolean>(command("Window", "IsMaximized")),
	setAlwaysOnTop: (value) => bridge.invoke(command("Window", "SetAlwaysOnTop"), value),
	setTitle: (title) => bridge.invoke(command("Window", "SetTitle"), title),
	close: () => bridge.invoke(command("Window", "Close")),
};
```

`server.ts`：

```ts
import { bridge, command } from "../bridge";

const native: ServerConfig = {
	getBaseUrl: () => bridge.invoke<string>(command("ServerConfig", "GetBaseURL")),
	setBaseUrl: (url) => bridge.invoke(command("ServerConfig", "SetBaseURL"), url),
};
```

其余文件同理：`AutoStart.*`、`Overlay.*`、`Tray.*`、`Notification.*`、`DeepLink.*`、`Dialog.*`、`Updater.*` 首字母全部大写。

- [ ] **步骤 7：运行校验脚本**

运行：`node scripts/check-desktop-commands.mjs`
预期：输出 `desktop bridge commands are in sync`，退出码 0

- [ ] **步骤 8：Commit**

```bash
git add scripts/check-desktop-commands.mjs scripts/check-desktop-commands.test.mjs packages/desktop/src
git commit -m "fix(desktop): align bridge command keys with mygo bindings"
```

---

## 任务 2：稳定回环端口

**文件：**
- 创建：`app/desktop/proxyport.go`、`app/desktop/proxyport_test.go`
- 修改：`app/desktop/proxyserver.go:56-84`

- [ ] **步骤 1：编写失败的测试**

创建 `app/desktop/proxyport_test.go`：

```go
package main

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestProxyPortRangeIsBelowEphemeralRange(t *testing.T) {
	if proxyPortFirst < 1024 {
		t.Fatalf("proxyPortFirst = %d, want a non-privileged port", proxyPortFirst)
	}
	if proxyPortLast >= 49152 {
		t.Fatalf("proxyPortLast = %d, want below the ephemeral range (49152)", proxyPortLast)
	}
	if proxyPortLast < proxyPortFirst {
		t.Fatalf("proxyPortLast %d < proxyPortFirst %d", proxyPortLast, proxyPortFirst)
	}
}

func TestLoadSavedPortIgnoresMissingAndInvalid(t *testing.T) {
	dir := t.TempDir()
	if got := loadSavedPort(dir); got != 0 {
		t.Fatalf("loadSavedPort on empty dir = %d, want 0", got)
	}
	if err := os.WriteFile(filepath.Join(dir, proxyPortFileName), []byte("{bad json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := loadSavedPort(dir); got != 0 {
		t.Fatalf("loadSavedPort on corrupt file = %d, want 0", got)
	}
}

func TestSavePortRoundTrip(t *testing.T) {
	dir := t.TempDir()
	savePort(dir, 47001)
	if got := loadSavedPort(dir); got != 47001 {
		t.Fatalf("loadSavedPort = %d, want 47001", got)
	}
}

func TestListenLoopbackPrefersSavedPort(t *testing.T) {
	dir := t.TempDir()
	savePort(dir, proxyPortFirst+7)

	ln, port, err := listenLoopback(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()

	if port != proxyPortFirst+7 {
		t.Fatalf("port = %d, want saved %d", port, proxyPortFirst+7)
	}
}

func TestListenLoopbackSkipsPortInUse(t *testing.T) {
	dir := t.TempDir()
	// 占用首选端口，逼它顺延。
	blocked, err := net.Listen("tcp", "127.0.0.1:"+itoa(proxyPortFirst))
	if err != nil {
		t.Skipf("cannot occupy %d: %v", proxyPortFirst, err)
	}
	defer func() { _ = blocked.Close() }()

	ln, port, err := listenLoopback(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()

	if port == proxyPortFirst {
		t.Fatalf("port = %d, want a port other than the occupied %d", port, proxyPortFirst)
	}
	if port < proxyPortFirst || port > proxyPortLast {
		t.Fatalf("port = %d, want within %d-%d", port, proxyPortFirst, proxyPortLast)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd app/desktop && go test ./... -run 'ProxyPort|LoadSavedPort|SavePort|ListenLoopback'`
预期：FAIL，编译错误 `undefined: proxyPortFirst`

- [ ] **步骤 3：编写最少实现代码**

创建 `app/desktop/proxyport.go`：

```go
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/egoist/mygo"
)

// The loopback proxy's port must stay stable across launches: the page's
// localStorage and session cookies are keyed by origin, which includes the
// port. A random port per launch would drop the cached user, the session
// expiry and the theme on every start.
//
// The range sits below 49152, where macOS and Windows start their ephemeral
// ports, so a fixed port here is unlikely to collide with a transient one.
const (
	proxyPortFirst = 47000
	proxyPortLast  = 47099
)

const proxyPortFileName = "proxy-port.json"

type proxyPortFile struct {
	Port int `json:"port"`
}

// loadSavedPort returns the port persisted in dir, or 0 when there is none or
// the file is unreadable/damaged.
func loadSavedPort(dir string) int {
	data, err := os.ReadFile(filepath.Join(dir, proxyPortFileName))
	if err != nil {
		return 0
	}
	var f proxyPortFile
	if json.Unmarshal(data, &f) != nil {
		return 0
	}
	if f.Port < proxyPortFirst || f.Port > proxyPortLast {
		return 0
	}
	return f.Port
}

// savePort persists port in dir. Failures are non-fatal: the next launch just
// picks a port again.
func savePort(dir string, port int) {
	if dir == "" || port == 0 {
		return
	}
	encoded, err := json.MarshalIndent(proxyPortFile{Port: port}, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, proxyPortFileName), encoded, 0o644)
}

// proxyPortDir is the user-data directory holding proxy-port.json.
func proxyPortDir() string {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return ""
	}
	return dir
}

// listenLoopback binds a loopback listener on the saved port when possible,
// then on each port of the range in turn, and finally on any free port. It
// returns the listener, its port, and whether that port is worth saving.
func listenLoopback(dir string) (net.Listener, int, error) {
	saved := loadSavedPort(dir)
	candidates := make([]int, 0, proxyPortLast-proxyPortFirst+2)
	if saved != 0 {
		candidates = append(candidates, saved)
	}
	for p := proxyPortFirst; p <= proxyPortLast; p++ {
		if p == saved {
			continue
		}
		candidates = append(candidates, p)
	}
	for _, p := range candidates {
		ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(p))
		if err == nil {
			if p != saved {
				savePort(dir, p)
			}
			return ln, p, nil
		}
	}
	// Every port in the range is taken: fall back to a system-assigned one.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, 0, fmt.Errorf("listen loopback: %w", err)
	}
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		_ = ln.Close()
		return nil, 0, fmt.Errorf("listen loopback: unexpected address %T", ln.Addr())
	}
	return ln, addr.Port, nil
}

// itoa is strconv.Itoa under a short name for tests.
func itoa(n int) string { return strconv.Itoa(n) }
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd app/desktop && go test ./... -run 'ProxyPort|LoadSavedPort|SavePort|ListenLoopback'`
预期：PASS，5 个测试全绿

- [ ] **步骤 5：让 `startProxyServer` 使用它**

修改 `app/desktop/proxyserver.go`，把 `net.Listen("tcp", "127.0.0.1:0")` 那一段替换为：

```go
	listener, port, err := listenLoopback(proxyPortDir())
	if err != nil {
		return nil, "", err
	}
	go func() {
		_ = http.Serve(listener, mux)
	}()
	return s, fmt.Sprintf("http://127.0.0.1:%d", port), nil
```

同时删掉原来的 `listener, err := net.Listen(...)`、`go func() { _ = http.Serve(...) }()` 和 `return s, fmt.Sprintf(... listener.Addr() ...)` 三处，并去掉不再使用的 `net` import（若 `listenLoopback` 已把它移入 `proxyport.go`）。

- [ ] **步骤 6：运行全部 Go 测试**

运行：`cd app/desktop && go build ./... && go test ./...`
预期：构建通过，全部测试 PASS

- [ ] **步骤 7：Commit**

```bash
git add app/desktop/proxyport.go app/desktop/proxyport_test.go app/desktop/proxyserver.go
git commit -m "feat(desktop): keep the loopback proxy on a stable port"
```

---

## 任务 3：应用图标与托盘图标

**文件：**
- 创建：`app/desktop/resources/icon.png`、`app/desktop/assets/icon.png`、`app/desktop/assets/tray.png`、`app/desktop/assets.go`
- 修改：`app/desktop/tray.go:37`、`app/desktop/main.go`

- [ ] **步骤 1：生成图标资源**

用 Python + Pillow（`load_workspace_dependencies` 提供的解释器）从 `app/web/public/favicon-512.png` 生成三个文件：

```python
from PIL import Image

src = Image.open("app/web/public/favicon-512.png").convert("RGBA")

# 1) 打包用彩色图标：MyGo 会按 1024 基准生成 .icns / .ico / .desktop 图标
src.resize((1024, 1024), Image.LANCZOS).save("app/desktop/resources/icon.png")

# 2) 未打包运行时的 Dock / 窗口图标（go:embed）
src.resize((512, 512), Image.LANCZOS).save("app/desktop/assets/icon.png")

# 3) 托盘模板图：深色墨迹 -> 不透明，绿色底/浅色 -> 透明，纯黑 + alpha。
#    macOS 用 setTemplate: 按菜单栏明暗反色，Windows 深色任务栏同理。
w, h = src.size
ink = Image.new("RGBA", (w, h), (0, 0, 0, 0))
sp, dp = src.load(), ink.load()
for y in range(h):
    for x in range(w):
        r, g, b, a = sp[x, y]
        lum = (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255.0
        alpha = max(0.0, min(1.0, (0.5 - lum) / 0.4)) * (a / 255.0)
        dp[x, y] = (0, 0, 0, int(alpha * 255))

# 裁到墨迹包围盒，留 8% 内边距，保证 18pt 菜单栏里不被切边
bbox = ink.getbbox()
ink = ink.crop(bbox)
side = max(ink.size)
pad = int(side * 0.08)
canvas = Image.new("RGBA", (side + pad * 2, side + pad * 2), (0, 0, 0, 0))
canvas.paste(ink, ((canvas.width - ink.width) // 2, (canvas.height - ink.height) // 2), ink)
canvas.resize((32, 32), Image.LANCZOS).save("app/desktop/assets/tray.png")
```

- [ ] **步骤 2：确认资源尺寸**

运行：`sips -g pixelWidth -g pixelHeight app/desktop/resources/icon.png app/desktop/assets/icon.png app/desktop/assets/tray.png`
预期：`1024 x 1024`、`512 x 512`、`32 x 32`

- [ ] **步骤 3：编写 `assets.go`**

创建 `app/desktop/assets.go`：

```go
package main

import (
	"embed"
	"log"

	"github.com/egoist/mygo"
)

// Desktop-only image assets, embedded so `go run` and `mygo dev` show the
// right icons too: the packaged bundle gets its icon from
// resources/icon.png (mygo.config.ts), which only exists in a build.
//
//go:embed assets/*.png
var embeddedAssets embed.FS

// appIconPNG returns the application icon, or nil when it is missing.
func appIconPNG() []byte {
	data, err := embeddedAssets.ReadFile("assets/icon.png")
	if err != nil || len(data) == 0 {
		return nil
	}
	return data
}

// trayIconPNG returns the monochrome template icon the system tray shows.
func trayIconPNG() []byte {
	data, err := embeddedAssets.ReadFile("assets/tray.png")
	if err != nil || len(data) == 0 {
		return nil
	}
	return data
}

// applyAppIcon sets the Dock icon (macOS) and the window icon (Windows,
// Linux) from the embedded asset. A packaged app already carries the icon, so
// this only matters when running from source; failures are logged and
// non-fatal.
func applyAppIcon() {
	icon := appIconPNG()
	if icon == nil {
		log.Printf("[GOSpeak] app icon asset missing; keeping the platform default")
		return
	}
	if err := mygo.App.Dock.SetIcon(icon); err != nil {
		log.Printf("[GOSpeak] dock icon unavailable: %v", err)
	}
	if mainWindow != nil {
		if err := mainWindow.SetIcon(icon); err != nil {
			log.Printf("[GOSpeak] window icon unavailable: %v", err)
		}
	}
}
```

- [ ] **步骤 4：让托盘使用模板图**

修改 `app/desktop/tray.go` 的 `ensureTray`，把读 favicon 的两行替换为：

```go
	icon := trayIconPNG()
	if len(icon) == 0 {
		log.Printf("[GOSpeak] tray icon asset missing")
		return
	}
```

并把 `mygo.TrayOptions` 里的 `IconIsTemplate` 设为 `true`：

```go
	t, err := mygo.NewTray(mygo.TrayOptions{
		ID:             "gospeak-tray",
		Icon:           icon,
		IconIsTemplate: true,
		ToolTip:        "GOSpeak",
		Menu:           trayMenu(),
	})
```

- [ ] **步骤 5：在 `main.go` 里应用图标**

在 `mygo.App.WhenReady` 里，`openMainWindow()` / dev 窗口创建之后、`initOverlay()` 之前插入：

```go
		applyAppIcon()
```

- [ ] **步骤 6：构建并手工验证**

运行：`cd app/desktop && go build ./...`
预期：构建通过

运行：`cd app/desktop && GOSPEAK_DESKTOP_DEV=1 go run .`（需先 `pnpm dev:web`）
预期：Dock 图标是品牌彩色图；菜单栏出现单色 GOSpeak 图标，切换系统深浅色时自动反色

- [ ] **步骤 7：Commit**

```bash
git add app/desktop/resources app/desktop/assets app/desktop/assets.go app/desktop/tray.go app/desktop/main.go
git commit -m "feat(desktop): ship app icon and a monochrome tray template"
```

---

## 任务 4：窗口尺寸与位置记忆

**文件：**
- 修改：`app/desktop/windows.go:46-55`

- [ ] **步骤 1：给主窗口加 StateKey 与默认尺寸**

修改 `openMainWindow()`：

```go
func openMainWindow() {
	base := appPageBase()
	mainWindow = mygo.NewWindow(mygo.WindowOptions{
		Title: "GOSpeak " + version,
		URL:   base + "/",
		Page:  mygo.PageOptions{TrustedOrigins: originOf(base)},
		// 首次启动的尺寸；StateKey 记住的位置、尺寸、最大化与全屏会覆盖它，
		// 并在保存的显示器不再连接时回落到可见区域居中。
		Width:     1200,
		Height:    800,
		MinWidth:  960,
		MinHeight: 640,
		StateKey:  "main",
	})
}
```

- [ ] **步骤 2：构建并手工验证**

运行：`cd app/desktop && go build ./...`
预期：构建通过

手工验证（release 或 dev 均可）：
1. 启动，把窗口拖到副屏并调整尺寸，最大化一次。
2. 退出应用（托盘「退出」）。
3. 重新启动 —— 窗口回到上次的位置、尺寸与最大化状态。
4. 检查 `~/Library/Application Support/GOSpeak/window-state.json` 出现 `"main"` 键。

- [ ] **步骤 3：Commit**

```bash
git add app/desktop/windows.go
git commit -m "feat(desktop): remember the main window bounds"
```

---

## 任务 5：托盘行为与关窗常驻

**文件：**
- 创建：`app/desktop/lifecycle.go`
- 修改：`app/desktop/tray.go`、`app/desktop/main.go`、`app/desktop/server_config.go`

- [ ] **步骤 1：编写 `lifecycle.go`**

```go
package main

import (
	"sync/atomic"

	"github.com/egoist/mygo"
)

// quitting is set when the user picked "quit" (tray menu, Cmd+Q, app menu).
// Until then, closing the main window hides it and the process keeps running
// in the tray.
var quitting atomic.Bool

// showMainWindow brings the main window back from the tray, restoring it when
// minimized, and gives it focus.
func showMainWindow() {
	if mainWindow == nil {
		return
	}
	mainWindow.Show()
	mainWindow.Restore()
	mainWindow.Focus()
}

// hideMainWindow hides the main window without closing it, so the process
// (and its socket, media session and tray icon) stays alive.
func hideMainWindow() {
	if mainWindow != nil {
		mainWindow.Hide()
	}
}

// initLifecycle wires the "close hides, tray keeps running" behaviour and the
// entry points that bring the window back.
func initLifecycle() {
	// Closing the last window must not quit: the tray keeps the app alive.
	mygo.App.OnWindowAllClosed(func() {})

	if mainWindow != nil {
		mainWindow.OnClose(func(e *mygo.CloseEvent) {
			if quitting.Load() {
				return
			}
			e.PreventDefault()
			hideMainWindow()
		})
	}

	// Dock icon click (macOS) reopens the hidden window.
	mygo.App.OnActivate(func(hasVisibleWindows bool) {
		if !hasVisibleWindows {
			showMainWindow()
		}
	})

	// A second launch focuses the running instance instead of starting a new
	// process.
	mygo.App.OnSecondInstance(func([]string, string) { showMainWindow() })
}

// requestQuit lets the close handler through and quits the app.
func requestQuit() {
	quitting.Store(true)
	mygo.App.Quit()
}
```

- [ ] **步骤 2：重写托盘菜单**

修改 `app/desktop/tray.go`，把 `ensureTray` 里内联的 `Menu` 换成函数，并在文件末尾加 `trayMenu()`：

```go
// trayMenu builds the tray menu: open the window, hide it, or quit for real.
func trayMenu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{
			Label: "打开 GOSpeak",
			Click: func(*mygo.MenuItem, *mygo.Window) { showMainWindow() },
		},
		{
			Label: "隐藏到托盘",
			Click: func(*mygo.MenuItem, *mygo.Window) { hideMainWindow() },
		},
		mygo.Separator(),
		{
			Label: "退出",
			Click: func(*mygo.MenuItem, *mygo.Window) { requestQuit() },
		},
	})
}
```

- [ ] **步骤 3：在 `main.go` 里接线**

把 `main()` 开头改为（单实例锁必须在 `Bind` 之后、`Run` 之前；`RequestSingleInstanceLock` 在 `MYGO_GENERATE` 下自动放行）：

```go
func main() {
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}

	mygo.Bind(Window{})
	// ...其余 Bind 不变
```

在 `WhenReady` 里 `ensureTray()` 之后追加：

```go
		initLifecycle()
```

- [ ] **步骤 4：换服务器时清理旧会话**

修改 `app/desktop/server_config.go` 的 `SetBaseURL`，在「持久化成功、切换代理目标」之后、`reloadAllWindows()` 之前插入：

```go
	// Cookies live on the loopback origin, not on the backend: without this,
	// switching servers would carry the previous server's session to the new
	// one.
	_ = mygo.Cookies.Clear()
	// The page keeps the cached user and the session expiry in localStorage;
	// clear them so it re-authenticates against the new server.
	if mainWindow != nil {
		if p := mainWindow.Page(); p != nil {
			_, _ = p.Eval(`(() => {
				try {
					localStorage.removeItem("user");
					localStorage.removeItem("gospeak_session_expires_at");
				} catch {}
			})()`)
		}
	}
```

- [ ] **步骤 5：构建并手工验证**

运行：`cd app/desktop && go build ./... && go vet ./...`
预期：构建与 vet 通过

手工验证：
1. 启动，点窗口关闭按钮 —— 窗口消失，菜单栏托盘图标仍在，进程未退出。
2. 点托盘「打开 GOSpeak」—— 窗口回来并聚焦。
3. 点托盘「隐藏到托盘」—— 窗口隐藏。
4. 点 Dock 图标 —— 隐藏的窗口重新出现。
5. 再次双击应用图标启动 —— 不出现第二个进程，已有窗口聚焦。
6. 点托盘「退出」—— 进程结束。
7. 窗口最小化按钮 —— 正常最小化到 Dock，不是隐藏。

- [ ] **步骤 6：Commit**

```bash
git add app/desktop/lifecycle.go app/desktop/tray.go app/desktop/main.go app/desktop/server_config.go
git commit -m "feat(desktop): keep running in the tray when the window closes"
```

---

## 任务 6：登录态持久化加固

**文件：**
- 修改：`app/web/src/api/authTransport.ts`、`app/web/src/stores/userStore.ts`、`app/web/src/stores/userStore.test.ts`

- [ ] **步骤 1：编写失败的测试**

在 `app/web/src/stores/userStore.test.ts` 末尾追加：

```ts
describe("userStore 网络错误不踢下线", () => {
	beforeEach(() => {
		localStorage.clear();
		userStore.clearAuth();
		getProfileMock.mockReset();
		refreshSessionMock.mockReset();
	});

	it("无网络（非鉴权错误）：保留缓存会话并放行", async () => {
		await userStore.login(fakeUser);
		getProfileMock.mockRejectedValue(new Error("Network Error"));
		refreshSessionMock.mockRejectedValue(new Error("Network Error"));

		await expect(userStore.ensureSession()).resolves.toBe(true);
		expect(userStore.user()).not.toBeNull();
		expect(localStorage.getItem("user")).not.toBeNull();
	});

	it("服务器 5xx：保留缓存会话并放行", async () => {
		await userStore.login(fakeUser);
		const serverError = { response: { status: 502, data: {} } };
		getProfileMock.mockRejectedValue(serverError);
		refreshSessionMock.mockRejectedValue(serverError);

		await expect(userStore.ensureSession()).resolves.toBe(true);
		expect(userStore.user()).not.toBeNull();
	});

	it("鉴权错误码仍然清会话", async () => {
		await userStore.login(fakeUser);
		getProfileMock.mockRejectedValue(authError(1003));
		refreshSessionMock.mockRejectedValue(authError(1003));

		await expect(userStore.ensureSession()).resolves.toBe(false);
		expect(userStore.user()).toBeNull();
	});

	it("无缓存用户 + 网络错误：不放行（没有可沿用的会话）", async () => {
		getProfileMock.mockRejectedValue(new Error("Network Error"));
		refreshSessionMock.mockRejectedValue(new Error("Network Error"));

		await expect(userStore.ensureSession()).resolves.toBe(false);
	});
});
```

- [ ] **步骤 2：运行测试验证失败**

运行：`pnpm --filter @gospeak/web test -- userStore`
预期：FAIL，「无网络（非鉴权错误）：保留缓存会话并放行」得到 `false`

- [ ] **步骤 3：在 `authTransport.ts` 增加判定**

在 `app/web/src/api/authTransport.ts` 末尾追加：

```ts
/** 服务端下发的、表示"凭据确实无效"的错误码。其余（网络/5xx）不代表会话已死。 */
const AUTH_FAILURE_CODES = new Set([1001, 1002, 1003, 1014, 1015]);

/**
 * 判断一个错误是否意味着会话真的失效。
 *
 * 只有服务端明确回了 token 相关错误码才算；网络中断、超时、502/503
 * 都不算 —— 离线启动时把用户踢下线是最糟的体验。
 */
export function isAuthFailure(error: unknown): boolean {
	const code = (error as { response?: { data?: { code?: number } } })?.response
		?.data?.code;
	return typeof code === "number" && AUTH_FAILURE_CODES.has(code);
}
```

- [ ] **步骤 4：改 `ensureSession` 的清理条件**

修改 `app/web/src/stores/userStore.ts`：

1. 顶部 import 加入 `isAuthFailure`：

```ts
import {
	isAuthFailure,
	readSessionExpiry,
	recordSessionExpiry as persistSessionExpiry,
	refreshSession,
} from "@/api/authTransport";
```

2. 把 `ensureSession` 的 `catch` 块替换为：

```ts
			} catch (e) {
				if (isRateLimitedError(e) && get().user) return true;
				// 网络中断 / 超时 / 5xx：会话未必失效，保留缓存用户继续用，
				// 下次请求仍会走拦截器重试或刷新。
				if (!isAuthFailure(e) && get().user) return true;
				get().clearAuth();
				return false;
			}
```

- [ ] **步骤 5：运行测试验证通过**

运行：`pnpm --filter @gospeak/web test -- userStore`
预期：PASS，全部用例（含原有 13 个）通过

- [ ] **步骤 6：Commit**

```bash
git add app/web/src/api/authTransport.ts app/web/src/stores/userStore.ts app/web/src/stores/userStore.test.ts
git commit -m "fix(web): keep the cached session through network errors"
```

---

## 任务 7：文档同步与整体验证

**文件：**
- 修改：`packages/desktop/README.md`、`AGENTS.md`、`app/desktop/README.md`

- [ ] **步骤 1：修正 `packages/desktop/README.md` 的命令名契约**

把「方法名契约」那一条改为：

```md
- 方法名契约：`<Struct>.<Method>`，与 Go 侧逐字一致，即 Go 的 `Overlay.PushState` → `"Overlay.PushState"`、
  `ServerConfig.GetBaseURL` → `"ServerConfig.GetBaseURL"`。用 `bridge.ts` 的 `command("Overlay", "PushState")`
  构造，不要手写字符串；`scripts/check-desktop-commands.mjs` 会在 CI 双向校验 Go 绑定与 SDK 调用。
```

同时把代码示例里的 `"overlay.setEnabled"` 改为 `command("Overlay", "SetEnabled")`。

- [ ] **步骤 2：修正 `AGENTS.md`**

在「Web / 桌面能力边界与条件编译」一节的硬性规则里追加第 5 条：

```md
5. 桥接命令名必须是 `<Struct>.<Method>`（与 `mygo.Bind` 的类型名和方法名逐字一致），用 `@gospeak/desktop` 的 `command()` 构造；新增或改名绑定方法后跑 `node scripts/check-desktop-commands.mjs`，CI 会拦下不匹配的调用。
```

- [ ] **步骤 3：更新 `app/desktop/README.md` 的目录表**

在目录表里补两行：

```md
| `resources/icon.png` | 打包用应用图标（1024×1024），MyGo 生成 .icns / .ico / .desktop 图标 |
| `assets/` | go:embed 的运行时图标：`icon.png`（Dock/窗口兜底）、`tray.png`（单色托盘模板图） |
| `proxyport.go` | 回环代理的稳定端口选择与持久化（`proxy-port.json`） |
| `lifecycle.go` | 关窗进托盘、Dock 激活、单实例聚焦与退出标记 |
```

- [ ] **步骤 4：跑完整验证**

```bash
node scripts/check-desktop-commands.mjs
node --test scripts/check-desktop-commands.test.mjs
cd app/desktop && go build ./... && go vet ./... && go test ./...
cd ../.. && pnpm --filter @gospeak/web test -- userStore
pnpm typecheck
pnpm lint
```

预期：全部通过

- [ ] **步骤 5：写测试日志**

按 `AGENTS.md` §Test Logging 的要求，把上面的命令与输出摘要写入 `agent_test_logs/desktop-shell-icon-window-tray-<时间>.md`。

- [ ] **步骤 6：Commit**

```bash
git add packages/desktop/README.md AGENTS.md app/desktop/README.md agent_test_logs
git commit -m "docs(desktop): document the bridge command contract and new shell files"
```

---

## 自检

**1. 规格覆盖度**

| 规格章节 | 覆盖任务 |
| --- | --- |
| 1. 修复桥接契约 | 任务 1 |
| 2. 稳定的回环 origin | 任务 2 |
| 3. 登录态持久化加固 | 任务 6（会话判定）+ 任务 5 步骤 4（换服务器清凭据） |
| 4. 窗口尺寸记忆 | 任务 4 |
| 5. 应用图标 | 任务 3 |
| 6. 托盘行为 | 任务 5 |

**2. 占位符扫描**：无 TODO / 待定；每个代码步骤都给了可直接粘贴的代码。

**3. 类型一致性**：
- `command(service, method)` 在任务 1 定义，任务 1 步骤 6 全部调用点使用。
- `proxyPortFirst` / `proxyPortLast` / `proxyPortFileName` / `loadSavedPort` / `savePort` / `listenLoopback` 在任务 2 定义并被同任务测试与 `proxyserver.go` 使用。
- `appIconPNG` / `trayIconPNG` / `applyAppIcon` 在任务 3 定义，任务 3 步骤 4、5 使用。
- `quitting` / `showMainWindow` / `hideMainWindow` / `initLifecycle` / `requestQuit` / `trayMenu` 在任务 5 定义，任务 5 的 `tray.go` 与 `main.go` 使用。
- `isAuthFailure` 在任务 6 步骤 3 定义，步骤 4 使用。
