import type {
	AnnouncementDTO,
	AnnouncementInput,
	AnnouncementListResult,
} from "@/protocol/announcement";
import apiClient from "./apiClient";

export async function listAnnouncements(
	onlyUnread = false,
	page = 1,
	pageSize = 20,
): Promise<AnnouncementListResult> {
	const data = await apiClient.post<AnnouncementListResult>({
		url: "/api/v1/announcement/list",
		data: { only_unread: onlyUnread, page, page_size: pageSize },
		skipErrorToast: true,
	});
	return data ?? { items: [], total: 0 };
}

export async function getUnreadCount(): Promise<number> {
	const data = await apiClient.post<{ count: number }>({
		url: "/api/v1/announcement/unread-count",
		data: {},
		skipErrorToast: true,
	});
	return data?.count ?? 0;
}

export async function markAnnouncementRead(uuid: string): Promise<void> {
	await apiClient.post({
		url: "/api/v1/announcement/mark-read",
		data: { uuid },
	});
}

export async function adminListAnnouncements(
	page = 1,
	pageSize = 50,
): Promise<AnnouncementListResult> {
	const data = await apiClient.post<AnnouncementListResult>({
		url: "/api/v1/announcement/admin/list",
		data: { page, page_size: pageSize },
	});
	return data ?? { items: [], total: 0 };
}

export async function createAnnouncement(
	input: AnnouncementInput,
): Promise<AnnouncementDTO> {
	const data = await apiClient.post<AnnouncementDTO>({
		url: "/api/v1/announcement/admin/create",
		data: input,
	});
	if (!data) throw new Error("announcement missing");
	return data;
}

export async function updateAnnouncement(
	input: AnnouncementInput,
): Promise<AnnouncementDTO> {
	const data = await apiClient.post<AnnouncementDTO>({
		url: "/api/v1/announcement/admin/update",
		data: input,
	});
	if (!data) throw new Error("announcement missing");
	return data;
}

export async function deleteAnnouncement(uuid: string): Promise<void> {
	await apiClient.post({
		url: "/api/v1/announcement/admin/delete",
		data: { uuid },
	});
}
