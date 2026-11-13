# 游戏内覆盖层原生渲染 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 用 mygo 原生 UI（`github.com/egoist/mygo/ui`）在 GPU 上直接绘制游戏内覆盖层 HUD，去掉第二个 WebView，同时把窗口改成真正的原生覆盖层（Panel、鼠标穿透、不抢焦点、跟随系统主题）。

**架构：** 窗口从 `URL: /overlay` 改为 `Panel: true` + `Content: ui.View(...)`。主窗口仍通过 `useOverlayRelay` 把房间状态推给宿主（`Overlay.PushState`），宿主保存快照并 `win.Invalidate()` 触发原生重绘——不再 `Broadcast` 给第二个窗口。显示策略由「前台有全屏应用」改为「离开 GOSpeak」，用 `mygo.App.IsActive()` + 激活事件驱动，删除全部 Win32 全屏检测与轮询。

**技术栈：** Go 1.27.3 + MyGo 0.8.3（`ui` 包）、React 19、TypeScript、Tailwind 4。

**Spec:** `docs/superpowers/specs/2026-11-12-game-overlay-native-design.md`

## 全局约束

- **Go 1.27.3**（`app/desktop/go.mod` 的 `go` 指令）；本机 `go` 需 `GOTOOLCHAIN=auto` 才能构建。
- **`app/desktop` 是独立 Go module**，与 `app/server` 的 Go 1.26 不同，命令必须在 `app/desktop` 目录下执行。
- **代码中不加注释**，除非复杂逻辑必须；**不用 emoji**（`CLAUDE.md` 规则）。
- **`go vet ./...` 是唯一能进 CI 的桌面端门禁**（`go test` 需要真实 `webui/dist` 产物，CI 只放占位文件）。新增测试在本地跑。
- 构建/测试前若缺 `app/desktop/webui/dist/index.html`，先造占位：`mkdir -p app/desktop/webui/dist && printf '<!doctype html><title>placeholder</title>\n' > app/desktop/webui/dist/index.html`
- **步骤 1 与步骤 2 必须连续完成，中间不可发布**（中间态 HUD 能看见但完全不能操作）。
- HUD 配色**跟随系统主题**，描影随主题反向翻转（深色模式黑晕、浅色模式白晕）。
- HUD **不做**动效、**不显示**房间名、**不做**卡片背景。
- 覆盖层窗口**不可关闭**：无标题栏 + `DisableClose` + `OnClose` 拦截（`DisableClose` 拦不住 Escape）。
- 显示时一律用 **`ShowInactive`**，绝不用 `Show`（Windows 会抢前台形成闪烁循环）。
- 防录屏默认**关闭**，作为可选开关。

---

## 文件结构

| 文件 | 职责 |
| --- | --- |
| 创建 `app/desktop/overlay_view.go` | HUD 原生视图：配色、行布局、图标、高度计算 |
| 创建 `app/desktop/overlay_view_test.go` | 视图无头测试（`ui.NewTester`） |
| 创建 `app/desktop/overlay_avatar.go` | 头像异步加载与缓存 |
| 创建 `app/desktop/overlay_avatar_test.go` | 头像加载测试（`httptest`） |
| 创建 `app/desktop/overlay_policy.go` | 显示策略纯函数求值 |
| 创建 `app/desktop/overlay_policy_test.go` | 策略表驱动测试 |
| 创建 `app/desktop/overlay_scale.go` | 屏幕自适应缩放系数与窗口尺寸换算 |
| 创建 `app/desktop/overlay_scale_test.go` | 缩放测试 |
| 修改 `app/desktop/overlay.go` | 窗口层：Panel、Content、ShowInactive、穿透、OnClose、策略接线；删全屏检测与 ticker |
| 删除 `app/desktop/overlay_fullscreen_windows.go` | 88 行 Win32 全屏检测 |
| 删除 `app/desktop/overlay_fullscreen_other.go` | 10 行恒 false 桩 |
| 修改 `app/desktop/windows.go` | `reloadAllWindows` 删掉 `w.Page()` 分支 |
| 修改 `app/desktop/server_config.go` | `SetBaseURL` 清空头像缓存 |
| 修改 `app/desktop/lifecycle.go` | 托盘菜单加「锁定/解锁覆盖层位置」 |
| 修改 `app/desktop/proxyserver_test.go` | `/overlay` 深链断言换成 `/domain/test` |
| 修改 `app/desktop/README.md` | 覆盖层一节重写 |
| 修改 `packages/desktop/src/capabilities/overlay.ts` | `always` 策略、`contentProtection`、删 `onState` |
| 修改 `app/web/src/components/modal/settingModal/tab_item/desktop.tsx` | 三选策略、防录屏开关、无边框提示 |
| 修改 `app/web/src/hooks/useOverlayRelay.ts` | 删 `/overlay` 自推判断 |
| 修改 `app/web/src/main.tsx` | 删 `/overlay` 麦克风权限判断 |
| 修改 `app/web/src/layouts/layout.tsx` | 注释更新 |
| 删除 `app/web/src/pages/overlay.tsx` | 覆盖层不再有 Web 路由 |
| 删除 `app/web/src/stores/overlayStore.ts` | 同上 |
| 重生成 `app/web/src/routeTree.gen.ts` | 去掉 `/overlay` |

---

### Task 1: 窗口层改造（保留旧 WebView HUD）

**Files:**
- Modify: `app/desktop/overlay.go`
- Modify: `app/desktop/windows.go`
- Delete: `app/desktop/overlay_fullscreen_windows.go`
- Delete: `app/desktop/overlay_fullscreen_other.go`
- Test: 手动（窗口行为无法自动化）

**Interfaces:**
- Consumes: 无（首个任务）
- Produces: `overlayWindow() *mygo.Window` 不变；`isFullscreenAppInFront()` 被删除，后续任务不得引用。

- [ ] **Step 1: 删除全屏检测文件**

```bash
cd /Users/noelorin/Projects/GOSpeak
git rm app/desktop/overlay_fullscreen_windows.go app/desktop/overlay_fullscreen_other.go
```

- [ ] **Step 2: 从 overlay.go 删掉全屏检测相关代码**

在 `app/desktop/overlay.go` 中删除：
- `IsFullscreenAppInFront()` 方法（约 283-285 行）
- `startOverlayPolicyLoop()` 函数（约 395-405 行）
- `evaluateOverlayPolicy()` 里对 `isFullscreenAppInFront()` 的调用
- `initOverlay()` 里对 `startOverlayPolicyLoop()` 的调用

`evaluateOverlayPolicy()` 暂时改成只依据连接状态（Task 3 再换成 `App.IsActive()`）：

```go
func evaluateOverlayPolicy() {
	overlayCfg.mu.Lock()
	if !overlayCfg.enabled || overlayCfg.policy != overlayPolicyAuto {
		overlayCfg.mu.Unlock()
		return
	}
	want := overlayCfg.snap.Connected
	if want != overlayCfg.lastAutoWant {
		overlayCfg.lastAutoWant = want
		overlayCfg.manualOverride = nil
	}
	var target bool
	if overlayCfg.manualOverride != nil {
		target = *overlayCfg.manualOverride
	} else {
		target = want
	}
	overlayCfg.mu.Unlock()
	setOverlayVisible(target)
}
```

- [ ] **Step 3: 窗口选项改成 Panel + 不可关闭**

