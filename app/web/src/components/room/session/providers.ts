import type { SFUProvider } from "@gospeak/sfu-client/types";
import type { JoinTokenResponse } from "@/api/sfu";
import type { VoiceProviderAdapter } from "./voiceSessionTypes";

function defaultJoinKey(token: JoinTokenResponse): string {
	return `${token.room}::${token.identity}::${token.stream || ""}`;
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
	},
};

const agoraAdapter: VoiceProviderAdapter = {
	provider: "agora",
	resolveConnectTarget: (token) => token.appId || "",
	signalJoinMode: "await",
	serializeJoins: false,
	joinKey: defaultJoinKey,
};

const dailyAdapter: VoiceProviderAdapter = {
	provider: "daily",
	resolveConnectTarget: (token) => token.dailyDomain || token.serverUrl,
	signalJoinMode: "await",
	serializeJoins: false,
	joinKey: defaultJoinKey,
};

const mediasoupAdapter: VoiceProviderAdapter = {
	provider: "mediasoup",
	resolveConnectTarget: (token) => token.bridgeUrl || token.serverUrl,
	signalJoinMode: "await",
	serializeJoins: false,
	joinKey: defaultJoinKey,
};

const ADAPTERS: Record<SFUProvider, VoiceProviderAdapter> = {
	livekit: livekitAdapter,
	srs: srsAdapter,
	agora: agoraAdapter,
	daily: dailyAdapter,
	mediasoup: mediasoupAdapter,
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
