import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// vi.mock 工厂会被提升到 import 之前，因此共享状态必须用 vi.hoisted 创建。
const { state, markRead, loadList } = vi.hoisted(() => ({
	state: {
		items: [] as Array<Record<string, unknown>>,
		loading: false,
		unreadCount: 0,
	},
	markRead: vi.fn(),
	loadList: vi.fn(),
}));

vi.mock("@/stores/notificationStore", () => ({
	useNotificationStore: (selector: (s: typeof state) => unknown) =>
		selector(state),
	notificationStore: { loadList, markRead },
}));

import NotificationModal from "./notificationModal";

describe("NotificationModal", () => {
	beforeEach(() => {
		vi.clearAllMocks();
		state.items = [];
		state.loading = false;
	});

	// Radix Dialog 渲染在 portal 里，不清理会跨用例残留导致 getByText 命中多个。
	afterEach(() => {
		cleanup();
	});

	it("shows the empty state when there are no notifications", () => {
		render(<NotificationModal open onClose={() => {}} />);
		expect(screen.getByText("暂无通知")).toBeTruthy();
	});

	it("closes when the close button is clicked", async () => {
		const onClose = vi.fn();
		render(<NotificationModal open onClose={onClose} />);
		await userEvent.click(screen.getByLabelText("关闭"));
		expect(onClose).toHaveBeenCalled();
	});

	it("loads the list when opened", () => {
		render(<NotificationModal open onClose={() => {}} />);
		expect(loadList).toHaveBeenCalled();
	});

	it("marks an item read when selected", async () => {
		state.items = [
			{
				uuid: "a-1",
				scope_type: "platform",
				title: "维护公告",
				content: "body",
				level: "info",
				pinned: false,
				created_by_name: "admin",
				is_read: false,
				created_at: Date.now(),
			},
		];
		render(<NotificationModal open onClose={() => {}} />);
		// 列表项与详情标题都会渲染标题文本，这里只点列表里的那个按钮。
		const listButton = screen
			.getAllByText("维护公告")
			.find((el) => el.closest("button") !== null);
		expect(listButton).toBeTruthy();
		await userEvent.click(listButton as HTMLElement);
		expect(markRead).toHaveBeenCalledWith("a-1");
	});

	it("renders markdown content sanitized", () => {
		state.items = [
			{
				uuid: "a-2",
				scope_type: "platform",
				title: "安全",
				content: '<img src=x onerror="alert(1)">正文',
				level: "warning",
				pinned: false,
				created_by_name: "admin",
				is_read: true,
				created_at: Date.now(),
			},
		];
		const { container } = render(
			<NotificationModal open onClose={() => {}} />,
		);
		expect(container.querySelector("img[onerror]")).toBeNull();
	});
});
