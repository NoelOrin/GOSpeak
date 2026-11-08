import { assertSFUProviderEnabled, type SFUProvider } from "./provider";
import type { SFUClient, SFUClientOptions } from "./types";

const providerLoaders: Record<SFUProvider, () => Promise<unknown>> = {
	livekit: () => import("./livekit-client"),
};

export async function preloadSFUClient(provider: SFUProvider): Promise<void> {
	assertSFUProviderEnabled(provider);
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
	assertSFUProviderEnabled(provider);
	switch (provider) {
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
