import type { SFUClient, SFUProvider } from "@gospeak/sfu-client/types";
import type { JoinTokenResponse } from "@/api/sfu";
import { setServerMutedByIdentity } from "@/handler_audio";
import { socketStore } from "@/stores/socketStore";
import type { VoiceProviderAdapter } from "./voiceSessionTypes";

function defaultJoinKey(token: JoinTokenResponse): string {
	return `${token.room}::${token.identity}::${token.stream || ""}`;
}

// 无 SFU 原生 active speaker 的 provider（SRS / Cloudflare）：
// 本端分析本地麦克风音量（onLocalSpeakingChange）→ 信令层聚合广播房间级 active speakers。
// 采样由 AudioWorkletProcessor 事件驱动（音频线程按块 postMessage，无 JS 定时器），
// 本地滞回只在状态翻转时回调，这里再做同值节流兜底（翻转立即上报，同值重复至少间隔
// 150ms）；服务端另有状态去重。
function bindSignalActiveSpeakers(
	client: SFUClient,
	token: JoinTokenResponse,
): void {
	let lastValue: boolean | null = null;
	let lastSentAt = 0;
	const MIN_SEND_INTERVAL_MS = 150;
	client.onLocalSpeakingChange?.((speaking) => {
		const now = Date.now();
		if (speaking === lastValue && now - lastSentAt < MIN_SEND_INTERVAL_MS) {
			return;
		}
		lastValue = speaking;
		lastSentAt = now;
		socketStore.emitSpeaking(token.room, token.identity, speaking);
	});
}

// 服务器禁言状态同步：以 join ack 快照为权威双向设置。
// 只置位不重置会让断线重连错过 member:unmuted 的成员残留陈旧静音。
function syncServerMuteState(
	ack: { members?: Array<{ identity: string; isMuted?: boolean }> },
	selfIdentity: string,
): void {
	for (const m of ack.members ?? []) {
		if (m.identity === selfIdentity) continue;
		setServerMutedByIdentity(m.identity, Boolean(m.isMuted));
	}
}

// LiveKit：完整 media + signal 后才 ready。不串行、不 background。
const livekitAdapter: VoiceProviderAdapter = {
	provider: "livekit",
	resolveConnectTarget: (token) => token.serverUrl,
	signalJoinMode: "await",
	serializeJoins: false,
	joinKey: defaultJoinKey,
};

// SRS WHIP/WHEP：publish 成功即可交互；同 stream 必须串行。
const srsAdapter: VoiceProviderAdapter = {
	provider: "srs",
	resolveConnectTarget: (token) => token.whipUrl || "",
	interactiveAfterMedia: true,
	signalJoinMode: "background",
	serializeJoins: true,
	joinKey: defaultJoinKey,
	afterMediaJoin(client, token, ack) {
		const peers = (ack.members ?? [])
			.filter((m) => m.identity !== token.identity && m.stream)
			.map((m) => ({ identity: m.identity, stream: m.stream as string }));
		if (peers.length) client.subscribePeers?.(peers);
		syncServerMuteState(ack, token.identity);
		bindSignalActiveSpeakers(client, token);
	},
};

const agoraAdapter: VoiceProviderAdapter = {
	provider: "agora",
	resolveConnectTarget: (token) => token.appId || "",
	signalJoinMode: "await",
	serializeJoins: false,
	joinKey: defaultJoinKey,
};

// Cloudflare Realtime：本端 publish 成功即可交互；stream 字段承载 sessionId，用于拉远端 track。
const cloudflareAdapter: VoiceProviderAdapter = {
	provider: "cloudflare",
	resolveConnectTarget: (token) => token.serverUrl,
	interactiveAfterMedia: true,
	signalJoinMode: "background",
	serializeJoins: true,
	joinKey: defaultJoinKey,
	afterMediaJoin(client, token, ack) {
		const peers = (ack.members ?? [])
			.filter((m) => m.identity !== token.identity && m.stream)
			.map((m) => ({ identity: m.identity, stream: m.stream as string }));
		if (peers.length) client.subscribePeers?.(peers);
		syncServerMuteState(ack, token.identity);
		bindSignalActiveSpeakers(client, token);
	},
};

const ADAPTERS: Record<SFUProvider, VoiceProviderAdapter> = {
	livekit: livekitAdapter,
	srs: srsAdapter,
	agora: agoraAdapter,
	cloudflare: cloudflareAdapter,
};

export function getVoiceProviderAdapter(
	provider: SFUProvider,
): VoiceProviderAdapter {
	return ADAPTERS[provider] ?? livekitAdapter;
}

export function resolveConnectTarget(
	provider: SFUProvider,
	token: JoinTokenResponse,
): string {
	return getVoiceProviderAdapter(provider).resolveConnectTarget(token);
}
