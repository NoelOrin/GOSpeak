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

export type VoiceJoinAck = {
	members?: MemberInfo[];
	room?: string;
	identity?: string;
};

/**
 * media 成功后信令 join 的执行策略：
 * - await: 等信令完成再返回（LiveKit 等）
 * - background: media 成功即可返回，信令后台跑（SRS WHIP 等）
 */
export type VoiceSignalJoinMode = "await" | "background";

export interface VoiceProviderAdapter {
	provider: SFUProvider;
	resolveConnectTarget(token: JoinTokenResponse): string;
	/**
	 * true: media join 完成后即可交互（VoiceChat 可加载）。
	 * 通常与 signalJoinMode=background 一起用。
	 */
	interactiveAfterMedia?: boolean;
	/**
	 * media 后信令 join 模式。默认 await。
	 * SRS 用 background。
	 */
	signalJoinMode?: VoiceSignalJoinMode;
	/**
	 * true: 同 joinKey 串行 media join，避免 effect 重入双 publish。
	 * 默认 false；SRS 开。
	 */
	serializeJoins?: boolean;
	/**
	 * 串行队列 key。默认 room::identity::stream。
	 */
	joinKey?(token: JoinTokenResponse): string;
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
		phase === "ready" || phase === "media_ready" || phase === "reconnecting"
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
