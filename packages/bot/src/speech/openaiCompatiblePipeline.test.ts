import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { OpenAICompatibleSpeechPipeline } from "./openaiCompatiblePipeline";

function frame(identity = "alice", values: number[] = [400, 400, 400]) {
	return {
		room: "room-a",
		identity,
		pcm16: new Int16Array(values),
		sampleRate: 48000,
		channels: 1,
		timestamp: Date.now(),
		mediaProvider: "test",
	};
}

describe("OpenAICompatibleSpeechPipeline idle cleanup", () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});
	afterEach(() => {
		vi.useRealTimers();
	});

	it("drops buffers idle for more than 30s", () => {
		const transcribe = vi.fn().mockResolvedValue({ text: "hi" });
		const pipeline = new OpenAICompatibleSpeechPipeline({
			apiUrl: "",
			transcribe,
		});

		pipeline.write(frame());
		vi.advanceTimersByTime(31_000);
		pipeline.write(frame());

		const buffers = (pipeline as any).buffers as Map<
			string,
			{ frames: unknown[] }
		>;
		expect(buffers.size).toBe(1);
		expect(buffers.get("room-a:alice")?.frames.length).toBe(1);
	});
});
