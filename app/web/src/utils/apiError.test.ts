import { describe, expect, it } from "vitest";
import { apiErrorCode, apiErrorMessage } from "./apiError";

describe("apiErrorMessage", () => {
	it("prefers backend Result.msg", () => {
		const error = { response: { data: { msg: "房间已满" } } };
		expect(apiErrorMessage(error)).toBe("房间已满");
	});

	it("falls back to Error.message", () => {
		expect(apiErrorMessage(new Error("boom"))).toBe("boom");
	});

	it("falls back to the provided default", () => {
		expect(apiErrorMessage({}, "删除房间失败")).toBe("删除房间失败");
		expect(apiErrorMessage(new Error(""), "请求失败")).toBe("请求失败");
	});

	it("ignores non-string backend msg", () => {
		const error = { response: { data: { msg: undefined } } };
		expect(apiErrorMessage(error, "请求失败")).toBe("请求失败");
	});
});

describe("apiErrorCode", () => {
	it("reads business code from the response body", () => {
		expect(apiErrorCode({ response: { data: { code: 1017 } } })).toBe(1017);
	});

	it("returns undefined without a response", () => {
		expect(apiErrorCode(new Error("network"))).toBeUndefined();
	});
});
