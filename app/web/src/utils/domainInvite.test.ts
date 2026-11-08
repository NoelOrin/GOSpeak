import { describe, expect, it } from "vitest";
import {
	domainDeepLinkUrl,
	extractDomainInviteCode,
	parseInviteLink,
} from "./domainInvite";

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

describe("parseInviteLink", () => {
	it("keeps the server of an https invite link", () => {
		expect(
			parseInviteLink("https://gospeak.example.com/invite/d/B2JF5EDV"),
		).toEqual({ code: "B2JF5EDV", server: "https://gospeak.example.com" });
	});

	it("keeps the port of an http invite link", () => {
		expect(
			parseInviteLink("http://192.168.1.9:8998/invite/d/B2JF5EDV"),
		).toEqual({ code: "B2JF5EDV", server: "http://192.168.1.9:8998" });
	});

	it("treats loopback links as having no server", () => {
		expect(parseInviteLink("http://127.0.0.1:47000/invite/d/B2JF5EDV")).toEqual(
			{ code: "B2JF5EDV" },
		);
		expect(parseInviteLink("http://localhost:3000/invite/d/B2JF5EDV")).toEqual({
			code: "B2JF5EDV",
		});
	});

	it("treats deep links as having no server", () => {
		expect(parseInviteLink("gospeak://invite/d/B2JF5EDV")).toEqual({
			code: "B2JF5EDV",
		});
	});

	it("treats a bare code or relative path as having no server", () => {
		expect(parseInviteLink("b2jf5edv")).toEqual({ code: "B2JF5EDV" });
		expect(parseInviteLink("/invite/d/B2JF5EDV")).toEqual({ code: "B2JF5EDV" });
	});

	it("returns null for unrelated text", () => {
		expect(parseInviteLink("https://example.com/home")).toBeNull();
		expect(parseInviteLink("")).toBeNull();
	});
});

describe("domainDeepLinkUrl", () => {
	it("builds the gospeak scheme link", () => {
		expect(domainDeepLinkUrl("B2JF5EDV")).toBe("gospeak://invite/d/B2JF5EDV");
	});
});
