import { describe, expect, it } from "vitest";
import { shouldSkipLoginRedirect } from "@/api/apiClient";

/**
 * P0-1 回归保护：邀请链接是对外分享的唯一入口。
 * 未登录时 ensureSession 会拿到 401，若拦截器整页硬跳 /login，
 * beforeLoad 里带邀请码的 redirect 会被吞掉。
 */
describe("shouldSkipLoginRedirect", () => {
	it("leaves invite landing pages to the router so the code survives", () => {
		expect(shouldSkipLoginRedirect("/invite/d/ABCD1234")).toBe(true);
		expect(shouldSkipLoginRedirect("/invite/d/abcd-1234")).toBe(true);
	});

	it("leaves auth pages alone to avoid a refresh loop", () => {
		expect(shouldSkipLoginRedirect("/login")).toBe(true);
		expect(shouldSkipLoginRedirect("/register")).toBe(true);
	});

	it("still hard-redirects protected pages on a dead session", () => {
		expect(shouldSkipLoginRedirect("/")).toBe(false);
		expect(shouldSkipLoginRedirect("/discover")).toBe(false);
		expect(shouldSkipLoginRedirect("/domain/domain-1")).toBe(false);
		expect(shouldSkipLoginRedirect("/manage/users")).toBe(false);
	});
});
