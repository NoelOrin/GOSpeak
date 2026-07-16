import type { AudioFrameEvent } from "../../media/types";
import type { SpeechResult } from "../types";
import type { ASRProvider, ASRSession } from "./types";

/**
 * Local HTTP ASR provider (FunASR etc.).
 * POSTs PCM chunks to GOSPEAK_ASR_URL when configured; otherwise emits mock finals.
 */
export class LocalHttpASRProvider implements ASRProvider {
	readonly name = "local";
	constructor(
		private opts: {
			url?: string;
			framesPerFinal?: number;
		} = {},
	) {}

	createSession(opts: {
		room: string;
		identity: string;
		onPartial?: (r: SpeechResult) => void;
		onFinal?: (r: SpeechResult) => void;
	}): ASRSession {
		let count = 0;
		const framesPerFinal = this.opts.framesPerFinal ?? 8;
		const url = this.opts.url;
		return {
			write: (frame: AudioFrameEvent) => {
				count += 1;
				opts.onPartial?.({
					room: opts.room,
					speaker: opts.identity,
					text: `…${count}`,
					isFinal: false,
					provider: this.name,
					mediaProvider: frame.mediaProvider,
					timestamp: frame.timestamp || Date.now(),
				});
				if (count % framesPerFinal === 0) {
					const text = url
						? `[local-asr pending upload #${count / framesPerFinal}]`
						: `[local mock final #${count / framesPerFinal}]`;
					// Fire-and-forget optional HTTP; do not block audio path
					if (url) {
						void fetch(url, {
							method: "POST",
							headers: { "Content-Type": "application/octet-stream" },
							body: Buffer.from(
								frame.pcm16.buffer,
								frame.pcm16.byteOffset,
								frame.pcm16.byteLength,
							),
						}).catch(() => {});
					}
					opts.onFinal?.({
						room: opts.room,
						speaker: opts.identity,
						text,
						isFinal: true,
						confidence: 0.6,
						provider: this.name,
						mediaProvider: frame.mediaProvider,
						timestamp: frame.timestamp || Date.now(),
					});
				}
			},
			end: () => {
				count = 0;
			},
		};
	}
}
