import type {
	SFUListenAdapter,
	SFUListenJoinParams,
	SFUProviderName,
} from "../listenTypes";
import type { AudioFrameEvent } from "../types";

/**
 * Test/dev adapter that can inject synthetic PCM frames.
 * Used when LiveKit SDK is unavailable or in unit tests.
 */
export class MockListenAdapter implements SFUListenAdapter {
	readonly provider: SFUProviderName = "livekit";
	private frameCb: ((frame: AudioFrameEvent) => void) | null = null;
	private endCb: ((info: { room: string; identity: string }) => void) | null =
		null;
	private active = new Map<string, Set<string>>(); // room -> identities

	async join(params: SFUListenJoinParams): Promise<void> {
		if (!this.active.has(params.room)) this.active.set(params.room, new Set());
		// no remote identities yet; tests inject frames via injectFrame
	}

	async leave(room: string): Promise<void> {
		const ids = this.active.get(room);
		if (ids) {
			for (const identity of ids) this.endCb?.({ room, identity });
		}
		this.active.delete(room);
	}

	onAudioFrame(cb: (frame: AudioFrameEvent) => void): void {
		this.frameCb = cb;
	}

	onTrackEnded(cb: (info: { room: string; identity: string }) => void): void {
		this.endCb = cb;
	}

	listActiveIdentities(room: string): string[] {
		return [...(this.active.get(room) ?? [])];
	}

	async dispose(): Promise<void> {
		for (const room of [...this.active.keys()]) await this.leave(room);
		this.frameCb = null;
		this.endCb = null;
	}

	/** Test helper: publish a synthetic frame */
	injectFrame(frame: AudioFrameEvent): void {
		if (!this.active.has(frame.room)) this.active.set(frame.room, new Set());
		this.active.get(frame.room)?.add(frame.identity);
		this.frameCb?.(frame);
	}
}
