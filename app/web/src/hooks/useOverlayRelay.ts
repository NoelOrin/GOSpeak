import { useEffect } from "react";
import { useLocation } from "@tanstack/react-router";
import { inDesktop, createOverlay, type OverlayMember } from "@gospeak/desktop";
import { useSocketStore, selectMembers } from "@/stores/socketStore";
import { useSpeakingStore } from "@/handler_audio/speakingStore";
import type { MemberInfo } from "@/socket/types";

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
 * broadcasts it to the overlay window, which renders it. Pushing is throttled
 * to one per animation frame and de-duplicated by JSON key.
 *
 * No-op in a plain browser (isDesktop() is false), preserving platform
 * isolation.
 */
export function useOverlayRelay() {
	const location = useLocation();

	useEffect(() => {
		if (!inDesktop()) return; // Web 构建且无宿主：无需推送（桌面构建此分支被折叠剔除）
		// 覆盖层窗口本身停在 /overlay，不应反向推送给自己。
		if (location.pathname === "/overlay") return;

		const overlay = createOverlay();
		let frame = 0;
		let lastKey = "";

		const push = () => {
			if (frame) return;
			frame = requestAnimationFrame(() => {
				frame = 0;
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
			});
		};

		push(); // 首帧立即推送一次，覆盖层打开时即使主窗口状态未变也有内容
		const unsubSocket = useSocketStore.subscribe(push);
		const unsubSpeaking = useSpeakingStore.subscribe(push);

		return () => {
			unsubSocket();
			unsubSpeaking();
			if (frame) cancelAnimationFrame(frame);
		};
	}, [location.pathname]);
}
