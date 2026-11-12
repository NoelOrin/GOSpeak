import { describe, expect, it } from "vitest";
import {
	domainDeepLinkUrl,
	domainInviteUrl,
	extractDomainInviteCode,
	parseInviteLink,
} from "./domainInvite";

/** 36 字符邀请码：字符集 [A-HJ-NP-Z2-9]（排除 I、O、0、1）。 */
const VALID = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789ABCD";

describe("extractDomainInviteCode", () => {
	it("accepts a 36-char code", () => {
		expect(extractDomainInviteCode(VALID)).toBe(VALID);
	});

	it("normalizes lowercase codes to upper", () => {
		expect(extractDomainInviteCode(VALID.toLowerCase())).toBe(VALID);
	});

	it("rejects the legacy 8-char code", () => {
		expect(extractDomainInviteCode("ABCDEFGH")).toBeNull();
	});

	it("rejects codes containing I or O", () => {
		expect(extractDomainInviteCode(`${VALID.slice(0, 35)}I`)).toBeNull();
		expect(extractDomainInviteCode(`${VALID.slice(0, 35)}O`)).toBeNull();
	});

	it("extracts code from invite link path", () => {
		expect(
			extractDomainInviteCode(`https://gospeak.local/invite/d/${VALID}`),
		).toBe(VALID);
	});

	it("accepts the invite path without relying on a host", () => {
		expect(extractDomainInviteCode(`/invite/d/${VALID}`)).toBe(VALID);
	});

	it("extracts code from invite link with query", () => {
		expect(
			extractDomainInviteCode(
				`https://gospeak.local/invite/d/${VALID}?from=test`,
			),
		).toBe(VALID);
	});

	it("extracts code from a query parameter", () => {
		expect(extractDomainInviteCode(`https://example.com/x?code=${VALID}`)).toBe(
			VALID,
		);
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
			parseInviteLink(`https://gospeak.example.com/invite/d/${VALID}`),
		).toEqual({ code: VALID, server: "https://gospeak.example.com" });
	});

	it("keeps the port of an http invite link", () => {
		expect(
			parseInviteLink(`http://192.168.1.9:8998/invite/d/${VALID}`),
		).toEqual({ code: VALID, server: "http://192.168.1.9:8998" });
	});

	it("treats loopback links as having no server", () => {
		expect(parseInviteLink(`http://127.0.0.1:47000/invite/d/${VALID}`)).toEqual(
			{
				code: VALID,
			},
		);
		expect(parseInviteLink(`http://localhost:3000/invite/d/${VALID}`)).toEqual({
			code: VALID,
		});
	});

	it("treats deep links as having no server", () => {
		expect(parseInviteLink(`gospeak://invite/d/${VALID}`)).toEqual({
			code: VALID,
		});
	});

	it("treats a bare code or relative path as having no server", () => {
		expect(parseInviteLink(VALID.toLowerCase())).toEqual({ code: VALID });
		expect(parseInviteLink(`/invite/d/${VALID}`)).toEqual({ code: VALID });
	});

	it("returns null for unrelated text", () => {
		expect(parseInviteLink("https://example.com/home")).toBeNull();
		expect(parseInviteLink("")).toBeNull();
	});
});

describe("domainInviteUrl / domainDeepLinkUrl", () => {
	it("builds a browser invite link", () => {
		expect(domainInviteUrl(VALID, "https://example.com")).toBe(
			`https://example.com/invite/d/${VALID}`,
		);
	});

	it("builds the gospeak scheme link", () => {
		expect(domainDeepLinkUrl(VALID)).toBe(`gospeak://invite/d/${VALID}`);
	});
});
