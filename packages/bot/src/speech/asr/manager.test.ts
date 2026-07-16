import { describe, expect, it, vi } from "vitest";
import { ASRManager } from "./manager";
import type { ASRProvider } from "./types";

function mockProvider(): ASRProvider {
	const sessions: any[] = [];
	return {
		name: "mock",
		createSession(opts) {
			const s = {
				writes: 0,
				ended: false,
				write() {
					this.writes += 1;
					opts.onFinal?.({
						room: opts.room,
						speaker: opts.identity,
						text: "hi",
						isFinal: true,
						provider: "mock",
						timestamp: Date.now(),
					});
				},
				end() {
					this.ended = true;
				},
			};
			sessions.push(s);
			return s;
		},
		_sessions: sessions,
	} as any;
}

describe("ASRManager", () => {
	it("ignores bot identity and ends sessions", () => {
		const onResult = vi.fn();
		const provider = mockProvider();
		const mgr = new ASRManager({
			provider,
			botIdentity: "bot",
			onResult,
		});
		mgr.write({
			room: "r",
			identity: "bot",
			pcm16: new Int16Array([1]),
			sampleRate: 16000,
			channels: 1,
			timestamp: 1,
			mediaProvider: "livekit",
		});
		expect(mgr.size).toBe(0);

		mgr.write({
			room: "r",
			identity: "alice",
			pcm16: new Int16Array([1]),
			sampleRate: 16000,
			channels: 1,
			timestamp: 1,
			mediaProvider: "livekit",
		});
		expect(mgr.size).toBe(1);
		expect(onResult).toHaveBeenCalled();
		mgr.endTrack("r", "alice");
		expect(mgr.size).toBe(0);
		mgr.dispose();
	});
});
