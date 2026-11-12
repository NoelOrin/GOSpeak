import { describe, expect, it } from "vitest";
import { resolveFirstScreen } from "./firstScreen";

describe("resolveFirstScreen", () => {
	it("sends users with no domains to the discovery/onboarding page", () => {
		expect(resolveFirstScreen([], "g-old")).toBeNull();
		expect(resolveFirstScreen([], null)).toBeNull();
	});

	it("returns to the last visited domain when still a member", () => {
		expect(resolveFirstScreen(["g-1", "g-2"], "g-2")).toBe("g-2");
	});

	it("falls back to the first domain when the remembered one is stale", () => {
		expect(resolveFirstScreen(["g-1", "g-2"], "g-removed")).toBe("g-1");
		expect(resolveFirstScreen(["g-1"], null)).toBe("g-1");
	});
});
