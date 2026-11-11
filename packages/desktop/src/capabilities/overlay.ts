import { bridge, command } from "../bridge";
import { isDesktop } from "../runtime";

/** A single member as shown in the overlay HUD. */
export interface OverlayMember {
	identity: string;
	name: string;
	displayName: string;
	avatar: string;
	/** Mic muted locally or server-side. */
	isMicMuted: boolean;
}

/**
 * Lightweight snapshot the main window pushes to the floating overlay so the
 * overlay never needs its own socket / SFU session (which would double-join the
 * voice room). The host relays it to the overlay window via `mygo.Broadcast`.
 */
export interface OverlaySnapshot {
	roomName: string | null;
	members: OverlayMember[];
	speakingIdentities: string[];
	connected: boolean;
}

/** 覆盖层显示策略。 */
export type OverlayPolicy = "manual" | "auto";

/**
 * 持久化的覆盖层配置（宿主存于用户数据目录 overlay-config.json）。
 * Keep the JSON tags in sync with OverlayConfig in app/desktop/overlay.go.
 */
export interface OverlayConfig {
	enabled: boolean;
	opacity: number;
	/** 显隐全局快捷键（mygo accelerator，如 "Ctrl+Shift+O"）；空串 = 未设置。 */
	hotkey: string;
	/** 锁定/解锁拖拽的全局快捷键；空串 = 未设置。 */
	lockHotkey: string;
	policy: OverlayPolicy;
}

export interface Overlay {
	show(): Promise<void>;
	hide(): Promise<void>;
	setEnabled(value: boolean): Promise<void>;
	isEnabled(): Promise<boolean>;
	setOpacity(value: number): Promise<void>;
	getOpacity(): Promise<number>;
	setHotkey(hotkey: string): Promise<void>;
	setLockHotkey(hotkey: string): Promise<void>;
	setPolicy(policy: OverlayPolicy): Promise<void>;
	setLocked(locked: boolean): Promise<void>;
	getLocked(): Promise<boolean>;
	isFullscreenAppInFront(): Promise<boolean>;
	/** 一次性读回全部持久化配置（设置页 bootstrap）。 */
	getConfig(): Promise<OverlayConfig>;
	isVisible(): Promise<boolean>;
	/** Fires when the overlay is shown/hidden by the host (hotkey, policy...). */
	onVisibilityChange(handler: (visible: boolean) => void): () => void;
	/** Fires when the master switch is toggled (settings UI, host). */
	onEnabledChange(handler: (enabled: boolean) => void): () => void;
	/** Fires when dragging is locked/unlocked (hotkey or settings). */
	onLockChange(handler: (locked: boolean) => void): () => void;

	/** Push a fresh room/member/speaking snapshot to the overlay window. */
	pushState(snapshot: OverlaySnapshot): Promise<void>;
	/** Request the last pushed snapshot (overlay window bootstrap). */
	getState(): Promise<OverlaySnapshot | null>;
	/** Subscribe to relayed snapshots from the main window. */
	onState(handler: (snapshot: OverlaySnapshot) => void): () => void;
}

const noopUnsub = () => {};

const webFallback: Overlay = {
	async show() {},
	async hide() {},
	async setEnabled() {},
	async isEnabled() {
		return false;
	},
	async setOpacity() {},
	async getOpacity() {
		return 1;
	},
	async setHotkey() {},
	async setLockHotkey() {},
	async setPolicy() {},
	async setLocked() {},
	async getLocked() {
		return true;
	},
	async isFullscreenAppInFront() {
		return false;
	},
	async getConfig() {
		return {
			enabled: false,
			opacity: 1,
			hotkey: "",
			lockHotkey: "",
			policy: "manual",
		};
	},
	async isVisible() {
		return false;
	},
	onVisibilityChange() {
		return noopUnsub;
	},
	onEnabledChange() {
		return noopUnsub;
	},
	onLockChange() {
		return noopUnsub;
	},
	async pushState() {},
	async getState() {
		return null;
	},
	onState() {
		return noopUnsub;
	},
};

// 命令名由 command("Overlay", "PushState") 构造，与 app/desktop/overlay.go 中
// MyGo 绑定的结构体与方法名逐字一致。参数是位置参数，顺序对齐 Go 签名。
const native: Overlay = {
	show: () => bridge.invoke(command("Overlay", "Show")),
	hide: () => bridge.invoke(command("Overlay", "Hide")),
	setEnabled: (v) => bridge.invoke(command("Overlay", "SetEnabled"), v),
	isEnabled: () => bridge.invoke<boolean>(command("Overlay", "IsEnabled")),
	setOpacity: (v) => bridge.invoke(command("Overlay", "SetOpacity"), v),
	getOpacity: () => bridge.invoke<number>(command("Overlay", "GetOpacity")),
	setHotkey: (h) => bridge.invoke(command("Overlay", "SetHotkey"), h),
	setLockHotkey: (h) => bridge.invoke(command("Overlay", "SetLockHotkey"), h),
	setPolicy: (p) => bridge.invoke(command("Overlay", "SetPolicy"), p),
	setLocked: (v) => bridge.invoke(command("Overlay", "SetLocked"), v),
	getLocked: () => bridge.invoke<boolean>(command("Overlay", "GetLocked")),
	isFullscreenAppInFront: () =>
		bridge.invoke<boolean>(command("Overlay", "IsFullscreenAppInFront")),
	getConfig: () =>
		bridge.invoke<OverlayConfig>(command("Overlay", "GetConfig")),
	isVisible: () => bridge.invoke<boolean>(command("Overlay", "IsVisible")),
	onVisibilityChange: (handler) =>
		bridge.listen("overlay.visibility", (visible) => handler(Boolean(visible))),
	onEnabledChange: (handler) =>
		bridge.listen("overlay.enabled", (enabled) => handler(Boolean(enabled))),
	onLockChange: (handler) =>
		bridge.listen("overlay.lock", (locked) => handler(Boolean(locked))),
	pushState: (snapshot) =>
		bridge.invoke(command("Overlay", "PushState"), snapshot),
	getState: () =>
		bridge.invoke<OverlaySnapshot | null>(command("Overlay", "GetState")),
	onState: (handler) =>
		bridge.listen("overlay:state", (s) => handler(s as OverlaySnapshot)),
};

/** Returns native overlay controls inside the desktop host, web fallback otherwise. */
export function createOverlay(): Overlay {
	return isDesktop() ? native : webFallback;
}
