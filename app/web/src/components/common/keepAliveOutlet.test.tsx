import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const routerState = vi.hoisted(() => ({ pathname: "/a" }));

vi.mock("@tanstack/react-router", () => ({
	useLocation: () => ({ pathname: routerState.pathname }),
	Outlet: () => <div data-testid="outlet" />,
}));

// 默认页面表会拉入真实页面（及整棵组件依赖），测试只用注入的假页面。
vi.mock("@/pages/(app)/chat", () => ({ ChatPage: () => null }));
vi.mock("@/pages/(app)/discover", () => ({ DiscoverPage: () => null }));
vi.mock("@/pages/(app)/profile", () => ({ ProfilePage: () => null }));

import { KeepAliveOutlet } from "./keepAliveOutlet";

function PageA() {
	const [count, setCount] = useState(0);
	return (
		<button type="button" onClick={() => setCount((v) => v + 1)}>
			A:{count}
		</button>
	);
}

function PageB() {
	return <div>B</div>;
}

const pages = { "/a": <PageA />, "/b": <PageB /> };

/** 取某个页面所在的外壳（保活容器用 data-keep-alive 标记 pathname）。 */
function shellOf(text: string): HTMLElement {
	const shell = screen.getByText(text).closest("[data-keep-alive]");
	if (!(shell instanceof HTMLElement)) throw new Error("shell not found");
	return shell;
}

afterEach(() => {
	cleanup();
	routerState.pathname = "/a";
});

describe("KeepAliveOutlet", () => {
	it("首次进入只渲染当前页", () => {
		render(<KeepAliveOutlet pages={pages} />);
		expect(screen.getByText("A:0")).toBeTruthy();
		expect(screen.queryByText("B")).toBeNull();
		expect(screen.queryByTestId("outlet")).toBeNull();
	});

	it("切到另一页后旧页仍留在 DOM，只是被隐藏", () => {
		const { rerender } = render(<KeepAliveOutlet pages={pages} />);
		routerState.pathname = "/b";
		rerender(<KeepAliveOutlet pages={pages} />);

		expect(shellOf("B").style.visibility).toBe("visible");
		expect(shellOf("A:0").style.visibility).toBe("hidden");
		expect(shellOf("A:0").getAttribute("aria-hidden")).toBe("true");
	});

	it("切回后组件状态不丢（没有被重新挂载）", () => {
		const { rerender } = render(<KeepAliveOutlet pages={pages} />);
		fireEvent.click(screen.getByText("A:0"));
		expect(screen.getByText("A:1")).toBeTruthy();

		routerState.pathname = "/b";
		rerender(<KeepAliveOutlet pages={pages} />);
		routerState.pathname = "/a";
		rerender(<KeepAliveOutlet pages={pages} />);

		expect(screen.getByText("A:1")).toBeTruthy();
		expect(shellOf("A:1").style.visibility).toBe("visible");
	});

	it("非保活路由走 Outlet，已挂载的页面继续保持隐藏", () => {
		const { rerender } = render(<KeepAliveOutlet pages={pages} />);
		routerState.pathname = "/other";
		rerender(<KeepAliveOutlet pages={pages} />);

		expect(screen.getByTestId("outlet")).toBeTruthy();
		expect(shellOf("A:0").style.visibility).toBe("hidden");
	});

	it("从没进过保活页时直接用 Outlet，不额外套容器", () => {
		routerState.pathname = "/other";
		const { container } = render(<KeepAliveOutlet pages={pages} />);
		expect(screen.getByTestId("outlet")).toBeTruthy();
		expect(container.querySelector("[data-keep-alive]")).toBeNull();
	});
});
