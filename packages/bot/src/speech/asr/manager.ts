import type { AudioFrameEvent } from "../../media/types";
import type { SpeechResult } from "../types";
import type { ASRProvider, ASRSession } from "./types";

/**
 * Per room+identity ASR session manager.
 * Ignores bot self identity to prevent self-feedback loops.
 */
export class ASRManager {
	private sessions = new Map<string, ASRSession>();
	private provider: ASRProvider;
	private botIdentity: string;
	private onResult: (r: SpeechResult) => void;

	constructor(opts: {
		provider: ASRProvider;
		botIdentity: string;
		onResult: (r: SpeechResult) => void;
	}) {
		this.provider = opts.provider;
		this.botIdentity = opts.botIdentity;
		this.onResult = opts.onResult;
	}

	write(frame: AudioFrameEvent): void {
		if (frame.identity === this.botIdentity) return;
		const key = `${frame.room}:${frame.identity}`;
		let session = this.sessions.get(key);
		if (!session) {
			session = this.provider.createSession({
				room: frame.room,
				identity: frame.identity,
				onPartial: this.onResult,
				onFinal: this.onResult,
			});
			this.sessions.set(key, session);
		}
		session.write(frame);
	}

	endTrack(room: string, identity: string): void {
		const key = `${room}:${identity}`;
		const session = this.sessions.get(key);
		if (!session) return;
		session.end();
		this.sessions.delete(key);
	}

	dispose(): void {
		for (const s of this.sessions.values()) s.end();
		this.sessions.clear();
	}

	get size(): number {
		return this.sessions.size;
	}
}
