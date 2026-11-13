package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// OverlayMember is the per-member payload relayed to the overlay HUD. Keep the
// JSON tags in sync with OverlayMember in packages/desktop/src/capabilities/overlay.ts.
type OverlayMember struct {
	Identity    string `json:"identity"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Avatar      string `json:"avatar"`
	IsMicMuted  bool   `json:"isMicMuted"`
}

// OverlaySnapshot is the lightweight room state the main window pushes to the
// floating overlay so the overlay never opens its own socket / SFU session
// (which would double-join the voice room). Keep the JSON tags in sync with
// OverlaySnapshot in packages/desktop/src/capabilities/overlay.ts.
type OverlaySnapshot struct {
	RoomName           string          `json:"roomName"`
	Members            []OverlayMember `json:"members"`
	SpeakingIdentities []string        `json:"speakingIdentities"`
	Connected          bool            `json:"connected"`
}

// Overlay display policy.
//
//   - "manual": the overlay is only shown/hidden explicitly (hotkey, settings).
//   - "auto":   shown while the main window is connected to a voice room AND
//     GOSpeak is not the active app — you are in a game, a browser or anything
//     else and want to see who talks. The hotkey can still override the
//     decision until the underlying condition flips.
//   - "always": shown while connected, whether GOSpeak is active or not, for a
//     game on another display.
const (
	overlayPolicyManual = "manual"
	overlayPolicyAuto   = "auto"
	overlayPolicyAlways = "always"
)

// OverlayConfig is the persisted, UI-facing overlay configuration. Keep the
// JSON tags in sync with OverlayConfig in packages/desktop/src/capabilities/overlay.ts.
type OverlayConfig struct {
	Enabled    bool    `json:"enabled"`
	Opacity    float64 `json:"opacity"`
	Hotkey     string  `json:"hotkey"`
	LockHotkey string  `json:"lockHotkey"`
	Policy     string  `json:"policy"`
	// ContentProtection keeps the overlay out of screenshots and screen
	// recordings (macOS and Windows; Linux has no implementation).
	ContentProtection bool `json:"contentProtection"`
	// Scale is the HUD zoom; 0 picks one from the display's resolution.
	Scale float64 `json:"scale"`
}

// overlayCfg holds the floating game-overlay window's configuration and the
// last relayed snapshot. The window itself lives in overlayWin (created on
// demand by ensureOverlayWindow, see below).
//
// 默认关闭：enabled=false 且窗口创建时 Hidden，只有用户在设置里开启（或策略
// 判定 + 已连接）后才会显示。窗口本身无边框、透明、无阴影、置顶、不占任务栏。
var overlayCfg = struct {
	enabled           bool
	opacity           float64
	hotkey            string
	lockHotkey        string
	policy            string
	contentProtection bool
	// scale is the user's HUD zoom override; 0 picks the display's.
	scale   float64
	visible bool
	locked  bool
	// manualOverride is the hotkey's explicit show/hide in "auto" policy; it
	// wins over the computed decision until the computed decision flips.
	manualOverride *bool
	// lastAutoWant remembers the last computed auto decision to detect flips.
	lastAutoWant bool
	// currently registered accelerators ("" when none).
	registeredHotkey     string
	registeredLockHotkey string
	snap                 OverlaySnapshot
	mu                   sync.Mutex
}{
	enabled: false,
	opacity: 1.0,
	policy:  overlayPolicyAuto,
	locked:  true,
}

// The floating window is kept out of overlayCfg: it is created on demand (see
// ensureOverlayWindow) and needs its own lock, so that building it never
// serializes against the state broadcast paths.
var (
	overlayWinMu sync.Mutex
	overlayWin   *mygo.Window
)

// overlayHUD is what the native overlay window draws. PushState fills it and
// the window redraws; the HUD never reads the room itself.
var overlayHUD = &hudState{
	speaking: map[string]bool{},
	avatarOf: func(string) *ui.Bitmap { return nil },
	locked:   true,
}

// overlayAvatarFor returns the cached avatar of a member, or nil while it loads
// or when it has none, so the HUD draws the initial instead.
func overlayAvatarFor(identity string) *ui.Bitmap {
	overlayCfg.mu.Lock()
	members := overlayCfg.snap.Members
	overlayCfg.mu.Unlock()
	base := appPageBase()
	for _, m := range members {
		if m.Identity == identity {
			return overlayAvatars.lookup(resolveAvatarURL(base, m.Avatar))
		}
	}
	return nil
}

// overlayWindow returns the floating window, or nil when it has not been built
// yet (the overlay is off until the user enables it).
func overlayWindow() *mygo.Window {
	overlayWinMu.Lock()
	defer overlayWinMu.Unlock()
	return overlayWin
}

// Overlay 事件：类型化 Event[T]（mygo v0.8.3+），Broadcast 发到所有窗口页面，
// 仅主窗口设置 UI 消费；覆盖层由宿主原生绘制，不订阅页面事件。
var (
	overlayVisibilityEvent = mygo.NewEvent[bool]("overlay.visibility")
	overlayLockEvent       = mygo.NewEvent[bool]("overlay.lock")
	// overlayEnabledEvent tells the main window's relay whether the feature is
	// on, so it can keep the overlay's state warm while the window is hidden
	// (instant content when it pops up) and stop pushing entirely when off.
	overlayEnabledEvent = mygo.NewEvent[bool]("overlay.enabled")
)

const overlayConfigFileName = "overlay-config.json"

// Overlay exposes the floating game-overlay (MyGo "floating control":
// frameless + transparent + always-on-top) to the embedded web frontend.
//
// The method names become the typed client generated by `mygo generate`
// (e.g. overlay.show, overlay.pushState). Keep them in sync with
// packages/desktop/src/capabilities/overlay.ts.
type Overlay struct{}

// Show displays the floating overlay window over the game.
func (Overlay) Show() { setOverlayVisible(true) }

// Hide hides the floating overlay window.
func (Overlay) Hide() { setOverlayVisible(false) }

// setOverlayVisible applies a visibility change and notifies the settings UI.
func setOverlayVisible(v bool) {
	overlayCfg.mu.Lock()
	changed := overlayCfg.visible != v
	overlayCfg.visible = v
	overlayCfg.mu.Unlock()
	if !changed {
		return
	}
	if v {
		if w := overlayWindow(); w != nil {
			// ShowInactive, never Show: Show activates the window, which on
			// Windows takes the foreground from the game (and makes the auto
			// policy hide the overlay again) and on macOS takes the keyboard
			// from it.
			w.ShowInactive()
		} else {
			// Nothing to show yet: build the window, whose creation callback
			// shows it because visible is already true.
			ensureOverlayWindow()
		}
	} else if w := overlayWindow(); w != nil {
		w.Hide()
	}
	broadcastVisibility()
}

// SetEnabled turns the overlay feature on/off (master switch, default off).
// Turning it off always hides the window; turning it on hands control to the
// display policy (manual: stay hidden until told; auto/always: decided by the
// policy loop).
func (Overlay) SetEnabled(v bool) {
	overlayCfg.mu.Lock()
	overlayCfg.enabled = v
	overlayCfg.mu.Unlock()
	scheduleOverlayConfigSave()
	// Lets the main window start/stop relaying room state (see overlayEnabledEvent).
	_ = overlayEnabledEvent.Broadcast(v)
	if !v {
		setOverlayVisible(false)
		return
	}
	// Re-evaluate immediately so "auto" reacts without waiting a tick.
	evaluateOverlayPolicy()
	broadcastVisibility()
}

// IsEnabled reports whether the overlay is enabled.
func (Overlay) IsEnabled() bool {
	overlayCfg.mu.Lock()
	defer overlayCfg.mu.Unlock()
	return overlayCfg.enabled
}

// SetOpacity sets the overlay window's global opacity (0..1), applied live.
func (Overlay) SetOpacity(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	overlayCfg.mu.Lock()
	overlayCfg.opacity = v
	overlayCfg.mu.Unlock()
	scheduleOverlayConfigSave()
	if w := overlayWindow(); w != nil {
		w.SetOpacity(v)
	}
}

// GetOpacity returns the configured opacity.
func (Overlay) GetOpacity() float64 {
	overlayCfg.mu.Lock()
	defer overlayCfg.mu.Unlock()
	return overlayCfg.opacity
}

// SetContentProtection keeps the overlay out of screenshots and screen
// recordings, or lets them show it again. macOS and Windows implement it;
// Linux ignores it.
func (Overlay) SetContentProtection(v bool) {
	overlayCfg.mu.Lock()
	overlayCfg.contentProtection = v
	overlayCfg.mu.Unlock()
	scheduleOverlayConfigSave()
	if w := overlayWindow(); w != nil {
		w.SetContentProtection(v)
	}
}

// SetScale sets the HUD zoom, 0 to pick one from the display's resolution.
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
		width, height := hudWindowSize(len(members))
		w.SetContentSize(width, height)
		w.Invalidate()
	}
}

// SetHotkey sets the global keyboard shortcut that toggles the overlay.
// Accepts mygo accelerator syntax, e.g. "Ctrl+Shift+O".
func (Overlay) SetHotkey(hotkey string) {
	overlayCfg.mu.Lock()
	overlayCfg.hotkey = hotkey
	overlayCfg.mu.Unlock()
	scheduleOverlayConfigSave()
	registerOverlayHotkeys()
}

// GetHotkey returns the configured visibility hotkey.
func (Overlay) GetHotkey() string {
	overlayCfg.mu.Lock()
	defer overlayCfg.mu.Unlock()
	return overlayCfg.hotkey
}

// SetLockHotkey sets the global shortcut that locks/unlocks overlay dragging.
// When unlocked the native HUD is a drag region (ui.Element.DragWindow); when
// locked the window ignores mouse events so the game keeps the pointer.
func (Overlay) SetLockHotkey(hotkey string) {
	overlayCfg.mu.Lock()
	overlayCfg.lockHotkey = hotkey
	overlayCfg.mu.Unlock()
	scheduleOverlayConfigSave()
	registerOverlayHotkeys()
}

// GetLockHotkey returns the configured lock/unlock hotkey.
func (Overlay) GetLockHotkey() string {
	overlayCfg.mu.Lock()
	defer overlayCfg.mu.Unlock()
	return overlayCfg.lockHotkey
}

// SetPolicy switches the display policy ("manual" | "auto" | "always").
func (Overlay) SetPolicy(policy string) {
	if policy != overlayPolicyManual && policy != overlayPolicyAuto && policy != overlayPolicyAlways {
		return
	}
	overlayCfg.mu.Lock()
	overlayCfg.policy = policy
	overlayCfg.manualOverride = nil
	overlayCfg.mu.Unlock()
	scheduleOverlayConfigSave()
	// Manual policy never shows on its own; auto re-evaluates right away.
	if policy == overlayPolicyManual {
		setOverlayVisible(false)
	} else {
		evaluateOverlayPolicy()
	}
}

// GetPolicy returns the current display policy.
func (Overlay) GetPolicy() string {
	overlayCfg.mu.Lock()
	defer overlayCfg.mu.Unlock()
	return overlayCfg.policy
}

// SetLocked locks/unlocks overlay dragging (session state, not persisted).
func (Overlay) SetLocked(v bool) { setOverlayLocked(v) }

// GetLocked reports whether dragging is currently locked.
func (Overlay) GetLocked() bool {
	overlayCfg.mu.Lock()
	defer overlayCfg.mu.Unlock()
	return overlayCfg.locked
}

// GetConfig returns the full persisted configuration (settings bootstrap).
func (Overlay) GetConfig() OverlayConfig {
	overlayCfg.mu.Lock()
	defer overlayCfg.mu.Unlock()
	return OverlayConfig{
		Enabled:           overlayCfg.enabled,
		Opacity:           overlayCfg.opacity,
		Hotkey:            overlayCfg.hotkey,
		LockHotkey:        overlayCfg.lockHotkey,
		Policy:            overlayCfg.policy,
		ContentProtection: overlayCfg.contentProtection,
		Scale:             overlayCfg.scale,
	}
}

// IsVisible reports whether the overlay window is currently shown.
func (Overlay) IsVisible() bool {
	overlayCfg.mu.Lock()
	defer overlayCfg.mu.Unlock()
	return overlayCfg.visible
}

// PushState receives a fresh room/member/speaking snapshot from the main window
// and updates the native HUD directly (no second WebView, no page event).
func (Overlay) PushState(snapshot OverlaySnapshot) {
	overlayCfg.mu.Lock()
	overlayCfg.snap = snapshot
	overlayCfg.mu.Unlock()

	speaking := make(map[string]bool, len(snapshot.SpeakingIdentities))
	for _, id := range snapshot.SpeakingIdentities {
		speaking[id] = true
	}
	overlayHUD.setState(snapshot.Members, speaking)

	if w := overlayWindow(); w != nil {
		width, height := hudWindowSize(len(snapshot.Members))
		w.SetContentSize(width, height)
		w.Invalidate()
		// Avatars arrive later and redraw the HUD when they do.
		base := appPageBase()
		for _, m := range snapshot.Members {
			overlayAvatars.get(resolveAvatarURL(base, m.Avatar), w.Invalidate)
		}
	}
	// Connection changes affect the auto policy's decision immediately.
	evaluateOverlayPolicy()
}

// broadcastVisibility notifies the main window (settings UI) of visibility
// changes. Only the main window listens to "overlay.visibility".
func broadcastVisibility() {
	overlayCfg.mu.Lock()
	visible := overlayCfg.visible
	overlayCfg.mu.Unlock()
	_ = overlayVisibilityEvent.Broadcast(visible)
}

// toggleOverlayVisibility is the visibility hotkey action: an explicit user
// intent, allowed in every policy. In the automatic policies it overrides the
// computed decision until that decision flips.
func toggleOverlayVisibility() {
	overlayCfg.mu.Lock()
	target := !overlayCfg.visible
	if overlayCfg.policy != overlayPolicyManual {
		o := target
		overlayCfg.manualOverride = &o
	}
	overlayCfg.mu.Unlock()
	setOverlayVisible(target)
}

// toggleOverlayLock is the lock/unlock hotkey action.
func toggleOverlayLock() {
	overlayCfg.mu.Lock()
	locked := overlayCfg.locked
	overlayCfg.mu.Unlock()
	setOverlayLocked(!locked)
}

// setOverlayLocked applies a lock change: locked lets the pointer through to
// the game (SetIgnoreMouseEvents), unlocked makes the native HUD's root a drag
// handle (ui.Element.DragWindow) so the whole HUD moves the window.
func setOverlayLocked(locked bool) {
	overlayCfg.mu.Lock()
	changed := overlayCfg.locked != locked
	overlayCfg.locked = locked
	overlayCfg.mu.Unlock()
	if !changed {
		return
	}
	overlayHUD.setLocked(locked)
	if w := overlayWindow(); w != nil {
		w.SetIgnoreMouseEvents(locked)
		w.Invalidate()
	}
	_ = overlayLockEvent.Broadcast(locked)
}

// evaluateOverlayPolicy applies the automatic display decision, if any. It
// runs whenever inputs change (enabled/policy/snapshot/app activation).
func evaluateOverlayPolicy() {
	overlayCfg.mu.Lock()
	if !overlayCfg.enabled || overlayCfg.policy == overlayPolicyManual {
		overlayCfg.mu.Unlock()
		return
	}
	policy := overlayCfg.policy
	connected := overlayCfg.snap.Connected
	overlayCfg.mu.Unlock()

	want := policyWants(policy, connected, overlayAppActive())

	overlayCfg.mu.Lock()
	if want != overlayCfg.lastAutoWant {
		// The underlying condition flipped: drop the user's stale override.
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

// overlayAppActive reports whether GOSpeak is the app the user is in. It is a
// variable so tests can answer without a running app: mygo's App.IsActive
// waits for the main loop.
var overlayAppActive = func() bool { return mygo.App.IsActive() }

// overlayPolicyInterval is the fallback poll for "auto". Activation events
// (OnDidBecomeActive / OnDidResignActive) drive it on macOS, but mygo v0.8.3
// fires those only on darwin; on Windows/Linux the foreground app has no event
// source, so a slow ticker keeps "auto" correct without burning CPU.
const overlayPolicyInterval = time.Second

// startOverlayPolicyLoop re-evaluates the automatic policies periodically. It
// is cheap: evaluateOverlayPolicy is a no-op for manual or disabled, and
// setOverlayVisible returns early when the decision is unchanged.
func startOverlayPolicyLoop() {
	go func() {
		ticker := time.NewTicker(overlayPolicyInterval)
		defer ticker.Stop()
		for range ticker.C {
			evaluateOverlayPolicy()
		}
	}()
}

// registerOverlayHotkeys (re)binds the two global shortcuts to the persisted
// accelerators. Must run after the app is ready; failures are logged and
// non-fatal (e.g. the accelerator is taken by another app).
func registerOverlayHotkeys() {
	overlayCfg.mu.Lock()
	hotkey := overlayCfg.hotkey
	lockHotkey := overlayCfg.lockHotkey
	registered := overlayCfg.registeredHotkey
	registeredLock := overlayCfg.registeredLockHotkey
	overlayCfg.mu.Unlock()

	if registered != hotkey {
		if registered != "" {
			mygo.GlobalShortcut.Unregister(registered)
		}
		if hotkey != "" {
			if err := mygo.GlobalShortcut.Register(hotkey, toggleOverlayVisibility); err != nil {
				log.Printf("[GOSpeak] overlay hotkey %q unavailable: %v", hotkey, err)
			}
		}
		overlayCfg.mu.Lock()
		overlayCfg.registeredHotkey = hotkey
		overlayCfg.mu.Unlock()
	}
	if registeredLock != lockHotkey {
		if registeredLock != "" {
			mygo.GlobalShortcut.Unregister(registeredLock)
		}
		if lockHotkey != "" {
			if err := mygo.GlobalShortcut.Register(lockHotkey, toggleOverlayLock); err != nil {
				log.Printf("[GOSpeak] overlay lock hotkey %q unavailable: %v", lockHotkey, err)
			}
		}
		overlayCfg.mu.Lock()
		overlayCfg.registeredLockHotkey = lockHotkey
		overlayCfg.mu.Unlock()
	}
}

// overlayConfigSaveDelay is how long configuration writes are coalesced.
//
// The settings UI drives opacity from a range input: every step of a drag calls
// Overlay.SetOpacity, and writing the JSON file synchronously on each of them
// means dozens of fsync-less-but-still-syscall-heavy writes per drag. A short
// quiet period collapses a drag into one write; flushOverlayConfig covers the
// case where the app quits before it fires.
const overlayConfigSaveDelay = 500 * time.Millisecond

var overlaySaveTimer struct {
	sync.Mutex
	t *time.Timer
}

// scheduleOverlayConfigSave persists the configuration after a quiet period,
// coalescing bursts of changes into a single write.
func scheduleOverlayConfigSave() {
	overlaySaveTimer.Lock()
	defer overlaySaveTimer.Unlock()
	if overlaySaveTimer.t != nil {
		overlaySaveTimer.t.Stop()
	}
	overlaySaveTimer.t = time.AfterFunc(overlayConfigSaveDelay, func() {
		overlaySaveTimer.Lock()
		overlaySaveTimer.t = nil
		overlaySaveTimer.Unlock()
		writeOverlayConfig()
	})
}

// flushOverlayConfig writes a pending configuration immediately. Called on quit
// so a change made right before closing is not lost.
func flushOverlayConfig() {
	overlaySaveTimer.Lock()
	t := overlaySaveTimer.t
	overlaySaveTimer.t = nil
	overlaySaveTimer.Unlock()
	if t != nil && t.Stop() {
		writeOverlayConfig()
	}
}

// writeOverlayConfig persists the current configuration to the user-data dir.
// Callers may hold overlayCfg.mu; this function re-locks as needed.
func writeOverlayConfig() {
	overlayCfg.mu.Lock()
	cfg := OverlayConfig{
		Enabled:           overlayCfg.enabled,
		Opacity:           overlayCfg.opacity,
		Hotkey:            overlayCfg.hotkey,
		LockHotkey:        overlayCfg.lockHotkey,
		Policy:            overlayCfg.policy,
		Scale:             overlayCfg.scale,
		ContentProtection: overlayCfg.contentProtection,
	}
	overlayCfg.mu.Unlock()

	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return
	}
	encoded, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	if err := os.WriteFile(filepath.Join(dir, overlayConfigFileName), encoded, 0o644); err != nil {
		log.Printf("[GOSpeak] failed to save overlay config: %v", err)
	}
}

// loadOverlayConfig reads the persisted configuration (if any) into memory.
// Safe to call once during startup; failures are non-fatal.
func loadOverlayConfig() {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return
	}
	data, err := os.ReadFile(filepath.Join(dir, overlayConfigFileName))
	if err != nil {
		return // no config yet: keep defaults (disabled, auto)
	}
	var cfg OverlayConfig
	if json.Unmarshal(data, &cfg) != nil {
		return
	}
	overlayCfg.mu.Lock()
	overlayCfg.enabled = cfg.Enabled
	if cfg.Opacity >= 0.05 && cfg.Opacity <= 1 {
		overlayCfg.opacity = cfg.Opacity
	}
	overlayCfg.hotkey = cfg.Hotkey
	overlayCfg.lockHotkey = cfg.LockHotkey
	overlayCfg.contentProtection = cfg.ContentProtection
	if cfg.Scale >= 0 && cfg.Scale <= 2 {
		overlayCfg.scale = cfg.Scale
	}
	if cfg.Policy == overlayPolicyManual || cfg.Policy == overlayPolicyAuto || cfg.Policy == overlayPolicyAlways {
		overlayCfg.policy = cfg.Policy
	}
	overlayCfg.mu.Unlock()
}

// createOverlayWindow builds the floating overlay window (always hidden at
// creation; the display policy and hotkeys decide when it shows).
//
// Panel: a window of launchers and quick entries, floating above every app on
// every Space and over apps in full screen, out of the taskbar and the window
// cycle, without activating the app when it shows. That is what makes the
// overlay sit on a borderless full-screen game.
//
// Content: the HUD is drawn by MyGo's own ui package, on the GPU, so the
// overlay costs no second WebView or JavaScript runtime.
//
// Window look: frameless + transparent + DisableShadow — no title bar, no
// opaque background, no system shadow; the content draws the member list
// directly over the game. StateKey persists the user-dragged position.
//
// Main thread only (mygo builds windows there); call it through
// ensureOverlayWindow unless already inside App.WhenReady.
func createOverlayWindow() *mygo.Window {
	overlayCfg.mu.Lock()
	opacity := overlayCfg.opacity
	locked := overlayCfg.locked
	protect := overlayCfg.contentProtection
	overlayCfg.mu.Unlock()

	width, height := hudWindowSize(0)
	w := mygo.NewWindow(mygo.WindowOptions{
		Title:   "GOSpeak Overlay " + version,
		Content: ui.View(hudView(overlayHUD)),
		// Panel 是覆盖层的核心：浮在所有应用之上、跟随 Space、能盖住别人的全屏。
		Panel: true,
		// 无边框 + 透明 + 无阴影：覆盖层不是"一个窗口"，而是浮在游戏上的内容。
		Frameless:     true,
		DisableClose:  true,
		Transparent:   true,
		DisableShadow: true,
		AlwaysOnTop:   true,
		SkipTaskbar:   true,
		DisableResize: true,
		Opacity:       opacity,
		Width:         width,
		Height:        height,
		// X/Y 均非零时不居中（mygo 在 X、Y 同时为零时才自动居中）。
		X:        24,
		Y:        24,
		StateKey: "overlay",
		Hidden:   true,
	})
	// 覆盖层从不销毁，只隐藏：Escape 关闭 NSPanel 的路径必须在这里拦下，
	// DisableClose 管不到它（mygo 的 ShouldClose 不看 Closable）。
	w.OnClose(func(e *mygo.CloseEvent) {
		e.PreventDefault()
		w.Hide()
	})
	w.SetIgnoreMouseEvents(locked)
	w.SetContentProtection(protect)
	return w
}

// ensureOverlayWindow builds the floating window the first time it is needed.
//
// The overlay is off by default, and its window holds a native ui view: creating
// it at launch would add a window and a GPU surface for a feature most users
// never turn on. So it is built on the first real use — the switch being turned
// on, or something showing it — and the creation callback honours a show that
// was requested in the meantime.
func ensureOverlayWindow() {
	overlayWinMu.Lock()
	exists := overlayWin != nil
	overlayWinMu.Unlock()
	if exists {
		return
	}
	// Build inside WhenReady/postMain: window construction must happen on the
	// main thread, and posting it also keeps this safe when called from the
	// policy ticker or an IPC handler.
	mygo.App.WhenReady(func() {
		overlayWinMu.Lock()
		if overlayWin == nil {
			overlayWin = createOverlayWindow()
		}
		w := overlayWin
		overlayWinMu.Unlock()

		overlayCfg.mu.Lock()
		visible := overlayCfg.visible
		overlayCfg.mu.Unlock()
		if visible {
			w.ShowInactive()
		}
	})
}

// initOverlay wires the whole overlay subsystem up during app startup.
func initOverlay() {
	loadOverlayConfig()
	overlayHUD.setAvatarOf(overlayAvatarFor)
	overlayCfg.mu.Lock()
	enabled := overlayCfg.enabled
	locked := overlayCfg.locked
	overlayCfg.mu.Unlock()
	// Keep the native HUD's drag handle in sync with the (locked) session state.
	overlayHUD.setLocked(locked)
	// Only pay for the overlay window when the feature was left enabled; every
	// other path goes through ensureOverlayWindow.
	if enabled {
		ensureOverlayWindow()
	}
	registerOverlayHotkeys()
	// "auto" shows the overlay while GOSpeak is not the active app. Activation
	// events cover macOS; the ticker covers Windows/Linux, where mygo fires no
	// activation event, and any focus change neither source reports.
	mygo.App.OnDidBecomeActive(func() { evaluateOverlayPolicy() })
	mygo.App.OnDidResignActive(func() { evaluateOverlayPolicy() })
	startOverlayPolicyLoop()
	// Moving the overlay to a display of another resolution changes its zoom.
	mygo.Screen.OnDisplaysChanged(func() {
		if w := overlayWindow(); w != nil {
			overlayCfg.mu.Lock()
			n := len(overlayCfg.snap.Members)
			overlayCfg.mu.Unlock()
			width, height := hudWindowSize(n)
			w.SetContentSize(width, height)
			w.Invalidate()
		}
	})
}
