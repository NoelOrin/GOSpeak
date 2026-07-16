import type { SFUPublishAdapter } from "./types";

/**
 * LiveKit publish adapter (MVP).
 * Tracks joined rooms and accepts PCM; real WebRTC track publish requires livekit-client + Node WebRTC.
 */
export class LiveKitPublishAdapter implements SFUPublishAdapter {
	private rooms = new Map<string, { token: string; serverUrl: string }>();
	private logger?: {
		info: (...a: unknown[]) => void;
		warn: (...a: unknown[]) => void;
	};

	constructor(logger?: LiveKitPublishAdapter["logger"]) {
		this.logger = logger;
	}

	async join(params: {
		room: string;
		identity: string;
		token: string;
		serverUrl: string;
	}): Promise<void> {
		this.rooms.set(params.room, {
			token: params.token,
			serverUrl: params.serverUrl,
		});
		this.logger?.info(`[livekit-publish] joined ${params.room}`);
	}

	async publishPcm(
		room: string,
		pcm16: Int16Array,
		sampleRate = 16000,
	): Promise<void> {
		if (!this.rooms.has(room))
			throw new Error(`not joined for publish: ${room}`);
		this.logger?.info(
			`[livekit-publish] publishPcm room=${room} samples=${pcm16.length} rate=${sampleRate}`,
		);
		// Real track publish would encode PCM → WebRTC audio track here.
	}

	async unpublish(room: string): Promise<void> {
		this.logger?.info(`[livekit-publish] unpublish ${room}`);
	}

	async leave(room: string): Promise<void> {
		this.rooms.delete(room);
	}

	async dispose(): Promise<void> {
		this.rooms.clear();
	}
}