把 `createOverlayWindow()` 的 `mygo.WindowOptions` 改成：

```go
func createOverlayWindow() *mygo.Window {
	overlayCfg.mu.Lock()
	opacity := overlayCfg.opacity
	overlayCfg.mu.Unlock()

	base := appPageBase()
	w := mygo.NewWindow(mygo.WindowOptions{
		Title:        "GOSpeak Overlay " + version,
		URL:          base + "/overlay",
		Page:         mygo.PageOptions{TrustedOrigins: originOf(base)},
		Panel:        true,
		Frameless:    true,
		DisableClose: true,
		Transparent:  true,
		DisableShadow: true,
		AlwaysOnTop:  true,
		SkipTaskbar:  true,
		DisableResize: true,
		Opacity:      opacity,
		Width:        240,
		Height:       360,
		X:            24,
		Y:            24,
		StateKey:     "overlay",
		Hidden:       true,
	})
	w.OnClose(func(e *mygo.CloseEvent) {
		e.PreventDefault()
		w.Hide()
	})
	return w
}
```

- [ ] **Step 4: 显示改用 ShowInactive，并接入鼠标穿透**

`setOverlayVisible()` 里把 `w.Show()` 改成 `w.ShowInactive()`：

```go
	if v {
		if w := overlayWindow(); w != nil {
			w.ShowInactive()
		} else {
			ensureOverlayWindow()
		}
	} else if w := overlayWindow(); w != nil {
		w.Hide()
	}
```

`ensureOverlayWindow()` 的创建回调里同样改成 `w.ShowInactive()`。

在 `setOverlayLocked()` 末尾加上穿透切换：

```go
func setOverlayLocked(locked bool) {
	overlayCfg.mu.Lock()
	changed := overlayCfg.locked != locked
	overlayCfg.locked = locked
	overlayCfg.mu.Unlock()
	if !changed {
		return
	}
	if w := overlayWindow(); w != nil {
		w.SetIgnoreMouseEvents(locked)
	}
	_ = overlayLockEvent.Broadcast(locked)
}
```

新建窗口时也要应用当前锁定状态，在 `ensureOverlayWindow()` 的创建回调里、`w.ShowInactive()` 之前加：

```go
		overlayCfg.mu.Lock()
		locked := overlayCfg.locked
		overlayCfg.mu.Unlock()
		w.SetIgnoreMouseEvents(locked)
```

- [ ] **Step 5: 加防录屏配置与方法**

`OverlayConfig` 加字段：

```go
type OverlayConfig struct {
	Enabled           bool    `json:"enabled"`
	Opacity           float64 `json:"opacity"`
	Hotkey            string  `json:"hotkey"`
	LockHotkey        string  `json:"lockHotkey"`
	Policy            string  `json:"policy"`
	ContentProtection bool    `json:"contentProtection"`
}
```

`overlayCfg` 加 `contentProtection bool` 字段，默认 `false`。

`GetConfig()` 和 `writeOverlayConfig()` 里补上 `ContentProtection: overlayCfg.contentProtection`。

`loadOverlayConfig()` 里补上 `overlayCfg.contentProtection = cfg.ContentProtection`。

新增方法：

```go
func (Overlay) SetContentProtection(v bool) {
	overlayCfg.mu.Lock()
	overlayCfg.contentProtection = v
	overlayCfg.mu.Unlock()
	scheduleOverlayConfigSave()
	if w := overlayWindow(); w != nil {
		w.SetContentProtection(v)
	}
}
```

窗口创建时应用，在 `ensureOverlayWindow()` 回调里加：

```go
		overlayCfg.mu.Lock()
		protect := overlayCfg.contentProtection
		overlayCfg.mu.Unlock()
		w.SetContentProtection(protect)
```

- [ ] **Step 6: 清理 reloadAllWindows 的死分支**

`app/desktop/windows.go` 的 `reloadAllWindows()` 删掉覆盖层分支（原生窗口 `Page()` 恒 nil）：

```go
func reloadAllWindows() {
	if mainWindow != nil {
		if p := mainWindow.Page(); p != nil {
			_ = p.LoadURL(appPageBase() + "/")
		}
	}
}
```

- [ ] **Step 7: 编译验证**

```bash
cd /Users/noelorin/Projects/GOSpeak/app/desktop
mkdir -p webui/dist && test -f webui/dist/index.html || printf '<!doctype html><title>placeholder</title>\n' > webui/dist/index.html
GOTOOLCHAIN=auto go vet ./...
```

Expected: 无输出（通过）。若报 `isFullscreenAppInFront` 未定义，说明 Step 2 有遗漏。

- [ ] **Step 8: 手动验证窗口行为**

```bash
cd /Users/noelorin/Projects/GOSpeak
pnpm dev:web   # 另一个终端
GOSPEAK_DESKTOP_DEV=1 pnpm --filter @gospeak/desktop-app dev
```

在设置页开启覆盖层、策略选「手动」，然后按显隐热键：

- HUD 出现，且**游戏/别的 App 仍是前台**（不抢焦点）
- 按 **Escape，HUD 不消失**（验证 `OnClose` 拦截）
- HUD 不出现在 Alt+Tab / Mission Control
- 锁定状态下点 HUD 位置，点击穿透到下层窗口

- [ ] **Step 9: 提交**

```bash
cd /Users/noelorin/Projects/GOSpeak
git add -A
git commit -m "refactor(desktop): 覆盖层改为 Panel 窗口并删除全屏检测"
```

---

### Task 2: 原生 HUD 视图

**Files:**
- Create: `app/desktop/overlay_view.go`
- Create: `app/desktop/overlay_view_test.go`
- Modify: `app/desktop/overlay.go`
- Delete: `app/web/src/pages/overlay.tsx`
- Delete: `app/web/src/stores/overlayStore.ts`
- Modify: `app/web/src/main.tsx`
- Modify: `app/web/src/hooks/useOverlayRelay.ts`
- Modify: `app/web/src/layouts/layout.tsx`
- Regenerate: `app/web/src/routeTree.gen.ts`

**Interfaces:**
- Consumes: `OverlaySnapshot` / `OverlayMember`（`app/desktop/overlay.go` 已定义）
- Produces:
  - `type hudState struct { members []OverlayMember; speaking map[string]bool; avatarOf func(string) *ui.Bitmap }`
  - `func hudView(s *hudState) func(*ui.Context)`
  - `func hudHeight(n int) int` — 返回窗口高度 DIPs
  - `const hudWidth = 200`、`const hudRowHeight = 22`、`const hudMaxRows = 12`

- [ ] **Step 1: 写视图的失败测试**

创建 `app/desktop/overlay_view_test.go`：

