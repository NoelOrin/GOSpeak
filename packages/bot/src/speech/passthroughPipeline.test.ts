import { describe, expect, it, vi } from "vitest";
import { EventType } from "../core/types";
import { PassthroughSpeechPipeline } from "./passthroughPipeline";
import { createSpeechBusBridge } from "./speechBusBridge";

describe("PassthroughSpeechPipeline", () => {
	it("emits partials and finals", () => {
		const pipe = new PassthroughSpeechPipeline({ framesPerFinal: 2 });
		const results: any[] = [];
		pipe.onResult((r) => results.push(r));
		const frame = {
			room: "r",
			identity: "u",
			pcm16: new Int16Array([1]),
			sampleRate: 16000,
			channels: 1,
			timestamp: 1,
			mediaProvider: "livekit",
		};
		pipe.write(frame as any);
		pipe.write(frame as any);
		expect(results.some((r) => !r.isFinal)).toBe(true);
		expect(results.some((r) => r.isFinal)).toBe(true);
	});
});

describe("createSpeechBusBridge", () => {
	it("maps final/partial to EventType", () => {
		const emit = vi.fn();
		const bridge = createSpeechBusBridge(emit);
		bridge({
			room: "r",
			speaker: "u",
			text: "hi",
			isFinal: true,
			provider: "passthrough",
			timestamp: 1,
		});
		expect(emit.mock.calls[0][0].eventType).toBe(EventType.OnSpeechFinal);
	});
});
