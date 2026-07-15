import { PROVIDER_LABELS } from "@gospeak/sfu-client";
import { createSFUClient, preloadSFUClient } from "@gospeak/sfu-client/client";
import type {
	SFUClient,
	SFUClientOptions,
	SFUProvider,
} from "@gospeak/sfu-client/types";
import { showToast } from "solid-notifications";

const STORAGE_LAST_SFU_PROVIDER = "lastSfuProvider";

const providerPreloaders: Record<SFUProvider, () => Promise<void>> = {
	livekit: () => preloadSFUClient("livekit"),
	agora: () => preloadSFUClient("agora"),
	mediasoup: () => preloadSFUClient("mediasoup"),
	srs: () => preloadSFUClient("srs"),
	daily: () => preloadSFUClient("daily"),
	cloudflare: () => preloadSFUClient("cloudflare"),
};

const preloadedProviders = new Set<SFUProvider>();

export function rememberSfuProvider(provider: SFUProvider): void {
	localStorage.setItem(STORAGE_LAST_SFU_PROVIDER, provider);
}

const VALID_PROVIDERS = new Set<string>(Object.keys(PROVIDER_LABELS));

export function getRememberedSfuProvider(): SFUProvider | undefined {
	const provider = localStorage.getItem(STORAGE_LAST_SFU_PROVIDER);
	if (provider && VALID_PROVIDERS.has(provider)) {
		return provider as SFUProvider;
	}
	return undefined;
}

/** 预热 provider chunk；失败抛错（调用方可 catch 后仍继续刷新）。 */
export function preloadSfuClient(provider: SFUProvider): Promise<void> {
	if (preloadedProviders.has(provider)) {
		return Promise.resolve();
	}
	preloadedProviders.add(provider);
	return providerPreloaders[provider]()
		.then(() => undefined)
		.catch((error) => {
			preloadedProviders.delete(provider);
			console.warn("[SFU] preload failed:", provider, error);
			throw error;
		});
}

export async function loadSfuClient(
	provider: SFUProvider,
	options?: SFUClientOptions,
): Promise<SFUClient> {
	rememberSfuProvider(provider);
	try {
		await preloadSfuClient(provider);
		return await createSFUClient(provider, options);
	} catch (error) {
		showToast("新语音后端资源加载失败，请刷新后重试", { type: "error" });
		throw error;
	}
}