```go
package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func testSnapshot(names ...string) OverlaySnapshot {
	members := make([]OverlayMember, 0, len(names))
	for _, n := range names {
		members = append(members, OverlayMember{Identity: n, Name: n})
	}
	return OverlaySnapshot{RoomName: "r", Members: members, Connected: true}
}

func newHUD(snap OverlaySnapshot, speaking []string) *hudState {
	s := &hudState{
		members:  snap.Members,
		speaking: make(map[string]bool, len(speaking)),
		avatarOf: func(string) *ui.Bitmap { return nil },
	}
	for _, id := range speaking {
		s.speaking[id] = true
	}
	return s
}

func TestHUDShowsMemberNames(t *testing.T) {
	s := newHUD(testSnapshot("阿伟", "小林", "Kate"), nil)
	tt := ui.NewTester(hudView(s), hudWidth, hudHeight(3))
	if got := tt.Texts(); len(got) != 3 {
		t.Fatalf("Texts() = %v, want 3 names", got)
	}
	for _, want := range []string{"阿伟", "小林", "Kate"} {
		if !tt.HasText(want) {
			t.Errorf("HUD does not show %q; Texts() = %v", want, tt.Texts())
		}
	}
}

func TestHUDEmptyDrawsNothing(t *testing.T) {
	s := newHUD(testSnapshot(), nil)
	tt := ui.NewTester(hudView(s), hudWidth, hudHeight(0))
	if got := tt.Texts(); len(got) != 0 {
		t.Fatalf("Texts() = %v, want none for an empty room", got)
	}
}

func TestHUDSpeakingComesFirst(t *testing.T) {
	s := newHUD(testSnapshot("阿伟", "小林", "Kate"), []string{"Kate"})
	tt := ui.NewTester(hudView(s), hudWidth, hudHeight(3))
	got := tt.Texts()
	if len(got) != 3 || got[0] != "Kate" {
		t.Fatalf("Texts() = %v, want Kate first", got)
	}
}

func TestHUDCapsRowsWithOverflow(t *testing.T) {
	names := make([]string, 15)
	for i := range names {
		names[i] = string(rune('A' + i))
	}
	s := newHUD(testSnapshot(names...), nil)
	tt := ui.NewTester(hudView(s), hudWidth, hudHeight(15))
	if !tt.HasText("+4") {
		t.Errorf("HUD does not show +4 for 15 members; Texts() = %v", tt.Texts())
	}
	if tt.HasText("O") {
		t.Errorf("HUD shows the 15th member instead of +N; Texts() = %v", tt.Texts())
	}
}

func TestHUDHeight(t *testing.T) {
	cases := []struct{ n, want int }{
		{0, 8}, {1, 30}, {3, 74}, {12, 272}, {15, 272},
	}
	for _, c := range cases {
		if got := hudHeight(c.n); got != c.want {
			t.Errorf("hudHeight(%d) = %d, want %d", c.n, got, c.want)
		}
	}
}

func TestHUDThemeShadows(t *testing.T) {
	s := newHUD(testSnapshot("阿伟"), nil)
	tt := ui.NewTester(hudView(s), hudWidth, hudHeight(1))
	if !tt.HasText("阿伟") {
		t.Fatal("dark frame does not show the member")
	}
	tt.SetDark(false)
	if !tt.HasText("阿伟") {
		t.Fatal("light frame does not show the member")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd /Users/noelorin/Projects/GOSpeak/app/desktop
GOTOOLCHAIN=auto go test ./... -run TestHUD -v
```

Expected: FAIL，`undefined: hudState` / `undefined: hudView` / `undefined: hudHeight` / `undefined: hudWidth`。

- [ ] **Step 3: 实现视图**

创建 `app/desktop/overlay_view.go`：

```go
package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	hudWidth     = 200
	hudRowHeight = 22
	hudMaxRows   = 12
	hudPadding   = 4
	hudAvatar    = 16
	hudGap       = 6
)

var micIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 19v3"/><path d="M19 10v2a7 7 0 0 1-14 0v-2"/><rect x="9" y="2" width="6" height="13" rx="3"/></svg>`))

var micOffIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="2" x2="22" y1="2" y2="22"/><path d="M18.89 13.23A7.12 7.12 0 0 0 19 12v-2"/><path d="M5 10v2a7 7 0 0 0 12 5"/><path d="M15 9.34V5a3 3 0 0 0-5.68-1.33"/><path d="M9 9v3a3 3 0 0 0 5.12 2.12"/><path d="M12 19v3"/></svg>`))

type hudState struct {
	members  []OverlayMember
	speaking map[string]bool
	avatarOf func(identity string) *ui.Bitmap
}

func hudHeight(n int) int {
	if n > hudMaxRows {
		n = hudMaxRows
	}
	return hudPadding*2 + n*hudRowHeight
}

func hudView(s *hudState) func(*ui.Context) {
	return func(c *ui.Context) {
		t := c.Theme()
		root := c.Root()
		root.Background(ui.Transparent)
		if len(s.members) == 0 {
			return
		}
		shadow := ui.RGBA(0, 0, 0, 0.9)
		if !t.Dark {
			shadow = ui.RGBA(255, 255, 255, 0.9)
		}
		textColor := t.Text
		mutedColor := t.TextMuted
		ui.Column(c).Padding(hudPadding).Gap(0).Children(func() {
			shown := s.members
			overflow := 0
			if len(shown) > hudMaxRows {
				overflow = len(shown) - (hudMaxRows - 1)
				shown = shown[:hudMaxRows-1]
			}
			for _, m := range shown {
				hudRow(c, s, m, textColor, mutedColor, shadow)
			}
			if overflow > 0 {
				ui.Text(c, fmt.Sprintf("+%d", overflow)).
					FontSize(12).
					TextColor(mutedColor).
					TextShadow(0, 1, 4, shadow)
			}
		})
	}
}

func hudRow(c *ui.Context, s *hudState, m OverlayMember, textColor, mutedColor, shadow ui.Color) {
	speaking := s.speaking[m.identity]
	nameColor := mutedColor
	if speaking {
		nameColor = textColor
	}
	ui.Row(c).Height(hudRowHeight).Gap(hudGap).AlignItems(ui.Center).Children(func() {
		hudAvatarView(c, s, m, speaking)
		icon := micIcon
		iconColor := mutedColor
		if m.IsMicMuted {
			icon = micOffIcon
		} else if speaking {
			iconColor = textColor
		}
		ui.Icon(c, icon).Size(12, 12).TextColor(iconColor)
		name := displayNameOf(m)
		txt := ui.Text(c, name).
			FontSize(12).
			TextColor(nameColor).
			TextShadow(0, 1, 4, shadow).
			MaxWidth(hudWidth - hudPadding*2 - hudAvatar - hudGap*2 - 12).
			NoWrap()
		if speaking {
			txt.Bold()
		}
	})
}

func hudAvatarView(c *ui.Context, s *hudState, m OverlayMember, speaking bool) {
	t := c.Theme()
	box := ui.Box(c).Size(hudAvatar, hudAvatar)
	if speaking {
		box.Border(2, t.Success).Radius(hudAvatar / 2)
	}
	box.Children(func() {
		if s.avatarOf != nil {
			if bmp := s.avatarOf(m.identity); bmp != nil {
				ui.Image(c, bmp).Size(hudAvatar, hudAvatar).Radius(hudAvatar / 2)
				return
			}
		}
		ui.Box(c).
			Size(hudAvatar, hudAvatar).
			Radius(hudAvatar/2).
			Background(t.Accent).
			Center().
			Children(func() {
				ui.Text(c, initialOf(displayNameOf(m))).FontSize(10).TextColor(t.AccentText)
			})
	})
}

func displayNameOf(m OverlayMember) string {
	if m.DisplayName != "" {
		return m.DisplayName
	}
	if m.Name != "" {
		return m.Name
	}
	return m.Identity
}

func initialOf(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "?"
	}
	for _, r := range name {
		return strings.ToUpper(string(r))
	}
	return "?"
}
```

