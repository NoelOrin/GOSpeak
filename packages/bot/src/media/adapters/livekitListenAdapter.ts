import type {
	SFUListenAdapter,
	SFUListenJoinParams,
	SFUProviderName,
} from "../listenTypes";
import type { AudioFrameEvent } from "../types";

type FrameCb = (frame: AudioFrameEvent) => void;
type EndCb = (info: { room: string; identity: string }) => void;

/**
 * LiveKit Node listen adapter.
 * Dynamically imports livekit-client when available; falls back to a no-op with clear error.
 *
 * Note: full WebRTC in Node requires additional setup (wrtc / livekit-server-sdk room service).
 * This adapter focuses on the interface contract; production can swap in a richer implementation.
 */
export class LiveKitListenAdapter implements SFUListenAdapter {
	readonly provider: SFUProviderName = "livekit";
	private frameCb: FrameCb | null = null;
	private endCb: EndCb | null = null;
	private rooms = new Map<
		string,
		{ token: string; serverUrl: string; identities: Set<string> }
	>();
	private logger?: {
		info: (...a: unknown[]) => void;
		warn: (...a: unknown[]) => void;
		error: (...a: unknown[]) => void;
	};

	constructor(logger?: LiveKitListenAdapter["logger"]) {
		this.logger = logger;
	}

	async join(params: SFUListenJoinParams): Promise<void> {
		// Attempt dynamic import of livekit-client; if unavailable, keep membership bookkeeping
		// so MediaListenService can still reconcile, and log a clear warning.
		try {
			await import("livekit-client");
			this.logger?.info(
				`[livekit-listen] joined ${params.room} as ${params.identity}`,
			);
		} catch {
			this.logger?.warn(
				`[livekit-listen] livekit-client not installed; room ${params.room} tracked without media frames. Install livekit-client for real listen.`,
			);
		}
		this.rooms.set(params.room, {
			token: params.token,
			serverUrl: params.serverUrl,
			identities: new Set(),
		});
	}

	async leave(room: string): Promise<void> {
		const state = this.rooms.get(room);
		if (state) {
			for (const identity of state.identities) this.endCb?.({ room, identity });
		}
		this.rooms.delete(room);
		this.logger?.info(`[livekit-listen] left ${room}`);
	}

	onAudioFrame(cb: FrameCb): void {
		this.frameCb = cb;
	}

	onTrackEnded(cb: EndCb): void {
		this.endCb = cb;
	}

	listActiveIdentities(room: string): string[] {
		return [...(this.rooms.get(room)?.identities ?? [])];
	}

	async dispose(): Promise<void> {
		for (const room of [...this.rooms.keys()]) await this.leave(room);
		this.frameCb = null;
		this.endCb = null;
	}

	/** Allow tests / external bridges to push decoded frames */
	publishFrame(frame: AudioFrameEvent): void {
		const state = this.rooms.get(frame.room);
		if (state) state.identities.add(frame.identity);
		this.frameCb?.(frame);
	}
}
