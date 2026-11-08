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

const ADAPTERS: Record<SFUProvider, VoiceProviderAdapter> = {
	livekit: livekitAdapter,
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
