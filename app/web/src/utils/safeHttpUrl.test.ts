import { describe, expect, it } from "vitest";
import { safeHttpUrl } from "./safeHttpUrl";

/**
 * 发布页链接来自后端（其上游是 GitHub API），必须校验 scheme 后再放进 href：
 * 一个 javascript:/data: 链接会被浏览器当成当前页的脚本执行。
 */
describe("safeHttpUrl", () => {
	it("放行 http / https", () => {
		expect(safeHttpUrl("https://github.com/gospeak/releases/tag/v1.2.0")).toBe(
			"https://github.com/gospeak/releases/tag/v1.2.0",
		);
		expect(safeHttpUrl("http://example.com/release")).toBe(
			"http://example.com/release",
		);
	});

	it("拒绝非 http(s) scheme", () => {
		expect(safeHttpUrl("javascript:alert(1)")).toBeNull();
		expect(safeHttpUrl("data:text/html,<script>alert(1)</script>")).toBeNull();
		expect(safeHttpUrl("file:///etc/passwd")).toBeNull();
		expect(safeHttpUrl("ftp://files.example.com/x")).toBeNull();
	});

	it("空值与不可解析值返回 null（不渲染链接）", () => {
		expect(safeHttpUrl(undefined)).toBeNull();
		expect(safeHttpUrl("")).toBeNull();
		expect(safeHttpUrl("not a url")).toBeNull();
		expect(safeHttpUrl("/relative/path")).toBeNull();
	});
});
