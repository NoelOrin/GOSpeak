export type SFUProviderName = "livekit" | "srs" | "agora" | "cloudflare";

export interface SFUListenJoinParams {
	room: string;
	identity: string;
	token: string;
	serverUrl: string;
	provider: SFUProviderName;
	stream?: string;
	streamToken?: string;
	clientInfo?: Record<string, unknown>;
	socket?: unknown;
}

export interface SFUListenAdapter {
	readonly provider: SFUProviderName;
	join(params: SFUListenJoinParams): Promise<void>;
	leave(room: string): Promise<void>;
	onAudioFrame(cb: (frame: import("./types").AudioFrameEvent) => void): void;
	onTrackEnded(cb: (info: { room: string; identity: string }) => void): void;
	listActiveIdentities(room: string): string[];
	dispose(): Promise<void>;
}

export interface ListenRoomChange {
	added: string[];
	removed: string[];
	current: string[];
}
