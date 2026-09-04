import { describe, expect, it } from "vitest";
import { PcmStreamHub, pcm16ToBuffer } from "./pcmStream";
import type { AudioFrameEvent, PcmStream } from "./types";

function frame(
	partial: Partial<AudioFrameEvent> &
		Pick<AudioFrameEvent, "room" | "identity">,
): AudioFrameEvent {
	return {
		pcm16: new Int16Array([1, -2, 3, -4]),
		sampleRate: 16000,
		channels: 1,
		timestamp: 123,
		mediaProvider: "livekit",
		...partial,
	};
}

describe("PcmStream (in-process interface)", () => {
	it("exposes readable interface for external consumers", () => {
		const hub = new PcmStreamHub();
		const stream: PcmStream = hub;
		const got: string[] = [];

		const unsub = stream.subscribeFiltered({ room: "lobby" }, (f) => {
			got.push(`${f.room}:${f.identity}:${f.pcm16.length}`);
		});

		hub.publish(frame({ room: "lobby", identity: "alice" }));
		hub.publish(frame({ room: "other", identity: "bob" }));
		hub.publish(
			frame({ room: "lobby", identity: "bob", pcm16: new Int16Array(8) }),
		);

		expect(got).toEqual(["lobby:alice:4", "lobby:bob:8"]);
		unsub();
		hub.publish(frame({ room: "lobby", identity: "alice" }));
		expect(got).toHaveLength(2);
	});

	it("open() returns reader with onFrame callback", () => {
		const hub = new PcmStreamHub();
		const reader = hub.open({ room: "r1", identity: "u1" });
		const got: number[] = [];

		reader.onFrame((f) => {
			got.push(f.pcm16.length);
		});

		hub.publish(frame({ room: "r1", identity: "u1" }));
		hub.publish(frame({ room: "r1", identity: "u2" }));
		expect(got).toEqual([4]);

		reader.close();
		hub.publish(frame({ room: "r1", identity: "u1" }));
		expect(got).toEqual([4]);
		expect(reader.closed).toBe(true);
	});

	it("supports async iteration for external read loops", async () => {
		const hub = new PcmStreamHub();
		const reader = hub.open({ room: "lobby" });

		const collected: string[] = [];
		const loop = (async () => {
			for await (const f of reader) {
				collected.push(`${f.identity}:${f.pcm16.length}`);
				if (collected.length >= 2) break;
			}
		})();

		// allow iterator to attach waiter
		await new Promise((r) => setTimeout(r, 10));
		hub.publish(frame({ room: "lobby", identity: "a" }));
		hub.publish(frame({ room: "x", identity: "skip" }));
		hub.publish(
			frame({ room: "lobby", identity: "b", pcm16: new Int16Array(2) }),
		);

		await loop;
		expect(collected).toEqual(["a:4", "b:2"]);
		expect(reader.closed).toBe(true);
	});

	it("isolates subscriber errors", () => {
		const hub = new PcmStreamHub();
		const got: number[] = [];
		hub.subscribe(() => {
			throw new Error("boom");
		});
		hub.subscribe((f) => {
			got.push(f.pcm16.length);
		});
		hub.publish(frame({ room: "r", identity: "i" }));
		expect(got).toEqual([4]);
	});

	it("pcm16ToBuffer produces s16le bytes", () => {
		const samples = new Int16Array([1, -2, 3]);
		const buf = pcm16ToBuffer(samples);
		expect(buf.byteLength).toBe(6);
		expect(buf.readInt16LE(0)).toBe(1);
		expect(buf.readInt16LE(2)).toBe(-2);
		expect(buf.readInt16LE(4)).toBe(3);
	});
});
