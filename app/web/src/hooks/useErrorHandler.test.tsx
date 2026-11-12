import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ErrorBoundary } from "@/components/common/errorBoundary";
import { useErrorHandler } from "./useErrorHandler";

function AsyncAction() {
	const throwError = useErrorHandler();
	// 真实用法是 catch (error) { throwError(error) }；这里直接触发回调，
	// 避免测试里额外处理 Promise 的 act 时序。
	const simulateCatch = (error: unknown) => throwError(error);
	return (
		<button
			type="button"
			onClick={() => simulateCatch(new Error("async boom"))}
		>
			触发异步错误
		</button>
	);
}

describe("useErrorHandler", () => {
	beforeEach(() => {
		vi.spyOn(console, "error").mockImplementation(() => {});
	});

	afterEach(() => {
		cleanup();
		vi.restoreAllMocks();
	});

	it("rethrows async errors into the nearest error boundary", () => {
		render(
			<ErrorBoundary name="async-panel">
				<AsyncAction />
			</ErrorBoundary>,
		);
		expect(screen.getByRole("button", { name: "触发异步错误" })).toBeTruthy();

		fireEvent.click(screen.getByRole("button", { name: "触发异步错误" }));

		expect(screen.getByRole("alert")).toBeTruthy();
		expect(screen.getByText("模块加载失败")).toBeTruthy();
	});
});
