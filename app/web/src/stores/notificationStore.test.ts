import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/api/announcement", () => ({
	listAnnouncements: vi.fn(async () => ({ items: [], total: 0 })),
	getUnreadCount: vi.fn(async () => 3),
	markAnnouncementRead: vi.fn(async () => {}),
}));

vi.mock("@gospeak/components", () => ({
	toast: { info: vi.fn(), error: vi.fn() },
}));

import { getUnreadCount, markAnnouncementRead } from "@/api/announcement";
import { notificationStore, useNotificationStore } from "./notificationStore";

const dto = (uuid: string, isRead = false) => ({
	uuid,
	scope_type: "platform" as const,
	title: `t-${uuid}`,
	content: "body",
	level: "info" as const,
	pinned: false,
	created_by_name: "admin",
	is_read: isRead,
	created_at: Date.now(),
});

describe("notificationStore", () => {
	beforeEach(() => {
		vi.clearAllMocks();
		useNotificationStore.setState({
			unreadCount: 0,
			items: [],
			loading: false,
		});
	});

	it("loads unread count", async () => {
		await notificationStore.loadUnreadCount();
		expect(useNotificationStore.getState().unreadCount).toBe(3);
		expect(getUnreadCount).toHaveBeenCalled();
	});

	it("increments on incoming and prepends", () => {
		notificationStore.handleIncoming(dto("a"));
		const state = useNotificationStore.getState();
		expect(state.unreadCount).toBe(1);
		expect(state.items[0].uuid).toBe("a");
	});

	it("does not double count the same uuid", () => {
		notificationStore.handleIncoming(dto("a"));
		notificationStore.handleIncoming(dto("a"));
		expect(useNotificationStore.getState().unreadCount).toBe(1);
	});

	it("optimistically marks read and calls the API", async () => {
		useNotificationStore.setState({
			unreadCount: 1,
			items: [dto("a")],
			loading: false,
		});
		await notificationStore.markRead("a");
		const state = useNotificationStore.getState();
		expect(state.unreadCount).toBe(0);
		expect(state.items[0].is_read).toBe(true);
		expect(markAnnouncementRead).toHaveBeenCalledWith("a");
	});

	it("rolls back when the API fails", async () => {
		vi.mocked(markAnnouncementRead).mockRejectedValueOnce(new Error("boom"));
		useNotificationStore.setState({
			unreadCount: 1,
			items: [dto("a")],
			loading: false,
		});
		await notificationStore.markRead("a");
		const state = useNotificationStore.getState();
		expect(state.unreadCount).toBe(1);
		expect(state.items[0].is_read).toBe(false);
	});

	it("ignores markRead for an already read item", async () => {
		useNotificationStore.setState({
			unreadCount: 0,
			items: [dto("a", true)],
			loading: false,
		});
		await notificationStore.markRead("a");
		expect(markAnnouncementRead).not.toHaveBeenCalled();
	});
});
