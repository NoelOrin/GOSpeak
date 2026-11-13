import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useUserStore } from "@/stores/userStore";

vi.mock("@/api/system", () => ({
	checkServerUpdate: vi.fn(),
}));

import { ServerUpdateSection } from "./serverUpdate";

const setUser = (role: string) =>
	useUserStore.setState({ user: { id: 1, role } as never });

describe("ServerUpdateSection（Web 构建）", () => {
	beforeEach(() => {
		cleanup();
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
	});

	it("管理员看到「检查更新」，另有发布页入口", () => {
		setUser("admin");
		render(<ServerUpdateSection />);
		expect(screen.getByRole("button", { name: "检查更新" })).toBeTruthy();
		expect(screen.getByRole("link", { name: "前往发布页" })).toBeTruthy();
	});

	it("普通用户看不到「检查更新」，只能去发布页", () => {
		setUser("user");
		render(<ServerUpdateSection />);
		expect(screen.queryByRole("button", { name: "检查更新" })).toBeNull();
		expect(screen.getByRole("link", { name: "前往发布页" })).toBeTruthy();
	});
});
