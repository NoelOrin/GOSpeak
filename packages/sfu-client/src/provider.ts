export type SFUProvider = "livekit";

export function isSFUProviderEnabled(_provider: SFUProvider): boolean {
	return true;
}

export function assertSFUProviderEnabled(_provider: SFUProvider): void {}

export const DEFAULT_SFU_PROVIDER: SFUProvider = "livekit";

export const PROVIDER_LABELS: Record<SFUProvider, string> = {
	livekit: "LiveKit",
};