`app/desktop/overlay.go` 里加 `mygo` 已 import；`overlay_view.go` 里的 `mygo` import 若未用到则删掉。

- [ ] **Step 4: 运行测试确认通过**

```bash
cd /Users/noelorin/Projects/GOSpeak/app/desktop
GOTOOLCHAIN=auto go test ./... -run TestHUD -v
```

Expected: PASS（6 个测试）。

- [ ] **Step 5: 窗口切换到原生 Content**

`app/desktop/overlay.go` 的 `createOverlayWindow()` 里，把 `URL` / `Page` 换成 `Content`：

```go
	w := mygo.NewWindow(mygo.WindowOptions{
		Title:         "GOSpeak Overlay " + version,
		Content:       ui.View(hudView(overlayHUD)),
		Panel:         true,
		Frameless:     true,
		DisableClose:  true,
		Transparent:   true,
		DisableShadow: true,
		AlwaysOnTop:   true,
		SkipTaskbar:   true,
		DisableResize: true,
		Opacity:       opacity,
		Width:         hudWidth,
		Height:        hudHeight(0),
		X:             24,
		Y:             24,
		StateKey:      "overlay",
		Hidden:        true,
	})
```

加包级变量（放在 `overlayWin` 旁边）：

```go
var overlayHUD = &hudState{
	speaking: map[string]bool{},
	avatarOf: func(string) *ui.Bitmap { return nil },
}
```

`PushState` 改成更新 HUD 并触发重绘：

```go
func (Overlay) PushState(snapshot OverlaySnapshot) {
	overlayCfg.mu.Lock()
	overlayCfg.snap = snapshot
	overlayCfg.mu.Unlock()

	speaking := make(map[string]bool, len(snapshot.SpeakingIdentities))
	for _, id := range snapshot.SpeakingIdentities {
		speaking[id] = true
	}
	ordered := orderMembers(snapshot.Members, speaking)

	overlayHUD.members = ordered
	overlayHUD.speaking = speaking

	if w := overlayWindow(); w != nil {
		w.SetContentSize(hudWidth, hudHeight(len(ordered)))
		w.Invalidate()
	}
	evaluateOverlayPolicy()
}
```

`orderMembers` 放在 `overlay_view.go`：

```go
func orderMembers(members []OverlayMember, speaking map[string]bool) []OverlayMember {
	out := make([]OverlayMember, len(members))
	copy(out, members)
	sort.SliceStable(out, func(i, j int) bool {
		return speaking[out[i].identity] && !speaking[out[j].identity]
	})
	return out
}
```

（`overlay_view.go` 加 `sort` import。）

- [ ] **Step 6: 前端删除覆盖层页面**

```bash
cd /Users/noelorin/Projects/GOSpeak
git rm app/web/src/pages/overlay.tsx app/web/src/stores/overlayStore.ts
```

`app/web/src/main.tsx` 把麦克风权限判断改回无条件：

```ts
void requestAudioPermission();
```

`app/web/src/hooks/useOverlayRelay.ts` 删掉这两行：

```ts
		// 覆盖层窗口本身停在 /overlay，不应反向推送给自己。
		if (location.pathname === "/overlay") return;
```

并删掉 `useLocation` 的 import 与 `const location = useLocation();`，以及 `}, [location.pathname]);` 的依赖数组改成 `}, []);`。

`app/web/src/layouts/layout.tsx` 的注释改成：

```ts
	// 把主窗口的房间/成员/发言状态接力推给原生覆盖层 HUD。
	useOverlayRelay();
```

- [ ] **Step 7: 重新生成路由树并验证前端**

```bash
cd /Users/noelorin/Projects/GOSpeak
pnpm --filter @gospeak/web build
grep -c "overlay" app/web/src/routeTree.gen.ts || echo "no overlay route (good)"
```

Expected: 构建成功；`routeTree.gen.ts` 不再含 `overlay`。

- [ ] **Step 8: 手动验证原生 HUD**

```bash
GOSPEAK_DESKTOP_DEV=1 pnpm --filter @gospeak/desktop-app dev
```

- 进语音房，HUD 显示成员名（原生绘制，不是网页）
- 说话时名字加粗、头像出现 `Theme.Success` 圆环
- 静音成员显示划掉的麦克风
- 系统切深色/浅色，配色与描影跟着变
- 15 人时显示前 11 行 + `+4`

- [ ] **Step 9: 提交**

```bash
cd /Users/noelorin/Projects/GOSpeak
git add -A
git commit -m "feat(desktop): 覆盖层 HUD 改用 mygo 原生渲染"
```

---

### Task 3: 屏幕自适应缩放

**Files:**
- Create: `app/desktop/overlay_scale.go`
- Create: `app/desktop/overlay_scale_test.go`
- Modify: `app/desktop/overlay.go`
- Modify: `app/desktop/overlay_view.go`

**Interfaces:**
- Consumes: `hudHeight(n int) int`（Task 2）、`hudWidth`（Task 2）
- Produces:
  - `func autoScale(displayWidth int) float64`
  - `func hudScale() float64` — 当前生效的缩放（手动覆盖优先）
  - `func hudWindowSize(n int) (int, int)` — 已乘缩放的窗口 DIPs
  - `OverlayConfig.Scale float64` — `0` = 自动

- [ ] **Step 1: 写缩放的失败测试**

创建 `app/desktop/overlay_scale_test.go`：

```go
package main

import "testing"

func TestAutoScale(t *testing.T) {
	cases := []struct {
		width int
		want  float64
	}{
		{1280, 1.0},
		{1920, 1.0},
		{2560, 1.25},
		{3440, 1.35},
		{3840, 1.5},
		{5120, 1.5},
	}
	for _, c := range cases {
		if got := autoScale(c.width); got != c.want {
			t.Errorf("autoScale(%d) = %v, want %v", c.width, got, c.want)
		}
	}
}

func TestHudWindowSize(t *testing.T) {
	overlayCfg.mu.Lock()
	prevScale := overlayCfg.scale
	overlayCfg.scale = 2
	overlayCfg.mu.Unlock()
	defer func() {
		overlayCfg.mu.Lock()
		overlayCfg.scale = prevScale
		overlayCfg.mu.Unlock()
	}()

	w, h := hudWindowSize(3)
	if w != hudWidth*2 {
		t.Errorf("width = %d, want %d", w, hudWidth*2)
	}
	if h != hudHeight(3)*2 {
		t.Errorf("height = %d, want %d", h, hudHeight(3)*2)
	}
}

func TestHudScaleFallsBackToAuto(t *testing.T) {
	overlayCfg.mu.Lock()
	prevScale := overlayCfg.scale
	overlayCfg.scale = 0
	overlayCfg.mu.Unlock()
	defer func() {
		overlayCfg.mu.Lock()
		overlayCfg.scale = prevScale
		overlayCfg.mu.Unlock()
	}()

	got := hudScale()
	if got < 1.0 || got > 1.5 {
		t.Fatalf("hudScale() = %v, want an automatic value in [1.0, 1.5]", got)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd /Users/noelorin/Projects/GOSpeak/app/desktop
GOTOOLCHAIN=auto go test ./... -run "TestAutoScale|TestHudWindowSize|TestHudScale" -v
```

Expected: FAIL，`undefined: autoScale` / `undefined: hudWindowSize` / `undefined: hudScale` / `overlayCfg.scale` 不存在。

