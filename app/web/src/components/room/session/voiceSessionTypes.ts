import type { SFUClient, SFUProvider } from "@gospeak/sfu-client/types";
import type { JoinTokenResponse } from "@/api/sfu";
import type { MemberInfo } from "@/stores/socketStore";

export type VoicePhase =
	| "idle"
	| "resolving"
	| "loading_sfu"
	| "joining_media"
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
	return phase === "ready" || phase === "reconnecting";
}

export function voicePhaseLabel(phase: VoicePhase): string {
	switch (phase) {
		case "resolving":
			return "准备加入...";
		case "loading_sfu":
			return "加载语音引擎...";
		case "joining_media":
			return "连接媒体...";
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
