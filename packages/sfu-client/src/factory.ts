import type { SFUProvider } from "./provider";
import type { SFUClient, SFUClientOptions } from "./types";

const providerLoaders: Record<SFUProvider, () => Promise<unknown>> = {
	// MediaSoup/Daily 已禁用保留：对应 client 文件未注册，避免被工厂加载。
	agora: () => import("./agora-client"),
	srs: () => import("./srs-client"),
	livekit: () => import("./livekit-client"),
	cloudflare: () => import("./cloudflare-client"),
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
		case "agora": {
			const { AgoraSFUClient } = (await providerLoaders.agora()) as {
				AgoraSFUClient: new (o?: SFUClientOptions) => SFUClient;
			};
			return new AgoraSFUClient(options);
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
		case "cloudflare": {
			const { CloudflareSFUClient } = (await providerLoaders.cloudflare()) as {
				CloudflareSFUClient: new (o?: SFUClientOptions) => SFUClient;
			};
			return new CloudflareSFUClient(options);
		}
		default: {
			throw new Error(`unknown SFU provider: ${provider}`);
		}
	}
}