- [ ] **Step 3: 实现缩放**

创建 `app/desktop/overlay_scale.go`：

```go
package main

import "github.com/egoist/mygo"

func autoScale(displayWidth int) float64 {
	switch {
	case displayWidth <= 1920:
		return 1.0
	case displayWidth <= 2560:
		return 1.25
	case displayWidth <= 3440:
		return 1.35
	default:
		return 1.5
	}
}

func hudScale() float64 {
	overlayCfg.mu.Lock()
	manual := overlayCfg.scale
	overlayCfg.mu.Unlock()
	if manual > 0 {
		return manual
	}
	w := 1920
	if win := overlayWindow(); win != nil {
		x, y := win.Position()
		d := mygo.Screen.DisplayNearestPoint(mygo.Point{X: x, Y: y})
		if d.Bounds.Width > 0 {
			w = d.Bounds.Width
		}
	} else if d := mygo.Screen.PrimaryDisplay(); d.Bounds.Width > 0 {
		w = d.Bounds.Width
	}
	return autoScale(w)
}

func hudWindowSize(n int) (int, int) {
	z := hudScale()
	return int(float64(hudWidth)*z + 0.5), int(float64(hudHeight(n))*z + 0.5)
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd /Users/noelorin/Projects/GOSpeak/app/desktop
GOTOOLCHAIN=auto go test ./... -run "TestAutoScale|TestHudWindowSize|TestHudScale" -v
```

Expected: PASS（3 个测试）。

`app/desktop/overlay_view.go` 的 `hudView` 里，在 `t := c.Theme()` 之后加一行：

```go
		c.SetZoom(float32(hudScale()))
```

- [ ] **Step 6: 窗口尺寸改走 hudWindowSize**

`app/desktop/overlay.go` 里所有 `SetContentSize(hudWidth, hudHeight(...))` 改成：

```go
		w, h := hudWindowSize(len(ordered))
		w.SetContentSize(w, h)
```

`createOverlayWindow()` 里的 `Width: hudWidth, Height: hudHeight(0)` 改成：

```go
		Width:  width,
		Height: height,
```

并在函数开头算好：

```go
	width, height := hudWindowSize(0)
```

- [ ] **Step 7: 加配置字段与手动覆盖方法**

`OverlayConfig` 加：

```go
	Scale float64 `json:"scale"`
```

`overlayCfg` 加 `scale float64`（默认 0 = 自动）。

`GetConfig()`、`writeOverlayConfig()`、`loadOverlayConfig()` 都补上 `Scale: overlayCfg.scale`。

新增方法：

```go
func (Overlay) SetScale(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 2 {
		v = 2
	}
	overlayCfg.mu.Lock()
	overlayCfg.scale = v
	members := overlayCfg.snap.Members
	overlayCfg.mu.Unlock()
	scheduleOverlayConfigSave()
	if w := overlayWindow(); w != nil {
		wid, h := hudWindowSize(len(members))
		w.SetContentSize(wid, h)
		w.Invalidate()
	}
}
```

- [ ] **Step 8: 显示器变化时重算**

`initOverlay()` 末尾加：

```go
	mygo.Screen.OnDisplaysChanged(func() {
		if w := overlayWindow(); w != nil {
			overlayCfg.mu.Lock()
			n := len(overlayCfg.snap.Members)
			overlayCfg.mu.Unlock()
			wid, h := hudWindowSize(n)
			w.SetContentSize(wid, h)
			w.Invalidate()
		}
	})
```

- [ ] **Step 9: 补 SDK 类型**

`packages/desktop/src/capabilities/overlay.ts` 的 `OverlayConfig` 加：

```ts
	/** HUD 缩放；0 = 按显示器分辨率自动决定。 */
	scale: number;
```

`Overlay` 接口加：

```ts
	setScale(value: number): Promise<void>;
```

`native` 实现加：

```ts
	setScale: (v) => bridge.invoke(command("Overlay", "SetScale"), v),
```

`webFallback` 加：

```ts
	async setScale() {},
```

`getConfig` 的 fallback 返回值加 `scale: 0,`。

- [ ] **Step 10: 运行全部桌面端测试**

```bash
cd /Users/noelorin/Projects/GOSpeak/app/desktop
GOTOOLCHAIN=auto go vet ./... && GOTOOLCHAIN=auto go test ./... -run "TestHUD|TestPolicyWants|TestAvatar|TestAutoScale|TestHudWindowSize|TestHudScale" -v
```

Expected: vet 无输出；测试全 PASS。

- [ ] **Step 11: 手动验证缩放**

```bash
GOSPEAK_DESKTOP_DEV=1 pnpm --filter @gospeak/desktop-app dev
```

- 在 1080p 屏上 HUD 大小正常
- 把窗口拖到 4K 屏（或改系统缩放），HUD 自动变大
- 设置页手动选 150%，HUD 立即变大且文字依然清晰（不是位图放大）

- [ ] **Step 12: 提交**

```bash
cd /Users/noelorin/Projects/GOSpeak
git add -A
git commit -m "feat(desktop): 覆盖层按显示器分辨率自适应缩放"
```

---

### Task 4: 显示策略三模式

**Files:**
- Create: `app/desktop/overlay_policy.go`
- Create: `app/desktop/overlay_policy_test.go`
- Modify: `app/desktop/overlay.go`
- Modify: `packages/desktop/src/capabilities/overlay.ts`

**Interfaces:**
- Consumes: `OverlaySnapshot.Connected`（Task 1）
- Produces:
  - `const overlayPolicyAlways = "always"`
  - `func policyWants(policy string, connected, appActive bool) bool`

- [ ] **Step 1: 写策略的失败测试**

创建 `app/desktop/overlay_policy_test.go`：

```go
package main

import "testing"

func TestPolicyWants(t *testing.T) {
	cases := []struct {
		policy    string
		connected bool
		appActive bool
		want      bool
	}{
		{overlayPolicyAuto, false, false, false},
		{overlayPolicyAuto, true, true, false},
		{overlayPolicyAuto, true, false, true},
		{overlayPolicyAlways, false, false, false},
		{overlayPolicyAlways, true, true, true},
		{overlayPolicyAlways, true, false, true},
		{overlayPolicyManual, true, false, false},
		{overlayPolicyManual, true, true, false},
	}
	for _, c := range cases {
		if got := policyWants(c.policy, c.connected, c.appActive); got != c.want {
			t.Errorf("policyWants(%q, connected=%v, appActive=%v) = %v, want %v",
				c.policy, c.connected, c.appActive, got, c.want)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd /Users/noelorin/Projects/GOSpeak/app/desktop
GOTOOLCHAIN=auto go test ./... -run TestPolicyWants -v
```

Expected: FAIL，`undefined: policyWants` / `undefined: overlayPolicyAlways`。

- [ ] **Step 3: 实现策略纯函数**

创建 `app/desktop/overlay_policy.go`：

```go
package main

const overlayPolicyAlways = "always"

func policyWants(policy string, connected, appActive bool) bool {
	switch policy {
	case overlayPolicyAuto:
		return connected && !appActive
	case overlayPolicyAlways:
		return connected
	default:
		return false
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd /Users/noelorin/Projects/GOSpeak/app/desktop
GOTOOLCHAIN=auto go test ./... -run TestPolicyWants -v
```

