import { describe, expect, it, vi } from "vitest";
import { getVoiceProviderAdapter } from "./providers";

vi.mock("idb-keyval", () => ({
	get: vi.fn(async () => undefined),
	set: vi.fn(async () => {}),
	del: vi.fn(async () => {}),
}));

vi.mock("@/handler_audio", () => ({
	setServerMutedByIdentity: vi.fn(),
}));

describe("getVoiceProviderAdapter", () => {
	it("srs uses background signal + serialize joins (WHIP)", () => {
		const srs = getVoiceProviderAdapter("srs");
		expect(srs.interactiveAfterMedia).toBe(true);
		expect(srs.signalJoinMode).toBe("background");
		expect(srs.serializeJoins).toBe(true);

		const livekit = getVoiceProviderAdapter("livekit");
		expect(livekit.interactiveAfterMedia).toBeFalsy();
		expect(livekit.signalJoinMode).toBe("await");
		expect(livekit.serializeJoins).toBe(false);
	});

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

	it("srs afterMediaJoin applies server mute for muted members", async () => {
		const { setServerMutedByIdentity } = await import("@/handler_audio");
		const adapter = getVoiceProviderAdapter("srs");
		await adapter.afterMediaJoin?.(
			{ subscribePeers: vi.fn() } as any,
			{
				token: "t",
				room: "r1",
				identity: "alice",
				stream: "gs-alice",
			} as any,
			{
				members: [
					{ identity: "alice", stream: "gs-alice", isMuted: true } as any,
					{ identity: "bob", stream: "gs-bob", isMuted: true } as any,
					{ identity: "carol", stream: "gs-carol", isMuted: false } as any,
				],
			},
		);
		expect(setServerMutedByIdentity).toHaveBeenCalledWith("bob", true);
		expect(setServerMutedByIdentity).not.toHaveBeenCalledWith("alice", true);
		expect(setServerMutedByIdentity).not.toHaveBeenCalledWith("carol", true);
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

	it("cloudflare uses background signal and subscribes peers by session stream", async () => {
		const adapter = getVoiceProviderAdapter("cloudflare");
		expect(adapter.interactiveAfterMedia).toBe(true);
		expect(adapter.signalJoinMode).toBe("background");
		const subscribePeers = vi.fn();
		await adapter.afterMediaJoin?.(
			{ subscribePeers } as any,
			{
				token: "t",
				serverUrl: "https://rtc.live.cloudflare.com/v1/apps/x",
				room: "r1",
				identity: "alice",
				stream: "sess-alice",
			},
			{
				members: [
					{ identity: "alice", stream: "sess-alice" } as any,
					{ identity: "bob", stream: "sess-bob" } as any,
				],
			},
		);
		expect(subscribePeers).toHaveBeenCalledWith([
			{ identity: "bob", stream: "sess-bob" },
		]);
	});
});
