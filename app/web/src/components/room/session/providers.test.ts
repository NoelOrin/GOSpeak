import { describe, expect, it } from "vitest";
import { getVoiceProviderAdapter, resolveConnectTarget } from "./providers";

describe("getVoiceProviderAdapter", () => {
	it("livekit awaits signal and does not serialize joins", () => {
		const livekit = getVoiceProviderAdapter("livekit");
		expect(livekit.provider).toBe("livekit");
		expect(livekit.interactiveAfterMedia).toBeFalsy();
		expect(livekit.signalJoinMode).toBe("await");
		expect(livekit.serializeJoins).toBe(false);
		expect(livekit.afterMediaJoin).toBeUndefined();
	});

	it("resolveConnectTarget returns serverUrl for livekit", () => {
		expect(
			resolveConnectTarget("livekit", {
				token: "t",
				serverUrl: "wss://lk.example",
				room: "r1",
				identity: "alice",
			}),
		).toBe("wss://lk.example");
	});

	it("falls back to livekit adapter for unknown providers", () => {
		const adapter = getVoiceProviderAdapter("unknown" as never);
		expect(adapter.provider).toBe("livekit");
	});
});