Expected: PASS。

- [ ] **Step 5: 接入真实策略求值**

`app/desktop/overlay.go` 里 `evaluateOverlayPolicy()` 改为：

```go
func evaluateOverlayPolicy() {
	overlayCfg.mu.Lock()
	if !overlayCfg.enabled || overlayCfg.policy == overlayPolicyManual {
		overlayCfg.mu.Unlock()
		return
	}
	policy := overlayCfg.policy
	connected := overlayCfg.snap.Connected
	overlayCfg.mu.Unlock()

	want := policyWants(policy, connected, mygo.App.IsActive())

	overlayCfg.mu.Lock()
	if want != overlayCfg.lastAutoWant {
		overlayCfg.lastAutoWant = want
		overlayCfg.manualOverride = nil
	}
	var target bool
	if overlayCfg.manualOverride != nil {
		target = *overlayCfg.manualOverride
	} else {
		target = want
	}
	overlayCfg.mu.Unlock()
	setOverlayVisible(target)
}
```

`SetPolicy` 的校验加上 `always`：

```go
func (Overlay) SetPolicy(policy string) {
	if policy != overlayPolicyManual && policy != overlayPolicyAuto && policy != overlayPolicyAlways {
		return
	}
	overlayCfg.mu.Lock()
	overlayCfg.policy = policy
	overlayCfg.manualOverride = nil
	overlayCfg.mu.Unlock()
	scheduleOverlayConfigSave()
	if policy == overlayPolicyManual {
		setOverlayVisible(false)
	} else {
		evaluateOverlayPolicy()
	}
}
```

`loadOverlayConfig()` 的政策校验同样加上 `always`。

- [ ] **Step 6: 接线 App 激活事件**

`initOverlay()` 末尾加：

```go
	mygo.App.OnDidBecomeActive(func() { evaluateOverlayPolicy() })
	mygo.App.OnDidResignActive(func() { evaluateOverlayPolicy() })
```

- [ ] **Step 7: 运行全部桌面端测试**

```bash
cd /Users/noelorin/Projects/GOSpeak/app/desktop
GOTOOLCHAIN=auto go vet ./... && GOTOOLCHAIN=auto go test ./... -run "TestHUD|TestPolicyWants" -v
```

Expected: vet 无输出；测试全 PASS。

- [ ] **Step 8: 更新 SDK 类型**

`packages/desktop/src/capabilities/overlay.ts`：

```ts
export type OverlayPolicy = "manual" | "auto" | "always";
```

`OverlayConfig` 加：

```ts
	/** 防录屏：开启后直播/录屏/截图看不到覆盖层（仅 macOS / Windows 生效）。 */
	contentProtection: boolean;
```

`Overlay` 接口加：

```ts
	setContentProtection(value: boolean): Promise<void>;
```

`native` 实现加：

```ts
	setContentProtection: (v) => bridge.invoke(command("Overlay", "SetContentProtection"), v),
```

`webFallback` 加：

```ts
	async setContentProtection() {},
```

`getConfig` 的 fallback 返回值加 `contentProtection: false,`。

删掉 `onState`（接口、native、webFallback 三处），因为没有窗口订阅了。

- [ ] **Step 9: 手动验证策略**

```bash
GOSPEAK_DESKTOP_DEV=1 pnpm --filter @gospeak/desktop-app dev
```

- 策略 `auto`：进语音房 → 切到浏览器 → HUD 出现；切回 GOSpeak → HUD 消失
- 策略 `always`：进语音房 → HUD 一直在（即使 GOSpeak 在前台）
- 策略 `manual`：只有热键能控制

- [ ] **Step 10: 提交**

```bash
cd /Users/noelorin/Projects/GOSpeak
git add -A
git commit -m "feat(desktop): 覆盖层显示策略改为离开 GOSpeak 时显示"
```

---

### Task 5: 头像异步加载

**Files:**
- Create: `app/desktop/overlay_avatar.go`
- Create: `app/desktop/overlay_avatar_test.go`
- Modify: `app/desktop/overlay.go`
- Modify: `app/desktop/server_config.go`

**Interfaces:**
- Consumes: `hudState.avatarOf`（Task 2）、`appPageBase()`（已存在）
- Produces:
  - `type avatarCache struct` 含 `get(identity, url string) *ui.Bitmap`、`clear()`
  - `var overlayAvatars *avatarCache`

- [ ] **Step 1: 写头像加载的失败测试**

创建 `app/desktop/overlay_avatar_test.go`：

```go
package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for x := 0; x < 4; x++ {
		for y := 0; y < 4; y++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestAvatarCacheLoadsAndCaches(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes(t))
	}))
	defer srv.Close()

	c := newAvatarCache()
	url := srv.URL + "/a.png"

	done := make(chan struct{})
	c.get("u1", url, func() { close(done) })
	<-done

	if hits.Load() != 1 {
		t.Fatalf("server hits = %d, want 1", hits.Load())
	}
	if bmp := c.lookup(url); bmp == nil {
		t.Fatal("avatar was not cached after a successful load")
	}

	c.get("u1", url, func() {})
	if hits.Load() != 1 {
		t.Fatalf("server hits after cache hit = %d, want 1", hits.Load())
	}
}

func TestAvatarCacheFailureIsCached(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := newAvatarCache()
	url := srv.URL + "/missing.png"

	done := make(chan struct{})
	c.get("u1", url, func() { close(done) })
	<-done

	c.get("u1", url, func() {})
	if hits.Load() != 1 {
		t.Fatalf("server hits for a failed avatar = %d, want 1 (no retry)", hits.Load())
	}
	if bmp := c.lookup(url); bmp != nil {
		t.Fatal("a failed avatar was cached as a bitmap")
	}
}

func TestAvatarCacheEmptyURLIsSkipped(t *testing.T) {
	c := newAvatarCache()
	called := false
	c.get("u1", "", func() { called = true })
	if called {
		t.Fatal("an empty avatar URL triggered a load")
	}
}

func TestAvatarURLResolution(t *testing.T) {
	base := "http://127.0.0.1:1234"
	if got := resolveAvatarURL(base, "/uploads/a.png"); got != base+"/uploads/a.png" {
		t.Errorf("resolveAvatarURL relative = %q", got)
	}
	if got := resolveAvatarURL(base, "https://cdn.example.com/a.png"); got != "https://cdn.example.com/a.png" {
		t.Errorf("resolveAvatarURL absolute = %q", got)
	}
	if got := resolveAvatarURL(base, ""); got != "" {
		t.Errorf("resolveAvatarURL empty = %q, want empty", got)
	}
}

func TestAvatarCacheClear(t *testing.T) {
	c := newAvatarCache()
	c.failed["http://x/a.png"] = true
	c.clear()
	if len(c.failed) != 0 {
		t.Fatal("clear() did not empty the cache")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd /Users/noelorin/Projects/GOSpeak/app/desktop
GOTOOLCHAIN=auto go test ./... -run TestAvatar -v
```

Expected: FAIL，`undefined: newAvatarCache` / `undefined: resolveAvatarURL`。

- [ ] **Step 3: 实现头像缓存**

创建 `app/desktop/overlay_avatar.go`：

