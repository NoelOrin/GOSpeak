import type { SFUProvider } from "@gospeak/sfu-client/types";
import { type JoinTokenResponse, resolveSFUProvider } from "@/api/sfu";

export function resolveJoinSession(data: JoinTokenResponse): {
	provider: SFUProvider;
	connectTarget: string;
} {
	const provider = resolveSFUProvider(data);
	return {
		provider,
		connectTarget: resolveConnectTarget(provider, data),
	};
}

function resolveConnectTarget(
	provider: SFUProvider,
	data: JoinTokenResponse,
): string {
	switch (provider) {
		case "agora":
			return data.appId || "";
		case "mediasoup":
			return data.bridgeUrl || data.serverUrl;
		case "srs":
			return data.whipUrl || "";
		case "daily":
			return data.dailyDomain || data.serverUrl;
		case "livekit":
		default:
			return data.serverUrl;
	}
}
