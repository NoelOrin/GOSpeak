import { beforeEach, describe, expect, it, vi } from "vitest";

const toastMock = vi.hoisted(() => ({
	error: vi.fn(),
	success: vi.fn(),
	warning: vi.fn(),
	info: vi.fn(),
}));

vi.mock("@gospeak/components", () => ({ toast: toastMock }));

import { toastActionError } from "./toast";

/** 模拟 axios 错误：拦截器处理过 4xx/5xx/网络错误，都会带 isAxiosError。 */
const axiosError = (
	status: number | undefined,
	data?: { code?: number; msg?: string },
) => ({ isAxiosError: true, response: { status, data } });

describe("toastActionError", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it("returns the backend message without toasting when the interceptor already did", () => {
		expect(toastActionError(axiosError(500, { msg: "服务器错误" }))).toBe(
			"服务器错误",
		);
		expect(toastMock.error).not.toHaveBeenCalled();
	});

	it("does not toast 4xx business errors the interceptor already surfaced", () => {
		// 4xx 也由拦截器统一提示，调用方再弹一次就是重复 toast
		expect(toastActionError(axiosError(404, { msg: "房间不存在" }))).toBe(
			"房间不存在",
		);
		expect(toastMock.error).not.toHaveBeenCalled();
	});

	it("respects the __toastShown mark", () => {
		expect(
			toastActionError({ isAxiosError: true, __toastShown: true }),
		).toBeDefined();
		expect(toastMock.error).not.toHaveBeenCalled();
	});

	it("still toasts when the caller silenced the interceptor", () => {
		const silentError = {
			...axiosError(403, { msg: "该域未开放访客进入" }),
			__toastShown: false,
		};
		expect(toastActionError(silentError)).toBe("该域未开放访客进入");
		expect(toastMock.error).toHaveBeenCalledWith("该域未开放访客进入");
	});

	it("toasts local errors that never reached axios", () => {
		expect(toastActionError(new Error("房间已满"), "加入失败")).toBe(
			"房间已满",
		);
		expect(toastMock.error).toHaveBeenCalledWith("房间已满");
	});

	it("uses the fallback copy when the error carries no message", () => {
		expect(toastActionError(undefined, "删除失败")).toBe("删除失败");
		expect(toastMock.error).toHaveBeenCalledWith("删除失败");
	});
});