```go
package main

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	avatarCacheLimit = 64
	avatarFetchLimit = 512 * 1024
)

type avatarCache struct {
	mu      sync.Mutex
	images  map[string]*ui.Bitmap
	failed  map[string]bool
	loading map[string]bool
	client  *http.Client
}

func newAvatarCache() *avatarCache {
	return &avatarCache{
		images:  make(map[string]*ui.Bitmap),
		failed:  make(map[string]bool),
		loading: make(map[string]bool),
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

var overlayAvatars = newAvatarCache()

func resolveAvatarURL(base, avatar string) string {
	avatar = strings.TrimSpace(avatar)
	if avatar == "" {
		return ""
	}
	if strings.HasPrefix(avatar, "http://") || strings.HasPrefix(avatar, "https://") {
		return avatar
	}
	if strings.HasPrefix(avatar, "/") {
		return strings.TrimRight(base, "/") + avatar
	}
	return strings.TrimRight(base, "/") + "/" + avatar
}

func (c *avatarCache) lookup(url string) *ui.Bitmap {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.images[url]
}

func (c *avatarCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.images = make(map[string]*ui.Bitmap)
	c.failed = make(map[string]bool)
	c.loading = make(map[string]bool)
}

func (c *avatarCache) get(identity, url string, onLoaded func()) {
	if url == "" {
		return
	}
	c.mu.Lock()
	if _, ok := c.images[url]; ok {
		c.mu.Unlock()
		return
	}
	if c.failed[url] || c.loading[url] {
		c.mu.Unlock()
		return
	}
	c.loading[url] = true
	c.mu.Unlock()

	go func() {
		bmp := c.fetch(url)
		c.mu.Lock()
		delete(c.loading, url)
		if bmp == nil {
			c.failed[url] = true
		} else {
			if len(c.images) >= avatarCacheLimit {
				c.images = make(map[string]*ui.Bitmap)
			}
			c.images[url] = bmp
		}
		c.mu.Unlock()
		if bmp != nil && onLoaded != nil {
			onLoaded()
		}
	}()
}

func (c *avatarCache) fetch(url string) *ui.Bitmap {
	resp, err := c.client.Get(url)
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, avatarFetchLimit))
	if err != nil {
		return nil
	}
	bmp, err := ui.DecodeBitmap(data)
	if err != nil {
		return nil
	}
	return bmp
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd /Users/noelorin/Projects/GOSpeak/app/desktop
GOTOOLCHAIN=auto go test ./... -run TestAvatar -v
```

Expected: PASS（5 个测试）。

- [ ] **Step 5: 接入 HUD**

`app/desktop/overlay.go` 里把 `overlayHUD.avatarOf` 改成真实实现：

```go
var overlayHUD = &hudState{
	speaking: map[string]bool{},
}

func init() {
	overlayHUD.avatarOf = func(identity string) *ui.Bitmap {
		return nil
	}
}
```

`PushState` 里，在更新 `overlayHUD.members` 之后、`Invalidate()` 之前，为每个成员触发头像加载：

```go
	if w := overlayWindow(); w != nil {
		for _, m := range ordered {
			url := resolveAvatarURL(appPageBase(), m.Avatar)
			overlayAvatars.get(m.Identity, url, func() { w.Invalidate() })
		}
	}
```

`avatarOf` 的实现改为按 identity 找成员的 avatar 再查缓存：

```go
func overlayAvatarFor(identity string) *ui.Bitmap {
	overlayCfg.mu.Lock()
	members := overlayCfg.snap.Members
	overlayCfg.mu.Unlock()
	for _, m := range members {
		if m.Identity == identity {
			return overlayAvatars.lookup(resolveAvatarURL(appPageBase(), m.Avatar))
		}
	}
	return nil
}
```

并把 `overlayHUD.avatarOf` 指向它（在 `initOverlay()` 里赋值，避免 `appPageBase()` 在初始化早期被调用）：

```go
	overlayHUD.avatarOf = overlayAvatarFor
```

- [ ] **Step 6: 切换服务器时清空头像缓存**

`app/desktop/server_config.go` 的 `SetBaseURL` 里，在 `reloadAllWindows()` 之前加：

```go
	if newURL != previous {
		overlayAvatars.clear()
	}
```

- [ ] **Step 7: 运行全部测试**

```bash
cd /Users/noelorin/Projects/GOSpeak/app/desktop
GOTOOLCHAIN=auto go vet ./... && GOTOOLCHAIN=auto go test ./... -run "TestHUD|TestPolicyWants|TestAvatar" -v
```

Expected: vet 无输出；测试全 PASS。

- [ ] **Step 8: 手动验证头像**

```bash
GOSPEAK_DESKTOP_DEV=1 pnpm --filter @gospeak/desktop-app dev
```

- 有头像的成员显示圆形头像
- 无头像的成员显示首字 + 主题色圆
- 切换服务器后头像刷新，不残留旧服务器图片

- [ ] **Step 9: 提交**

```bash
cd /Users/noelorin/Projects/GOSpeak
git add -A
git commit -m "feat(desktop): 覆盖层头像异步加载与缓存"
```

---

### Task 6: 设置页、托盘、文档与测试收尾

**Files:**
- Modify: `app/web/src/components/modal/settingModal/tab_item/desktop.tsx`
- Modify: `app/desktop/lifecycle.go`
- Modify: `app/desktop/proxyserver_test.go`
- Modify: `app/desktop/README.md`

**Interfaces:**
- Consumes: `OverlayConfig.contentProtection`（Task 1）、`OverlayPolicy` 含 `always`（Task 3）、`setContentProtection`（Task 3）
- Produces: 无（收尾任务）

- [ ] **Step 1: 设置页策略三选 + 防录屏开关**

`app/web/src/components/modal/settingModal/tab_item/desktop.tsx` 的 `POLICY_OPTIONS` 改成：

```tsx
const POLICY_OPTIONS: { value: OverlayPolicy; label: string; desc: string }[] =
	[
		{
			value: "auto",
			label: "自动（离开 GOSpeak 时显示）",
			desc: "已进入语音房间且 GOSpeak 不在前台时显示；切回 GOSpeak 自动隐藏",
		},
		{
			value: "always",
			label: "始终显示",
			desc: "只要在语音房间里就一直显示，适合把游戏放在另一块屏幕",
		},
		{
			value: "manual",
			label: "手动",
			desc: "只由快捷键或托盘控制显隐",
		},
	];
```

`policy === "auto"` 的说明段落改成：

```tsx
				{cfg?.policy === "auto" && (
					<p className="text-faint-foreground text-xs">
						自动模式：进入语音房间后，切到其他应用时显示，切回 GOSpeak
						自动隐藏；显隐快捷键的最后一次操作优先生效，直到条件再次变化。
					</p>
				)}
```

在「不透明度」之前插入 HUD 大小与防录屏：

```tsx
				<Field
					label="HUD 大小"
					desc="自动会按显示器分辨率选择合适大小"
				>
					<Select
						value={String(cfg?.scale ?? 0)}
						onValueChange={(value) => {
							const v = Number(value);
							patch({ scale: v });
							void desktop.overlay.setScale(v);
						}}
					>
						<SelectTrigger>
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							{SCALE_OPTIONS.map((o) => (
								<SelectItem key={o.value} value={String(o.value)}>
									{o.label}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				</Field>

				<Toggle
					label="防录屏"
					desc="开启后直播、录屏与截图看不到覆盖层；仅 macOS 与 Windows 生效"
					checked={cfg?.contentProtection ?? false}
					onChange={(v) => {
						patch({ contentProtection: v });
						void desktop.overlay.setContentProtection(v);
					}}
				/>
```

