/**
 * Historical public entry for creating a frontend media-session client.
 *
 * The exported name remains `createSFUClient` for compatibility, but the intended
 * abstraction is a provider-backed media session client for the web app rather than
 * a generic moderation or SFU control surface.
 */
export { createSFUClient } from "./factory";
export { preloadSFUClient } from "./factory";
export type {
	ProducerReadyInfo,
	RemoteAudioTrackLike,
	RemoteTrackInfo,
	SFUAudioCaptureOptions,
	SFUClient,
	SFUClientOptions,
	SFUPublishAudioOptions,
} from "./types";
export type { SFUProvider } from "./provider";
export { DEFAULT_SFU_PROVIDER, PROVIDER_LABELS } from "./provider";
