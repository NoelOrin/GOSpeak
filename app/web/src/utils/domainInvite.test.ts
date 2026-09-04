import { describe, expect, it } from "vitest";
import { extractDomainInviteCode } from "./domainInvite";

describe("extractDomainInviteCode", () => {
	it("accepts a plain invite code", () => {
		expect(extractDomainInviteCode("ABCDEFGH")).toBe("ABCDEFGH");
	});

	it("normalizes lowercase invite codes", () => {
		expect(extractDomainInviteCode("abcdefgh")).toBe("ABCDEFGH");
	});

	it("extracts code from invite link path", () => {
		expect(
			extractDomainInviteCode("https://gospeak.local/invite/d/ABCDEFGH"),
		).toBe("ABCDEFGH");
	});

	it("accepts the invite path without relying on a host", () => {
		expect(extractDomainInviteCode("/invite/d/ABCDEFGH")).toBe("ABCDEFGH");
	});

	it("extracts code from invite link with query", () => {
		expect(
			extractDomainInviteCode(
				"https://gospeak.local/invite/d/ABCDEFGH?from=test",
			),
		).toBe("ABCDEFGH");
	});

	it("rejects malformed and overlong codes", () => {
		expect(extractDomainInviteCode("ABC")).toBeNull();
		expect(extractDomainInviteCode("ABCDEFGH1")).toBeNull();
		expect(extractDomainInviteCode("ABCD-1234")).toBeNull();
	});

	it("returns null for unrelated text", () => {
		expect(extractDomainInviteCode("https://example.com/home")).toBeNull();
	});

	it("returns null for empty text", () => {
		expect(extractDomainInviteCode("  ")).toBeNull();
	});
});