同文件的 `POLICY_OPTIONS` 之后加：

```tsx
const SCALE_OPTIONS: { value: number; label: string }[] = [
	{ value: 0, label: "自动（按显示器）" },
	{ value: 0.8, label: "80%" },
	{ value: 1, label: "100%" },
	{ value: 1.25, label: "125%" },
	{ value: 1.5, label: "150%" },
];
```

在覆盖层 `Section` 的末尾（`</Section>` 之前）加提示：

```tsx
				<p className="text-faint-foreground text-xs">
					若覆盖层未显示，请把游戏的显示模式改为「无边框窗口」——
					Windows 的独占全屏无法被任何置顶窗口覆盖。
				</p>
```

- [ ] **Step 2: 托盘菜单加锁定/解锁项**

`app/desktop/lifecycle.go` 的 `trayMenu()` 里，在「隐藏到托盘」之后加：

```go
		{
			Label: "锁定/解锁覆盖层位置",
			Click: func(*mygo.MenuItem, *mygo.Window) { toggleOverlayLock() },
		},
```

- [ ] **Step 3: 修正 proxyserver 测试**

`app/desktop/proxyserver_test.go` 里两处 `/overlay` 改成 `/domain/test`：

```go
	for _, target := range []string{"/", "/domain/test"} {
```

```go
	deep, err := client.Get(ts.URL + "/domain/test")
	if err != nil {
		t.Fatalf("GET /domain/test: %v", err)
	}
	defer func() { _ = deep.Body.Close() }()
	if deep.StatusCode != http.StatusOK {
		t.Fatalf("SPA deep link status = %d, want 200", deep.StatusCode)
	}
```

- [ ] **Step 4: 运行桌面端测试**

```bash
cd /Users/noelorin/Projects/GOSpeak/app/desktop
GOTOOLCHAIN=auto go vet ./...
```

Expected: 无输出。

（`go test ./...` 需要真实前端产物，本地构建后才能跑；CI 只跑 vet。）

- [ ] **Step 5: 更新 README**

`app/desktop/README.md` 目录表里 `overlay.go` 那行改成：

```
| `overlay.go` | 游戏覆盖层窗口：Panel 窗口、原生 HUD、全局快捷键、显示策略、成员状态接力 |
| `overlay_view.go` | 覆盖层 HUD 的原生视图（mygo `ui` 绘制，跟随系统主题） |
| `overlay_avatar.go` | 覆盖层头像的异步加载与缓存 |
| `overlay_policy.go` | 覆盖层显示策略的纯函数求值 |
```

在「性能与启动」一节之后新增一节：

```markdown
## 覆盖层（游戏内浮层）

覆盖层是一个 mygo **Panel 窗口**（`Panel: true`）：浮在所有应用之上、跟随 Space 切换、
能盖住别人的全屏应用、不进 Alt+Tab / Mission Control、不抢键盘焦点。HUD 由 mygo 的
`ui` 包在 GPU 上原生绘制（`WindowOptions.Content`），不加载第二个 WebView。

显示策略三选（设置页 → 桌面端）：

| 策略 | 行为 |
| --- | --- |
| `auto`（默认） | 已进入语音房间且 `mygo.App.IsActive()` 为假（GOSpeak 不在前台）时显示 |
| `always` | 已进入语音房间就一直显示 |
| `manual` | 只由快捷键 / 托盘控制 |

`auto` 完全事件驱动（`OnDidBecomeActive` / `OnDidResignActive` + 每次状态接力），
不做轮询。全屏检测（Win32 `GetForegroundWindow` 那套）已删除——Panel 本身就解决了
盖全屏的问题。

### 屏幕自适应缩放

HUD 尺寸以 DIPs 为基准，通过 `ui.Context.SetZoom(z)` 缩放：内容按 `1/z` 布局、放大
`z` 倍渲染，文字与图标按屏幕分辨率重绘，**不是位图放大**。默认按覆盖层所在显示器的
逻辑宽度自动选 z（≤1920 用 1.0，≤2560 用 1.25，≤3440 用 1.35，更大用 1.5），用户可在
设置页手动覆盖（`OverlayConfig.Scale`，0 = 自动）。

注意窗口尺寸换算：`SetContentSize` 收的是**窗口 DIPs**，是内容 DIPs 的 z 倍，所以
`hudWindowSize(n)` 已把 `hudHeight(n)` 乘上 z。

### 全屏覆盖的边界

Windows 上「全屏」分三种：

| 模式 | DWM | 能否覆盖 |
| --- | --- | --- |
| 无边框窗口 | 合成 | 能 |
| FSO（全屏优化） | 合成 | 能 |
| 真 FSE（legacy flip） | 合成被挂起 | 不能 |

FSO 从 Windows 10 起默认开启，所以玩家在游戏里选「全屏」的绝大多数场景都能覆盖。
盖不住时（用户手动禁用 FSO、老游戏），连 Windows 音量条和 Game Bar 也盖不上去。
设置页会提示用户改用无边框窗口。

Discord / Steam 能盖住真 FSE，是因为它们注入 DLL 并 hook `IDXGISwapChain::Present`。
本项目不做注入：纯 Go 无 cgo 的架构不允许，且有反作弊封号风险。
```

- [ ] **Step 6: 前端类型检查与测试**

```bash
cd /Users/noelorin/Projects/GOSpeak
pnpm typecheck
pnpm lint
pnpm test:web
```

Expected: 全部通过。

- [ ] **Step 7: 提交**

```bash
cd /Users/noelorin/Projects/GOSpeak
git add -A
git commit -m "feat(desktop): 覆盖层设置页、托盘入口与文档"
```

---

## 验证清单

全部任务完成后，在 **macOS 与 Windows 各跑一遍**：

| # | 验证项 | 期望 |
| --- | --- | --- |
| 1 | 进语音房 → 切到别的 App | HUD 出现 |
| 2 | 切回 GOSpeak | HUD 自动隐藏 |
| 3 | 无边框全屏游戏 | HUD 仍可见 |
| 4 | 锁定状态点 HUD 位置 | 游戏收到点击 |
| 5 | 热键解锁 → 拖动 | 位置重启后保留 |
| 6 | 按 Escape | HUD 不消失 |
| 7 | 3 人 vs 15 人 | 窗口高度正确，15 人显示 `+4` |
| 8 | 系统切深色/浅色 | 配色与描影跟着翻转 |
| 9 | 切换服务器 | 头像刷新，无旧图残留 |
| 10 | 开启防录屏 | 截图/录屏中 HUD 消失 |
| 11 | 关掉覆盖层 | 无第二个 WebView 进程 |
| 12 | 1080p 屏 | HUD 大小正常（z=1.0） |
| 13 | 4K 屏 / 高 DPI | HUD 自动变大且文字清晰（不是位图放大） |
| 14 | 拖到另一块分辨率不同的屏 | 缩放自动重算 |
| 15 | 设置页选 150% | HUD 立即变大，覆盖自动值 |

自动化门禁：

```bash
cd app/desktop && GOTOOLCHAIN=auto go vet ./...
cd /Users/noelorin/Projects/GOSpeak && pnpm typecheck && pnpm lint && pnpm test:web
```
