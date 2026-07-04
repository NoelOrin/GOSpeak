export type SFUProvider = "livekit" | "agora" | "mediasoup" | "srs" | "daily";

export const DEFAULT_SFU_PROVIDER: SFUProvider = "livekit";

export const PROVIDER_LABELS: Record<SFUProvider, string> = {
	livekit: "LiveKit",
	agora: "Agora",
	mediasoup: "MediaSoup",
	srs: "SRS",
	daily: "Daily",
};
