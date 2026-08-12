import { describe, expect, it } from "vitest";
import { createSFUClient, preloadSFUClient } from "./factory";
import { isSFUProviderEnabled } from "./provider";

describe("temporarily disabled SFU providers", () => {
	it("marks agora as disabled while keeping other providers enabled", () => {
		expect(isSFUProviderEnabled("agora")).toBe(false);
		expect(isSFUProviderEnabled("livekit")).toBe(true);
		expect(isSFUProviderEnabled("srs")).toBe(true);
		expect(isSFUProviderEnabled("cloudflare")).toBe(true);
	});

	it("preload resolves without loading the agora client chunk", async () => {
		await expect(preloadSFUClient("agora")).resolves.toBeUndefined();
	});

	it("create rejects agora with a disabled error", async () => {
		await expect(createSFUClient("agora")).rejects.toThrow(
			/temporarily disabled/,
		);
	});
});
