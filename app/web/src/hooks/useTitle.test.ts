import { describe, expect, it } from "vitest";
import { buildDocumentTitle } from "./useTitle";

describe("buildDocumentTitle", () => {
	it("uses the app name when no route title exists", () => {
		expect(buildDocumentTitle()).toBe("GOSpeak");
	});

	it("prefixes route titles with the app name", () => {
		expect(buildDocumentTitle("首页")).toBe("首页 | GOSpeak");
	});

	it("ignores blank titles", () => {
		expect(buildDocumentTitle("  ")).toBe("GOSpeak");
	});
});
