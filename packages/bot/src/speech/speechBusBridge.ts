import { EventType, type SpeechEvent } from "../core/types";
import type { SpeechResult } from "./types";

export type SpeechEventEmitter = (event: SpeechEvent) => void | Promise<void>;

/**
 * Converts SpeechPipeline results into bot SpeechEvents for EventBus.
 */
export function createSpeechBusBridge(emit: SpeechEventEmitter) {
	return (result: SpeechResult): void => {
		const event: SpeechEvent = {
			eventType: result.isFinal
				? EventType.OnSpeechFinal
				: EventType.OnSpeechPartial,
			room: result.room,
			speaker: result.speaker,
			text: result.text,
			isFinal: result.isFinal,
			confidence: result.confidence,
			timestamp: result.timestamp,
		};
		void emit(event);
	};
}
