import { cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import AsyncState from "./AsyncState";

afterEach(cleanup);

describe("AsyncState", () => {
	it("loading 且提供 skeleton 时渲染骨架而不是 spinner", () => {
		const { getByTestId, container } = render(
			<AsyncState
				loading
				loadingText="正在加载消息…"
				skeleton={<div data-testid="sk">骨架</div>}
			>
				<div>内容</div>
			</AsyncState>,
		);
		expect(getByTestId("sk")).toBeTruthy();
		expect(container.querySelector(".animate-spin")).toBeNull();
	});

	it("loading 且无 skeleton 时保留 spinner + loadingText", () => {
		const { getByText } = render(
			<AsyncState loading loadingText="正在加载…" />,
		);
		expect(getByText("正在加载…")).toBeTruthy();
	});

	it("非 loading/error/empty 时渲染 children", () => {
		const { getByText } = render(
			<AsyncState>
				<div>内容</div>
			</AsyncState>,
		);
		expect(getByText("内容")).toBeTruthy();
	});
});
