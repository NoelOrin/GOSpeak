import type { SFUClient, SFUProvider } from "@gospeak/sfu-client/types";
import type { JoinTokenResponse } from "@/api/sfu";
import type { MemberInfo } from "@/stores/socketStore";

export type VoicePhase =
	| "idle"
	| "resolving"
	| "loading_sfu"
	| "joining_media"
	| "media_ready"
	| "joining_signal"
	| "ready"
	| "reconnecting"
	| "failed"
	| "leaving";

export type VoiceSessionView = {
	phase: VoicePhase;
	roomName: string | null;
	provider: SFUProvider | undefined;
	client: SFUClient | null;
	error: string | null;
};

export type VoiceJoinAck = {
	members?: MemberInfo[];
	room?: string;
	identity?: string;
};

export interface VoiceProviderAdapter {
	provider: SFUProvider;
	resolveConnectTarget(token: JoinTokenResponse): string;
	/**
	 * true: WHIP/WHEP 等 media join 完成后即可交互（VoiceChat 可加载），
	 * 信令 join 继续后台进行。SRS 用。
	 */
	interactiveAfterMedia?: boolean;
	afterMediaJoin?(
		client: SFUClient,
		token: JoinTokenResponse,
		ack: VoiceJoinAck,
	): void | Promise<void>;
}

export function isVoiceLoading(phase: VoicePhase): boolean {
	return (
		phase === "resolving" ||
		phase === "loading_sfu" ||
		phase === "joining_media" ||
		phase === "joining_signal" ||
		phase === "leaving"
	);
}

export function isVoiceInteractive(phase: VoicePhase): boolean {
	// media_ready: WHIP 已成功，UI 可进 VoiceChat；信令可能仍在 joining_signal
	return (
		phase === "ready" ||
		phase === "media_ready" ||
		phase === "reconnecting"
	);
}

export function voicePhaseLabel(phase: VoicePhase): string {
	switch (phase) {
		case "resolving":
			return "准备加入...";
		case "loading_sfu":
			return "加载语音引擎...";
		case "joining_media":
			return "连接媒体...";
		case "media_ready":
			return "媒体已连接";
		case "joining_signal":
			return "加入房间...";
		case "reconnecting":
			return "正在重连...";
		case "leaving":
			return "正在离开...";
		case "failed":
			return "加入失败";
		case "ready":
			return "已连接";
		default:
			return "";
	}
}
