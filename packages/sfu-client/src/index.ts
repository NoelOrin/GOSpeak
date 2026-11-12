/**
 * Historical public entry for creating a frontend media-session client.
 *
 * The exported name remains `createSFUClient` for compatibility, but the intended
 * abstraction is a provider-backed media session client for the web app rather than
 * a generic moderation or SFU control surface.
 */
export { createSFUClient } from "./factory";
export { preloadSFUClient } from "./factory";
export {
	computeScreenShareLayerEncodings,
	DEFAULT_SCREEN_SHARE_QUALITY,
	normalizeScreenShareQuality,
	SCREEN_SHARE_CAPTURE_RESOLUTION,
	SCREEN_SHARE_QUALITY_ORDER,
	SCREEN_SHARE_QUALITY_PRESETS,
	type ScreenShareCaptureResolution,
	type ScreenShareLayerEncoding,
	type ScreenShareQuality,
	type ScreenShareQualityPreset,
	screenShareQualityLabel,
	screenShareQualityPreset,
	screenShareScaleDown,
} from "./screen-share-quality";
export type {
	RemoteAudioTrackLike,
	RemoteTrackInfo,
	SFUAudioCaptureOptions,
	SFUClient,
	SFUClientOptions,
	SFUPublishAudioOptions,
} from "./types";
export type { SFUProvider } from "./provider";
export {
	DEFAULT_SFU_PROVIDER,
	PROVIDER_LABELS,
} from "./provider";
