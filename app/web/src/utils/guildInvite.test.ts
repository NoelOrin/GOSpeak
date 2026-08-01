import { describe, expect, it } from "vitest";
import { extractGuildInviteCode } from "./guildInvite";

describe("extractGuildInviteCode", () => {
	it("accepts a plain invite code", () => {
		expect(extractGuildInviteCode("ABCDEFGH")).toBe("ABCDEFGH");
	});

	it("normalizes lowercase invite codes", () => {
		expect(extractGuildInviteCode("abcdefgh")).toBe("ABCDEFGH");
	});

	it("extracts code from invite link path", () => {
		expect(
			extractGuildInviteCode("https://gospeak.local/invite/g/ABCDEFGH"),
		).toBe("ABCDEFGH");
	});

	it("accepts the invite path without relying on a host", () => {
		expect(extractGuildInviteCode("/invite/g/ABCDEFGH")).toBe("ABCDEFGH");
	});

	it("extracts code from invite link with query", () => {
		expect(
			extractGuildInviteCode(
				"https://gospeak.local/invite/g/ABCDEFGH?from=test",
			),
		).toBe("ABCDEFGH");
	});

	it("rejects malformed and overlong codes", () => {
		expect(extractGuildInviteCode("ABC")).toBeNull();
		expect(extractGuildInviteCode("ABCDEFGH1")).toBeNull();
		expect(extractGuildInviteCode("ABCD-1234")).toBeNull();
	});

	it("returns null for unrelated text", () => {
		expect(extractGuildInviteCode("https://example.com/home")).toBeNull();
	});

	it("returns null for empty text", () => {
		expect(extractGuildInviteCode("  ")).toBeNull();
	});
});
