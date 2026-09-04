import type { SFUClient, SFUProvider } from "@gospeak/sfu-client/types";
import {
	dismissToast,
	removeToast,
	showToast,
	updateToast,
} from "solid-notifications";
import type { VoicePhase } from "./voiceSessionTypes";

const RECONNECTING_TOAST_ID = "voice-session-reconnecting";

export function showReconnectingToast() {
	// 同 id 重复 showToast 会叠多个实例；优先 update，避免 dismiss 只关掉其中一个
	const updated = updateToast({
		id: RECONNECTING_TOAST_ID,
		content: "正在重连...",
		type: "info",
		duration: false,
	});
	if (updated) return;
	showToast("正在重连...", {
		id: RECONNECTING_TOAST_ID,
		type: "info",
		duration: false,
	});
}

export function clearReconnectingToast() {
	// solid-notifications 同 id 可能叠多个实例，dismiss/remove 每次只处理一个
	for (let i = 0; i < 8; i++) {
		try {
			dismissToast({ id: RECONNECTING_TOAST_ID });
		} catch {
			// toast 可能尚未创建或已关闭
		}
		try {
			removeToast({ id: RECONNECTING_TOAST_ID });
		} catch {
			// 没有更多同 id toast
			break;
		}
	}
}

export type JoinState =
	| "idle"
	| "connecting"
	| "joined"
	| "reconnecting"
	| "failed";

export type Session = {
	roomName: string;
	domain_uuid?: string;
	client: SFUClient | null;
	signal: AbortSignal;
	status: VoicePhase;
	provider?: SFUProvider;
	error?: string | null;
};

export function toLegacyJoinState(phase: VoicePhase): JoinState {
	switch (phase) {
		case "ready":
		case "media_ready":
			return "joined";
		case "reconnecting":
			return "reconnecting";
		case "failed":
			return "failed";
		case "idle":
			return "idle";
		default:
			return "connecting";
	}
}
