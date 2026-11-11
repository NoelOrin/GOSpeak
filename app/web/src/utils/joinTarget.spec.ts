import { describe, expect, it } from "vitest";
import { joinTarget } from "./joinTarget";

describe("joinTarget", () => {
	it("prefers domain_uuid for public-domain joins", () => {
		expect(joinTarget("CODE", "d1")).toEqual({ domain_uuid: "d1" });
	});

	it("falls back to the invite code when no domain_uuid is known", () => {
		expect(joinTarget("CODE", "")).toEqual({ code: "CODE" });
	});

	it("returns null when neither is available", () => {
		expect(joinTarget("", "")).toBeNull();
	});
});
