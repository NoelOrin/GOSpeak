export type SFUProvider = "livekit" | "agora" | "srs" | "cloudflare";
// MediaSoup/Daily 已禁用保留：client 实现仍在仓库，但不进入运行时类型。

/** 暂时禁用但仍保留代码与类型的 SFU provider。 */
export const DISABLED_SFU_PROVIDERS: readonly SFUProvider[] = ["agora"];

export function isSFUProviderEnabled(provider: SFUProvider): boolean {
	return !DISABLED_SFU_PROVIDERS.includes(provider);
}

export function assertSFUProviderEnabled(provider: SFUProvider): void {
	if (!isSFUProviderEnabled(provider)) {
		throw new Error(`SFU provider "${provider}" is temporarily disabled`);
	}
}

export const DEFAULT_SFU_PROVIDER: SFUProvider = "livekit";

export const PROVIDER_LABELS: Record<SFUProvider, string> = {
	livekit: "LiveKit",
	agora: "Agora",
	srs: "SRS",
	cloudflare: "Cloudflare",
};
