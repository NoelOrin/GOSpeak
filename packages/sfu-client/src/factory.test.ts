import { describe, expect, it } from "vitest";
import { createSFUClient } from "./factory";
import { DEFAULT_SFU_PROVIDER, PROVIDER_LABELS } from "./provider";

describe("SFU client factory", () => {
	it("exposes known providers", () => {
		expect(Object.keys(PROVIDER_LABELS)).toEqual(["livekit"]);
		expect(DEFAULT_SFU_PROVIDER).toBe("livekit");
	});

	it("create rejects unknown providers", async () => {
		await expect(createSFUClient("mediasoup" as never)).rejects.toThrow(
			/unknown SFU provider/,
		);
	});
});
