import type { AudioFrameEvent } from "../../media/types";
import type { SpeechResult } from "../types";
import type { ASRProvider, ASRSession } from "./types";

/**
 * Deepgram-shaped provider stub.
 * Requires GOSPEAK_ASR_DEEPGRAM_KEY for real streaming; otherwise mock finals.
 */
export class DeepgramASRProvider implements ASRProvider {
	readonly name = "deepgram";
	constructor(
		private opts: {
			apiKey?: string;
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
		const nFinal = this.opts.framesPerFinal ?? 10;
		return {
			write: (frame: AudioFrameEvent) => {
				count += 1;
				opts.onPartial?.({
					room: opts.room,
					speaker: opts.identity,
					text: `…dg${count}`,
					isFinal: false,
					provider: this.name,
					mediaProvider: frame.mediaProvider,
					timestamp: frame.timestamp || Date.now(),
				});
				if (count % nFinal === 0) {
					opts.onFinal?.({
						room: opts.room,
						speaker: opts.identity,
						text: this.opts.apiKey
							? `[deepgram stream placeholder #${count / nFinal}]`
							: `[deepgram mock final #${count / nFinal}]`,
						isFinal: true,
						confidence: 0.7,
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
