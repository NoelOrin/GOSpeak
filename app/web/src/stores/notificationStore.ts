import { toast } from "@gospeak/components";
import { create } from "zustand";
import {
	getUnreadCount,
	listAnnouncements,
	markAnnouncementRead,
} from "@/api/announcement";
import type { AnnouncementDTO } from "@/protocol/announcement";

interface NotificationState {
	unreadCount: number;
	items: AnnouncementDTO[];
	loading: boolean;
}

export const useNotificationStore = create<NotificationState>(() => ({
	unreadCount: 0,
	items: [],
	loading: false,
}));

/** 重连后必须重新拉一次：WS 不重放断线期间的事件。 */
async function loadUnreadCount(): Promise<void> {
	try {
		const count = await getUnreadCount();
		useNotificationStore.setState({ unreadCount: count });
	} catch {
		// 未登录或网络失败：保持当前值，等下次连接成功再拉
	}
}

async function loadList(onlyUnread = false): Promise<void> {
	useNotificationStore.setState({ loading: true });
	try {
		const result = await listAnnouncements(onlyUnread);
		useNotificationStore.setState({ items: result.items, loading: false });
	} catch {
		useNotificationStore.setState({ loading: false });
	}
}

function handleIncoming(dto: AnnouncementDTO): void {
	const state = useNotificationStore.getState();
	if (state.items.some((item) => item.uuid === dto.uuid)) return;
	useNotificationStore.setState({
		items: [dto, ...state.items],
		unreadCount: state.unreadCount + 1,
	});
	toast.info(dto.title);
}

async function markRead(uuid: string): Promise<void> {
	const before = useNotificationStore.getState();
	const target = before.items.find((item) => item.uuid === uuid);
	if (!target || target.is_read) return;

	useNotificationStore.setState({
		unreadCount: Math.max(0, before.unreadCount - 1),
		items: before.items.map((item) =>
			item.uuid === uuid ? { ...item, is_read: true } : item,
		),
	});

	try {
		await markAnnouncementRead(uuid);
	} catch {
		useNotificationStore.setState({
			unreadCount: before.unreadCount,
			items: before.items,
		});
		toast.error("标记已读失败");
	}
}

function reset(): void {
	useNotificationStore.setState({ unreadCount: 0, items: [], loading: false });
}

export const notificationStore = {
	unreadCount: () => useNotificationStore.getState().unreadCount,
	items: () => useNotificationStore.getState().items,
	loading: () => useNotificationStore.getState().loading,
	loadUnreadCount,
	loadList,
	handleIncoming,
	markRead,
	reset,
};

export default notificationStore;
