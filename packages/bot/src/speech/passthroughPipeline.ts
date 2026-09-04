import type { AudioFrameEvent } from "../media/types";
import type { SpeechPipeline, SpeechResult } from "./types";

export interface PassthroughPipelineOptions {
	/** Emit a mock final after this many frames per speaker (default 5) */
	framesPerFinal?: number;
	/** Optional silence energy threshold for mock finals */
	mockText?: (room: string, speaker: string, frameCount: number) => string;
}

/**
 * Dev/test pipeline: counts frames and emits mock SpeechFinal periodically.
 */
export class PassthroughSpeechPipeline implements SpeechPipeline {
	private listeners = new Set<(r: SpeechResult) => void>();
	private counts = new Map<string, number>(); // room:identity -> frames
	private opts: Required<Pick<PassthroughPipelineOptions, "framesPerFinal">> &
		PassthroughPipelineOptions;

	constructor(opts: PassthroughPipelineOptions = {}) {
		this.opts = { framesPerFinal: opts.framesPerFinal ?? 5, ...opts };
	}

	write(frame: AudioFrameEvent): void {
		const key = `${frame.room}:${frame.identity}`;
		const n = (this.counts.get(key) ?? 0) + 1;
		this.counts.set(key, n);

		// partial every frame
		this.emit({
			room: frame.room,
			speaker: frame.identity,
			text: `…(${n})`,
			isFinal: false,
			provider: "passthrough",
			mediaProvider: frame.mediaProvider,
			timestamp: frame.timestamp || Date.now(),
		});

		if (n % this.opts.framesPerFinal === 0) {
			const text =
				this.opts.mockText?.(frame.room, frame.identity, n) ??
				`[mock speech from ${frame.identity} #${n / this.opts.framesPerFinal}]`;
			this.emit({
				room: frame.room,
				speaker: frame.identity,
				text,
				isFinal: true,
				confidence: 0.5,
				provider: "passthrough",
				mediaProvider: frame.mediaProvider,
				timestamp: frame.timestamp || Date.now(),
			});
		}
	}

	endTrack(room: string, identity: string): void {
		this.counts.delete(`${room}:${identity}`);
	}

	onResult(cb: (result: SpeechResult) => void): () => void {
		this.listeners.add(cb);
		return () => this.listeners.delete(cb);
	}

	dispose(): void {
		this.listeners.clear();
		this.counts.clear();
	}

	private emit(result: SpeechResult): void {
		for (const cb of this.listeners) {
			try {
				cb(result);
			} catch {
				// ignore
			}
		}
	}
}
