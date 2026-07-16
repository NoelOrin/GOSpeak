import type { AudioFrameEvent } from "../media/types";

export interface SpeechResult {
	room: string;
	speaker: string;
	text: string;
	isFinal: boolean;
	confidence?: number;
	language?: string;
	provider: string;
	mediaProvider?: string;
	timestamp: number;
}

export interface SpeechPipeline {
	write(frame: AudioFrameEvent): void;
	endTrack(room: string, identity: string): void;
	onResult(cb: (result: SpeechResult) => void): () => void;
	dispose(): void;
}
