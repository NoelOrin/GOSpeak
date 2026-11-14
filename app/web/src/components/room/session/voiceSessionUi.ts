import { toast } from "@gospeak/components";
import type { SFUClient, SFUProvider } from "@gospeak/sfu-client/types";
import type { VoicePhase } from "./voiceSessionTypes";

const RECONNECTING_TOAST_ID = "voice-session-reconnecting";

export function showReconnectingToast() {
	toast.loading("正在重连...", {
		id: RECONNECTING_TOAST_ID,
		duration: Infinity,
	});
}

export function clearReconnectingToast() {
	try {
		toast.dismiss(RECONNECTING_TOAST_ID);
	} catch {
		// toast 可能尚未创建或已关闭
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
