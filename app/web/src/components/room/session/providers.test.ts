import { describe, expect, it, vi } from "vitest";
import { getVoiceProviderAdapter } from "./providers";

describe("getVoiceProviderAdapter", () => {
	it("srs connect target uses whipUrl only", () => {
		const adapter = getVoiceProviderAdapter("srs");
		expect(
			adapter.resolveConnectTarget({
				token: "t",
				serverUrl: "http://srs:1985",
				room: "r1",
				identity: "alice",
				whipUrl: "/rtc/v1/whip/",
			}),
		).toBe("/rtc/v1/whip/");
		expect(
			adapter.resolveConnectTarget({
				token: "t",
				serverUrl: "http://srs:1985",
				room: "r1",
				identity: "alice",
			}),
		).toBe("");
	});

	it("srs afterMediaJoin subscribes non-self peers with stream", async () => {
		const adapter = getVoiceProviderAdapter("srs");
		const subscribePeers = vi.fn();
		const client = { subscribePeers } as any;
		await adapter.afterMediaJoin?.(
			client,
			{
				token: "t",
				serverUrl: "/rtc/v1/whip/",
				room: "r1",
				identity: "alice",
				stream: "gs-alice",
			},
			{
				members: [
					{ identity: "alice", stream: "gs-alice" } as any,
					{ identity: "bob", stream: "gs-bob" } as any,
					{ identity: "carol" } as any,
				],
			},
		);
		expect(subscribePeers).toHaveBeenCalledWith([
			{ identity: "bob", stream: "gs-bob" },
		]);
	});

	it("livekit afterMediaJoin is optional/no-op", async () => {
		const adapter = getVoiceProviderAdapter("livekit");
		expect(adapter.afterMediaJoin).toBeUndefined();
		expect(
			adapter.resolveConnectTarget({
				token: "t",
				serverUrl: "wss://lk.example",
				room: "r1",
				identity: "alice",
			}),
		).toBe("wss://lk.example");
	});
});
