import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useThemeStore } from "@/stores/themeStore";

const setAppearance = vi.fn(() => Promise.resolve());

vi.mock("@tanstack/react-router", () => ({
	useMatches: () => [{ staticData: { title: "域管理" } }],
	useLocation: () => ({ pathname: "/manage/domains" }),
}));

vi.mock("@gospeak/desktop", () => ({
	createDesktop: () => ({ window: { setAppearance } }),
}));

vi.mock("@/layouts/common/connectionIndicator", () => ({
	default: () => <span data-testid="connection" />,
}));

import DesktopTitleBar from "./desktopTitleBar";

describe("DesktopTitleBar", () => {
	beforeEach(() => {
		cleanup();
		setAppearance.mockClear();
		useThemeStore.setState({ isDark: false });
	});

	afterEach(() => {
		cleanup();
	});

	it("桌面构建渲染标题栏，Web 构建不渲染任何节点", () => {
		const { container } = render(<DesktopTitleBar />);
		if (__GOSPEAK_DESKTOP__) {
			expect(screen.getByText("域管理")).toBeTruthy();
		} else {
			expect(container.firstElementChild).toBeNull();
		}
	});

	it("桌面构建把应用主题推给原生外观", () => {
		render(<DesktopTitleBar />);
		if (__GOSPEAK_DESKTOP__) {
			expect(setAppearance).toHaveBeenCalledWith("light");
		} else {
			expect(setAppearance).not.toHaveBeenCalled();
		}
	});
});
