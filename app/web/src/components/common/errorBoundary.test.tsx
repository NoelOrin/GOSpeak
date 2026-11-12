import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	type ErrorReportPayload,
	registerErrorReporter,
} from "@/utils/errorReport";
import {
	ErrorBoundary,
	PageErrorBoundary,
	RouteErrorComponent,
	withErrorBoundary,
} from "./errorBoundary";

let shouldThrow = true;

function Flaky(): ReactNode {
	if (shouldThrow) throw new Error("boom");
	return <div>内容已恢复</div>;
}

describe("ErrorBoundary", () => {
	let reports: ErrorReportPayload[];
	let unregister: () => void;

	beforeEach(() => {
		shouldThrow = true;
		reports = [];
		unregister = registerErrorReporter((payload) => {
			reports.push(payload);
		});
		// React 会把捕获到的错误再打一份到 console，压掉测试噪音
		vi.spyOn(console, "error").mockImplementation(() => {});
	});

	afterEach(() => {
		cleanup();
		unregister();
		vi.restoreAllMocks();
	});

	it("renders a component-level fallback instead of a blank screen", () => {
		render(
			<ErrorBoundary name="flaky-widget">
				<Flaky />
			</ErrorBoundary>,
		);

		expect(screen.getByRole("alert")).toBeTruthy();
		expect(screen.getByText("模块加载失败")).toBeTruthy();
		expect(screen.getByRole("button", { name: "重试" })).toBeTruthy();
		expect(screen.queryByText("内容已恢复")).toBeNull();

		expect(reports).toHaveLength(1);
		expect(reports[0]).toMatchObject({
			scope: "component",
			source: "flaky-widget",
		});
		expect(reports[0].error.message).toBe("boom");
	});

	it("remounts the subtree when the retry button is clicked", () => {
		render(
			<ErrorBoundary onError={vi.fn()}>
				<Flaky />
			</ErrorBoundary>,
		);

		shouldThrow = false;
		fireEvent.click(screen.getByRole("button", { name: "重试" }));

		expect(screen.getByText("内容已恢复")).toBeTruthy();
		expect(screen.queryByRole("alert")).toBeNull();
	});

	it("invokes onError and onReset hooks", () => {
		const onError = vi.fn();
		const onReset = vi.fn();
		render(
			<ErrorBoundary onError={onError} onReset={onReset}>
				<Flaky />
			</ErrorBoundary>,
		);
		expect(onError).toHaveBeenCalledTimes(1);
		expect(onError.mock.calls[0][0]).toBeInstanceOf(Error);

		shouldThrow = false;
		fireEvent.click(screen.getByRole("button", { name: "重试" }));
		expect(onReset).toHaveBeenCalledTimes(1);
	});

	it("auto-recovers when resetKeys change", () => {
		const view = render(
			<ErrorBoundary resetKeys={["room-a"]}>
				<Flaky />
			</ErrorBoundary>,
		);
		expect(screen.getByRole("alert")).toBeTruthy();

		shouldThrow = false;
		view.rerender(
			<ErrorBoundary resetKeys={["room-b"]}>
				<Flaky />
			</ErrorBoundary>,
		);
		expect(screen.getByText("内容已恢复")).toBeTruthy();
	});

	it("supports a render-prop fallback and a plain ReactNode fallback", () => {
		const { unmount } = render(
			<ErrorBoundary
				fallback={({ error, reset }) => (
					<button type="button" onClick={reset}>
						{error.message}
					</button>
				)}
			>
				<Flaky />
			</ErrorBoundary>,
		);
		expect(screen.getByRole("button", { name: "boom" })).toBeTruthy();
		unmount();

		render(
			<ErrorBoundary fallback={<p>自定义兜底</p>}>
				<Flaky />
			</ErrorBoundary>,
		);
		expect(screen.getByText("自定义兜底")).toBeTruthy();
	});

	it("skips reporting when report is false", () => {
		render(
			<ErrorBoundary report={false}>
				<Flaky />
			</ErrorBoundary>,
		);
		expect(screen.getByRole("alert")).toBeTruthy();
		expect(reports).toHaveLength(0);
	});
});

describe("PageErrorBoundary", () => {
	beforeEach(() => {
		shouldThrow = true;
		vi.spyOn(console, "error").mockImplementation(() => {});
	});

	afterEach(() => {
		cleanup();
		vi.restoreAllMocks();
	});

	it("shows a full-page fallback with navigation actions", () => {
		render(
			<PageErrorBoundary name="app-shell">
				<Flaky />
			</PageErrorBoundary>,
		);

		expect(screen.getByText("页面出错了")).toBeTruthy();
		expect(screen.getByRole("button", { name: "刷新页面" })).toBeTruthy();
		expect(screen.getByRole("link", { name: "返回首页" })).toBeTruthy();
	});
});

describe("RouteErrorComponent", () => {
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
		cleanup();
		unregister();
		vi.restoreAllMocks();
	});

	it("reports the route error and runs onRetry + reset on retry", () => {
		const reset = vi.fn();
		const onRetry = vi.fn();
		render(
			<RouteErrorComponent
				error={new Error("loader failed")}
				reset={reset}
				onRetry={onRetry}
			/>,
		);

		expect(screen.getByText("页面出错了")).toBeTruthy();
		expect(reports).toHaveLength(1);
		expect(reports[0].scope).toBe("route");

		fireEvent.click(screen.getByRole("button", { name: "重试" }));
		expect(onRetry).toHaveBeenCalledTimes(1);
		expect(reset).toHaveBeenCalledTimes(1);
	});

	it("survives non-Error values thrown by a loader", () => {
		render(
			<RouteErrorComponent
				error={{ message: "后端返回了字符串" }}
				reset={vi.fn()}
			/>,
		);
		expect(screen.getByRole("alert")).toBeTruthy();
		expect(reports[0].error.message).toBe("后端返回了字符串");
	});
});

describe("withErrorBoundary", () => {
	beforeEach(() => {
		shouldThrow = true;
		vi.spyOn(console, "error").mockImplementation(() => {});
	});

	afterEach(() => {
		cleanup();
		vi.restoreAllMocks();
	});

	it("wraps a component with a component-level boundary", () => {
		const SafeFlaky = withErrorBoundary(Flaky, { name: "safe-flaky" });
		expect(SafeFlaky.displayName).toContain("withErrorBoundary");

		render(<SafeFlaky />);
		expect(screen.getByRole("alert")).toBeTruthy();
		expect(screen.getByText("模块加载失败")).toBeTruthy();
	});
});
