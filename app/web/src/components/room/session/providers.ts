import type { SFUProvider } from "@gospeak/sfu-client/types";
import type { JoinTokenResponse } from "@/api/sfu";
import type { VoiceProviderAdapter } from "./voiceSessionTypes";

const livekitAdapter: VoiceProviderAdapter = {
	provider: "livekit",
	resolveConnectTarget: (token) => token.serverUrl,
};

const srsAdapter: VoiceProviderAdapter = {
	provider: "srs",
	// WHIP 成功即可加载 VoiceChat，不堵在信令
	interactiveAfterMedia: true,
	resolveConnectTarget: (token) => token.whipUrl || "",
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
};

const dailyAdapter: VoiceProviderAdapter = {
	provider: "daily",
	resolveConnectTarget: (token) => token.dailyDomain || token.serverUrl,
};

const mediasoupAdapter: VoiceProviderAdapter = {
	provider: "mediasoup",
	resolveConnectTarget: (token) => token.bridgeUrl || token.serverUrl,
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
