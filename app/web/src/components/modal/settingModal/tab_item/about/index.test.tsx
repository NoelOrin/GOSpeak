import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useUserStore } from "@/stores/userStore";

const getPublicServerVersion = vi.fn();

vi.mock("@/api/system", () => ({
	getPublicServerVersion: () => getPublicServerVersion(),
	checkServerUpdate: vi.fn(),
}));

// Web 构建：桌面能力一律不可用（isDesktop 为 false），验证平台隔离的兜底路径。
vi.mock("@gospeak/desktop", () => ({
	createDesktop: () => ({
		isDesktop: false,
		updater: {
			currentVersion: () => Promise.reject(new Error("not desktop")),
		},
	}),
}));

import { AboutPage } from "./index";

describe("「关于」页（Web 构建）", () => {
	beforeEach(() => {
		cleanup();
		getPublicServerVersion.mockReset();
		getPublicServerVersion.mockResolvedValue({
			version: "1.0.1",
			commit: "abcdef1234",
		});
		useUserStore.setState({ user: { id: 1, role: "user" } as never });
	});

	afterEach(() => {
		cleanup();
	});

	it("展示前端、后端版本与项目地址", async () => {
		render(<AboutPage />);

		expect(screen.getByText("前端")).toBeTruthy();
		expect(screen.getByText("后端")).toBeTruthy();
		await waitFor(() => expect(screen.getByText("1.0.1")).toBeTruthy());
		expect(screen.getByText("commit abcdef1")).toBeTruthy();
		expect(screen.getByRole("link", { name: /项目主页/ })).toBeTruthy();
	});

	it("Web 构建不出现桌面端版本行与安装入口", async () => {
		render(<AboutPage />);
		// 等版本请求落地再断言，避免未 settle 的 promise 在清理后触发状态更新。
		await waitFor(() => expect(screen.getByText("1.0.1")).toBeTruthy());

		expect(screen.queryByText("桌面端")).toBeNull();
		expect(screen.queryByRole("button", { name: "下载并安装" })).toBeNull();
	});

	it("后端版本拉取失败时回退成「未知」，不卡在省略号", async () => {
		getPublicServerVersion.mockRejectedValue(new Error("network"));
		render(<AboutPage />);
		await waitFor(() => expect(screen.getByText("未知")).toBeTruthy());
	});
});
