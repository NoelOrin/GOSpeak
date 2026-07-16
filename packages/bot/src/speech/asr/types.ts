import type { AudioFrameEvent } from "../../media/types";
import type { SpeechResult } from "../types";

export interface ASRSession {
	write(frame: AudioFrameEvent): void;
	end(): void;
}

export interface ASRProvider {
	readonly name: string;
	createSession(opts: {
		room: string;
		identity: string;
		onPartial?: (r: SpeechResult) => void;
		onFinal?: (r: SpeechResult) => void;
	}): ASRSession;
}
