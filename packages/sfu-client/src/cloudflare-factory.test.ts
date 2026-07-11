import { describe, expect, it } from "vitest";
import { createSFUClient } from "./factory";

describe("cloudflare factory", () => {
	it("creates CloudflareSFUClient", async () => {
		const client = await createSFUClient("cloudflare");
		expect(client.constructor.name).toBe("CloudflareSFUClient");
		expect(typeof client.joinRoom).toBe("function");
	});
});
