/**
 * Minimal playback surface required by the web app after a remote track has
 * already been subscribed.
 *
 * Local mute/volume semantics belong to the web playback layer and are applied
 * through these methods. They are intentionally not modeled as provider/session
 * control methods on `SFUClient`.
 */
export interface RemoteAudioTrackLike {
	attach(): HTMLMediaElement;
	detach(): HTMLMediaElement[];
	/** 音量 0-1（0 静音，1 原音量）。provider 实现内部按需缩放至 SDK 各自范围。 */
	setVolume(volume: number): void;
}

/**
 * Provider-agnostic remote audio track payload delivered to the web playback layer.
 */
export interface RemoteTrackInfo {
	identity: string;
	track: RemoteAudioTrackLike;
}

export interface PeerStream {
	identity: string;
	stream: string;
}

export interface SFUAudioCaptureOptions {
	echoCancellation?: boolean;
	noiseSuppression?: boolean;
	autoGainControl?: boolean;
	voiceIsolation?: boolean;
	sampleRate?: number;
	sampleSize?: number;
	channelCount?: number;
}

export interface SFUPublishAudioOptions {
	maxBitrate?: number;
	dtx?: boolean;
	red?: boolean;
	forceStereo?: boolean;
}

/**
 * Options for establishing a media session.
 *
 * These options are scoped to local audio capture, local publish behavior, and
 * provider-specific signaling bootstrap when required. They do not represent
 * moderation, room administration, local playback mute, or user-level speech
 * restriction policies.
 */
export interface SFUClientOptions {
	audioCapture?: SFUAudioCaptureOptions;
	publishAudio?: SFUPublishAudioOptions;
	/**
	 * Socket.IO socket instance. Required when provider is "mediasoup" for its
	 * custom signaling flow; ignored by LiveKit and Agora.
	 */
	socket?: /** raw socket */ any;
}

export interface JoinParams {
	token: string;
	serverUrl: string;
	identity: string;
	room?: string;
	stream?: string;
	streamToken?: string;
}

/**
 * `SFUClient` is the frontend media-session interface.
 *
 * Despite the historical name, it intentionally covers only the client-side
 * media session lifecycle used by the web app:
 * - join / leave a room media session
 * - enable or disable local microphone publishing
 * - receive remote audio tracks
 * - receive active speaker updates
 * - release client resources
 *
 * It is not a frontend abstraction for moderation, kicking, provider admin APIs,
 * local playback mute, or user-level speech restriction. Those concerns remain in
 * higher-level web modules and backend services.
 */
export interface SFUClient {
	/**
	 * Establishes the media session for the current provider.
	 *
	 * `url` is a historical parameter name. Depending on the provider, it may carry
	 * a server URL, an app ID, or another provider-specific connect target.
	 */
	joinRoom(
	joinRoom(params: JoinParams): Promise<void>;
	/** Stops publishing, disconnects from the media session, and releases provider state. */
	leaveRoom(): Promise<void>;
	/** Enables or disables local microphone publishing for the current client only. */
	setMicEnabled(enabled: boolean): Promise<void>;
	/** Subscribes to active-speaker updates produced by the provider. */
	onActiveSpeakers(cb: (identities: string[]) => void): void;
	/** Subscribes to provider-agnostic remote audio track delivery. */
	onRemoteAudioTrack(cb: (info: RemoteTrackInfo) => void): void;
	/** Subscribes to remote audio track removal notifications. */
	onRemoteAudioTrackRemoved(cb: (identity: string) => void): void;
	/**
	 * 返回当前已订阅、但早于 `onRemoteAudioTrack` 回调注册就已到达的远端音频 track。
	 *
	 * 用于补齐 join 竞态：`joinRoom` 会在回调注册前订阅已存在的远端 track，
	 * 仅靠 `onRemoteAudioTrack` 事件会漏掉它们，导致全局静音/音量对这部分 track 失效。
	 */
	getExistingRemoteAudioTracks(): RemoteTrackInfo[];
	subscribePeers?(peers: PeerStream[]): void;
	unsubscribePeer?(identity: string): void;
	/** Subscribes to unexpected media-session disconnect notifications (not triggered by explicit leave). */
	onDisconnected(cb: () => void): void;
	/** Subscribes to media-session reconnect-start notifications. Optional: providers without transient reconnect never fire it. */
	onReconnecting?(cb: () => void): void;
	/** Subscribes to media-session reconnect-success notifications. Optional: providers without transient reconnect never fire it. */
	onReconnected?(cb: () => void): void;
	/** Returns true if the media session is currently connected and joined. */
	isConnected(): boolean;
	/** Final cleanup hook for any remaining provider resources. */
	destroy(): Promise<void>;
}

export type { SFUProvider } from "./provider";
