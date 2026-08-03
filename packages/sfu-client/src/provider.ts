export type SFUProvider = "livekit" | "agora" | "srs" | "cloudflare";
// MediaSoup/Daily 已禁用保留：client 实现仍在仓库，但不进入运行时类型。

export const DEFAULT_SFU_PROVIDER: SFUProvider = "livekit";

export const PROVIDER_LABELS: Record<SFUProvider, string> = {
	livekit: "LiveKit",
	agora: "Agora",
	srs: "SRS",
	cloudflare: "Cloudflare",
};
