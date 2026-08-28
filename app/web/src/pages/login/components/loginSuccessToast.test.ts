import { describe, expect, it, vi } from "vitest";

const showToast = vi.hoisted(() => vi.fn());

vi.mock("solid-notifications", () => ({
	showToast,
}));

import { showLoginSuccessToast } from "./loginSuccessToast";

describe("登录成功提示", () => {
	it("显示成功类型的登录提示", () => {
		showLoginSuccessToast();

		expect(showToast).toHaveBeenCalledWith("登录成功", { type: "success" });
	});
});
