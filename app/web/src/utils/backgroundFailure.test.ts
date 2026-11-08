import { beforeEach, describe, expect, it, vi } from "vitest";

const { toastMock } = vi.hoisted(() => ({
	toastMock: { error: vi.fn(), warning: vi.fn(), success: vi.fn() },
}));

vi.mock("@gospeak/components", () => ({ toast: toastMock }));

import {
	clearBackgroundFailure,
	reportBackgroundFailure,
	resetBackgroundFailures,
} from "./backgroundFailure";

describe("reportBackgroundFailure", () => {
	beforeEach(() => {
		vi.clearAllMocks();
		resetBackgroundFailures();
	});

	it("shows one toast per scope and returns a user-facing message", () => {
		const error = new Error("network");
		const first = reportBackgroundFailure(
			"members:g-1",
			error,
			"成员列表加载失败",
		);
		const second = reportBackgroundFailure(
			"members:g-1",
			error,
			"成员列表加载失败",
		);

		expect(first).toBe("成员列表加载失败");
		expect(second).toBe("成员列表加载失败");
		expect(toastMock.error).toHaveBeenCalledTimes(1);
	});

	it("keeps different scopes independent", () => {
		const error = new Error("network");
		reportBackgroundFailure("members:g-1", error, "成员列表加载失败");
		reportBackgroundFailure("permissions:g-1", error, "权限加载失败");
		expect(toastMock.error).toHaveBeenCalledTimes(2);
	});

	it("does not repeat a toast the api interceptor already showed", () => {
		const error = Object.assign(new Error("boom"), { __toastShown: true });
		reportBackgroundFailure("domains", error, "域列表加载失败");
		expect(toastMock.error).not.toHaveBeenCalled();
	});

	it("allows a new toast after a successful reload clears the scope", () => {
		const error = new Error("network");
		reportBackgroundFailure("devices", error, "设备读取失败");
		clearBackgroundFailure("devices");
		reportBackgroundFailure("devices", error, "设备读取失败");
		expect(toastMock.error).toHaveBeenCalledTimes(2);
	});
});
