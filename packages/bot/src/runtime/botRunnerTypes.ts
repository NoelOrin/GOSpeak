import type { AuthCredentials } from "./authClient";
import type { PermissionLevel } from "../core/types";

export interface BotConfig {
	/** GOSpeak server base URL, e.g. http://localhost:8998 */
	serverUrl: string;
	/** WebSocket endpoint */
	socketUrl: string;
	/** Bot account JWT — if provided, skips login */
	accessToken?: string;
	/** Bot login credentials — used when accessToken is not provided */
	credentials?: AuthCredentials;
	/** Auto-refresh interval (ms). Default: 20 minutes */
	refreshIntervalMs?: number;
	/** Bot identity name */
	identity: string;
	/** Bot display name */
	displayName: string;
	/** Directory to scan for plugin modules */
	pluginDir?: string;
	/** Watch pluginDir for runtime load/reload (default true when pluginDir set) */
	watchPlugins?: boolean;
	/** Auto-join these rooms on start (signaling only) */
	autoJoinRooms?: string[];
	/** Rooms to listen (SFU side) on start */
	listenRooms?: string[];
	/** Enable media listen + speech pipeline (default true when listenRooms set) */
	enableListen?: boolean;
	/** Enable TTS speak / publishPcm (Phase 4) */
	enableSpeak?: boolean;
	/** TTS settings (defaults to Edge neural voices) */
	tts?: {
		provider?: "edge" | "sine";
		voice?: string;
		lang?: string;
		rate?: string;
		pitch?: string;
		volume?: string;
		timeout?: number;
	};
	/** Speech recognition settings (defaults to none instead of mock output) */
	asr?: {
		provider?: "openai-compatible" | "passthrough" | "none";
		apiUrl?: string;
		apiKey?: string;
		model?: string;
		language?: string;
		minSilenceMs?: number;
		maxChunkMs?: number;
		minChunkMs?: number;
		vadThreshold?: number;
	};
	/** Debounce for plugin file changes (ms) */
	pluginWatchDebounceMs?: number;
	/** Per-plugin configuration map, keyed by plugin name */
	pluginConfigs?: Record<string, Record<string, unknown>>;
}

export interface BotStatus {
	connected: boolean;
	pluginCount: number;
	handlerCount: number;
	startedAt: number;
	loggedIn: boolean;
	listenRooms: string[];
}

// Permission rank hierarchy — matches PermissionFilter in filters/permissionFilter.ts
export const PERMISSION_RANK: Record<PermissionLevel, number> = {
	guest: 0,
	member: 1,
	moderator: 2,
	admin: 3,
	owner: 4,
};
