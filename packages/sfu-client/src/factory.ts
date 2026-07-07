import type { SFUProvider } from "./provider";
import type { SFUClient, SFUClientOptions } from "./types";

const providerLoaders: Record<SFUProvider, () => Promise<unknown>> = {
	daily: () => import("./daily-client"),
	agora: () => import("./agora-client"),
	mediasoup: () => import("./mediasoup-client"),
	srs: () => import("./srs-client"),
	livekit: () => import("./livekit-client"),
};

export async function preloadSFUClient(provider: SFUProvider): Promise<void> {
	await (providerLoaders[provider] ?? providerLoaders.livekit)();
}

/**
 * Creates a provider-backed frontend media-session client.
 *
 * The factory intentionally returns only the narrow `SFUClient` lifecycle surface
 * needed by the web app. Moderation and playback semantics remain outside of this
 * package.
 */
export async function createSFUClient(
	provider: SFUProvider,
	options?: SFUClientOptions,
): Promise<SFUClient> {
	switch (provider) {
		case "daily": {
			const { DailySFUClient } = (await providerLoaders.daily()) as {
				DailySFUClient: new (o?: SFUClientOptions) => SFUClient;
			};
			return new DailySFUClient(options);
		}
		case "agora": {
			const { AgoraSFUClient } = (await providerLoaders.agora()) as {
				AgoraSFUClient: new (o?: SFUClientOptions) => SFUClient;
			};
			return new AgoraSFUClient(options);
		}
		case "mediasoup": {
			const { MediaSoupSFUClient } = (await providerLoaders.mediasoup()) as {
				MediaSoupSFUClient: new (o?: SFUClientOptions) => SFUClient;
			};
			return new MediaSoupSFUClient(options);
		}
		case "srs": {
			const { SRSSFUClient } = (await providerLoaders.srs()) as {
				SRSSFUClient: new (o?: SFUClientOptions) => SFUClient;
			};
			return new SRSSFUClient(options);
		}
		case "livekit": {
			const { LiveKitSFUClient } = (await providerLoaders.livekit()) as {
				LiveKitSFUClient: new (o?: SFUClientOptions) => SFUClient;
			};
			return new LiveKitSFUClient(options);
		}
		default: {
			throw new Error(`unknown SFU provider: ${provider}`);
		}
	}
}
