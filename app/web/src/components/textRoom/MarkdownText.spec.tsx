// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { renderMarkdown } from "./MarkdownText";

describe("renderMarkdown", () => {
	it("renders common markdown blocks", () => {
		const html = renderMarkdown(
			"# 标题\n\n**加粗** `code` [链接](https://example.com)",
		);
		expect(html).toContain("<h1>标题</h1>");
		expect(html).toContain("<strong>加粗</strong>");
		expect(html).toContain("<code>code</code>");
		expect(html).toContain('href="https://example.com"');
	});

	it("renders images for uploaded attachments", () => {
		const html = renderMarkdown("![截图](/uploads/chat/a.png)");
		expect(html).toContain("<img");
		expect(html).toContain('src="/uploads/chat/a.png"');
	});

	it("sanitizes raw HTML", () => {
		const html = renderMarkdown("<script>alert(1)</script>");
		expect(html).not.toContain("<script");
		expect(html).not.toContain("alert(1)");
	});
});
