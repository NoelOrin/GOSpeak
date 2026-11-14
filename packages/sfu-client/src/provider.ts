export type SFUProvider = "livekit";

/** 服务端可能下发的 provider 白名单：网络层数据用作 SFUProvider 前必须过运行时校验 */
export const SFU_PROVIDERS: readonly SFUProvider[] = ["livekit"];

export function isSFUProvider(
	value: string | null | undefined,
): value is SFUProvider {
	return (
		typeof value === "string" &&
		(SFU_PROVIDERS as readonly string[]).includes(value)
	);
}

export function isSFUProviderEnabled(_provider: SFUProvider): boolean {
	return true;
}

export function assertSFUProviderEnabled(_provider: SFUProvider): void {}

export const DEFAULT_SFU_PROVIDER: SFUProvider = "livekit";

export const PROVIDER_LABELS: Record<SFUProvider, string> = {
	livekit: "LiveKit",
};
