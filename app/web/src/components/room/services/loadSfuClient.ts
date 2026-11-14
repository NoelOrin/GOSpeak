import { createSFUClient, preloadSFUClient } from "@gospeak/sfu-client/client";
import {
	isSFUProvider,
	type SFUClient,
	type SFUClientOptions,
	type SFUProvider,
} from "@gospeak/sfu-client/types";
import { toast } from "@gospeak/components";

const STORAGE_LAST_SFU_PROVIDER = "lastSfuProvider";

const providerPreloaders: Record<SFUProvider, () => Promise<void>> = {
	livekit: () => preloadSFUClient("livekit"),
};

const preloadedProviders = new Set<SFUProvider>();

export function rememberSfuProvider(provider: SFUProvider): void {
	localStorage.setItem(STORAGE_LAST_SFU_PROVIDER, provider);
}

export function getRememberedSfuProvider(): SFUProvider | undefined {
	const provider = localStorage.getItem(STORAGE_LAST_SFU_PROVIDER);
	// localStorage 是不可信输入：过白名单运行时校验后再当作 SFUProvider
	return isSFUProvider(provider) ? provider : undefined;
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
		toast.error("新语音后端资源加载失败，请刷新后重试");
		throw error;
	}
}
