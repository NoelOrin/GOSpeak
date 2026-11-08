import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	type ErrorReportPayload,
	errorSummary,
	installGlobalErrorHandlers,
	normalizeError,
	registerErrorReporter,
	reportError,
} from "./errorReport";

describe("normalizeError", () => {
	it("keeps real Error instances as-is", () => {
		const error = new Error("original");
		expect(normalizeError(error)).toBe(error);
	});

	it("wraps strings and error-like objects", () => {
		expect(normalizeError("bad thing").message).toBe("bad thing");
		const wrapped = normalizeError({ message: "oops", name: "CustomError" });
		expect(wrapped.message).toBe("oops");
		expect(wrapped.name).toBe("CustomError");
	});

	it("handles undefined and circular values without throwing", () => {
		expect(normalizeError(undefined).message).toContain("Unknown error");
		const circular: Record<string, unknown> = {};
		circular.self = circular;
		expect(normalizeError(circular).message).toBe("Unknown error");
	});

	it("builds a short user-facing summary", () => {
		expect(errorSummary(new TypeError("nope"))).toBe("TypeError: nope");
	});
});

describe("reportError", () => {
	let reports: ErrorReportPayload[];
	let unregister: () => void;

	beforeEach(() => {
		reports = [];
		unregister = registerErrorReporter((payload) => {
			reports.push(payload);
		});
		vi.spyOn(console, "error").mockImplementation(() => {});
	});

	afterEach(() => {
		unregister();
		vi.restoreAllMocks();
	});

	it("forwards normalized payloads to registered reporters", () => {
		const error = new Error("boom");
		reportError({
			error,
			scope: "component",
			source: "member-list",
			componentStack: "at MemberList",
		});

		expect(reports).toHaveLength(1);
		expect(reports[0]).toMatchObject({
			error,
			scope: "component",
			source: "member-list",
			componentStack: "at MemberList",
		});
		expect(reports[0].timestamp).toBeTypeOf("number");
	});

	it("deduplicates the same error and allows a more precise scope to upgrade it", () => {
		const error = new Error("boom");
		reportError({ error, scope: "global" });
		reportError({ error, scope: "global" });
		expect(reports).toHaveLength(1);

		reportError({ error, scope: "component" });
		expect(reports).toHaveLength(2);
		expect(reports[1].scope).toBe("component");

		// page 的优先级低于 component，不再重复上报
		reportError({ error, scope: "page" });
		expect(reports).toHaveLength(2);
	});

	it("keeps other reporters working when one throws", () => {
		const broken = registerErrorReporter(() => {
			throw new Error("reporter down");
		});
		try {
			expect(() =>
				reportError({ error: new Error("boom"), scope: "page" }),
			).not.toThrow();
			expect(reports).toHaveLength(1);
		} finally {
			broken();
		}
	});

	it("normalizes non-Error values before reporting", () => {
		reportError({ error: "plain string", scope: "global" });
		expect(reports[0].error).toBeInstanceOf(Error);
		expect(reports[0].error.message).toBe("plain string");
	});
});

describe("installGlobalErrorHandlers", () => {
	let reports: ErrorReportPayload[];
	let unregister: () => void;

	beforeEach(() => {
		reports = [];
		unregister = registerErrorReporter((payload) => {
			reports.push(payload);
		});
		vi.spyOn(console, "error").mockImplementation(() => {});
	});

	afterEach(() => {
		unregister();
		vi.restoreAllMocks();
	});

	it("records window.error and unhandledrejection and can be uninstalled", () => {
		const uninstall = installGlobalErrorHandlers();
		// jsdom 会把未 preventDefault 的 error 事件当成未捕获异常抛给 vitest，
		// 预先取消默认行为即可，不影响被测 listener 是否被调用。
		const dispatch = (type: string, payload: Record<string, unknown>) => {
			const event = Object.assign(
				new Event(type, { cancelable: true }),
				payload,
			);
			event.preventDefault();
			window.dispatchEvent(event);
		};

		dispatch("error", {
			error: new Error("window error"),
			message: "window error",
		});
		dispatch("unhandledrejection", {
			reason: new Error("promise rejected"),
		});

		expect(reports.map((r) => r.source)).toEqual([
			"window.error",
			"window.unhandledrejection",
		]);
		expect(reports[0].error.message).toBe("window error");
		expect(reports[1].error.message).toBe("promise rejected");

		uninstall();
		// 只带 message 的事件不会被 jsdom 当成未捕获异常；
		// 若 listener 没卸载干净，这里会多出一条上报记录。
		dispatch("error", { message: "after off" });
		expect(reports).toHaveLength(2);
	});
});
