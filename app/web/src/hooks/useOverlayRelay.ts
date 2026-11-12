import { useEffect } from "react";
import { inDesktop, createOverlay, type OverlayMember } from "@gospeak/desktop";
import { useSocketStore, selectMembers } from "@/stores/socketStore";
import { useSpeakingStore } from "@/handler_audio/speakingStore";
import type { MemberInfo } from "@/socket/types";

/**
 * 覆盖层状态推送的最小间隔（毫秒）。
 *
 * 发言状态由 LiveKit 的 active-speaker 回调驱动，连同一堆无关的 socket store
 * 变更（延迟、音量、在线状态……），每秒可能触发十几次推送。逐帧（rAF）推送
 * 意味着整份成员快照被反复 JSON 序列化、跨进程传给 Go、再由宿主 Broadcast
 * 到两个窗口——而人眼对发言高亮的感知差异远小于 100ms。
 */
const RELAY_INTERVAL_MS = 100;

function toOverlayMember(m: MemberInfo): OverlayMember {
	return {
		identity: m.identity,
		name: m.name,
		displayName: m.displayName,
		avatar: m.avatar,
		isMicMuted: m.isMicMuted || m.isMuted,
	};
}

/**
 * Relay the live room/member/speaking state to the floating overlay window.
 *
 * Mounted only in the MAIN window (it lives in the app Layout, which the
 * `/overlay` route does not use). It builds a small snapshot from the app's
 * socket/speaking stores and pushes it through the desktop host; the host
 * broadcasts it to the overlay window, which renders it.
 *
 * Costs are bounded on three sides:
 * - **throttled** to one push per {@link RELAY_INTERVAL_MS}, coalescing bursts
 *   instead of pushing once per animation frame;
 * - **skipped while the overlay is off** (the default — the feature is opt-in),
 *   so a machine that never enables it pays no IPC at all;
 * - while it is on but hidden the relay keeps running, so showing the overlay
 *   (hotkey, auto policy) paints real content instead of a stale frame; showing
 *   it triggers an immediate catch-up push as well.
 *
 * Pushes are de-duplicated by JSON key, so a throttle window with no real
 * change sends nothing at all.
 *
 * No-op in a plain browser (isDesktop() is false), preserving platform
 * isolation.
 */
export function useOverlayRelay() {
	useEffect(() => {
		if (!inDesktop()) return; // Web 构建且无宿主：无需推送（桌面构建此分支被折叠剔除）

		const overlay = createOverlay();
		let timer: ReturnType<typeof setTimeout> | null = null;
		let lastKey = "";
		let enabled = false;
		let visible = false;
		let disposed = false;
		// The boot queries below are only fallbacks: once an event has arrived it
		// is the newer truth, so a slow IPC reply must not overwrite it.
		let sawEnabledEvent = false;
		let sawVisibilityEvent = false;

		// 覆盖层关掉时不推送；开着时即使窗口隐藏也继续同步，保证弹出即是最新画面。
		const relaying = () => enabled || visible;

		const push = () => {
			timer = null;
			if (disposed || !relaying()) return;
			const st = useSocketStore.getState();
			const snapshot = {
				roomName: st.selectedRoomInfo?.name ?? st.currentRoom ?? null,
				members: selectMembers(st).map(toOverlayMember),
				speakingIdentities: useSpeakingStore.getState().speakingIdentities,
				connected: st.connected,
			};
			const key = JSON.stringify(snapshot);
			if (key === lastKey) return;
			lastKey = key;
			void overlay.pushState(snapshot);
		};

		// 尾部合并：窗口内的多次变更只留最后一次推送。
		const schedule = () => {
			if (!relaying() || timer !== null) return;
			timer = setTimeout(push, RELAY_INTERVAL_MS);
		};

		const pushNow = () => {
			if (timer !== null) {
				clearTimeout(timer);
				timer = null;
			}
			push();
		};

		const unsubSocket = useSocketStore.subscribe(schedule);
		const unsubSpeaking = useSpeakingStore.subscribe(schedule);

		const unsubEnabled = overlay.onEnabledChange((next) => {
			sawEnabledEvent = true;
			enabled = next;
			if (next) pushNow();
		});

		// 显示的一瞬间立即补推，覆盖层不必等下一次 store 变更。
		const unsubVisibility = overlay.onVisibilityChange((next) => {
			sawVisibilityEvent = true;
			const wasVisible = visible;
			visible = next;
			if (next && !wasVisible) {
				pushNow();
			} else if (!next && timer !== null) {
				clearTimeout(timer);
				timer = null;
			}
		});

		// 挂载时读取当前开关 / 可见性（宿主可能在页面加载期间就被快捷键唤起）。
		// 桥接失败时保持安全默认（不推送），但必须吞掉 rejection：否则启动期
		// 会冒出 unhandled rejection，且 relaying() 永远为 false，整个会话静默失效。
		void overlay
			.getConfig()
			.then((cfg) => {
				if (disposed || sawEnabledEvent || !cfg.enabled) return;
				enabled = true;
				schedule();
			})
			.catch(() => {});
		void overlay
			.isVisible()
			.then((v) => {
				if (disposed || sawVisibilityEvent || !v || visible) return;
				visible = true;
				pushNow();
			})
			.catch(() => {});

		return () => {
			disposed = true;
			unsubSocket();
			unsubSpeaking();
			unsubEnabled();
			unsubVisibility();
			if (timer !== null) clearTimeout(timer);
		};
		// 挂载一次即可：这个 hook 只存在于 (app) Layout，整个会话生命周期内不卸载；
		// 若跟随 pathname 重建，每次导航都会重置 lastKey 并重跑两条启动 IPC。
	}, []);
}
