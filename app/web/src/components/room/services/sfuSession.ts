import type { SFUProvider } from "@gospeak/sfu-client/types";
import { type JoinTokenResponse, resolveSFUProvider } from "@/api/sfu";
import { resolveConnectTarget } from "../session/providers";

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
