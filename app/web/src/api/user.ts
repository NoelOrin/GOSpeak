import apiClient from "./apiClient";
import type { BackendUser } from "./auth";

interface UserListResponse {
	list: BackendUser[];
	total: number;
	page: number;
}

export async function listUsers(
	page = 1,
	pageSize = 100,
	excludeBots = true,
	keyword?: string,
): Promise<{ users: BackendUser[]; total: number }> {
	const data = await apiClient.post<UserListResponse>({
		url: "/api/v1/user/list",
		data: {
			page,
			page_size: pageSize,
			exclude_bots: excludeBots,
			...(keyword?.trim() ? { keyword: keyword.trim() } : {}),
		},
	});

	return {
		users: data?.list || [],
		total: data?.total || 0,
	};
}

/** 更新当前用户资料 */
export async function updateProfile(data: {
	display_name: string;
	avatar: string;
}): Promise<BackendUser> {
	const profile = await apiClient.post<BackendUser>({
		url: "/api/v1/user/update-profile",
		data,
	});

	if (!profile) throw new Error("profile data is missing");
	return profile;
}

export async function fetchUserInfo(identity: string): Promise<BackendUser> {
	const data = await apiClient.post<BackendUser>({
		url: "/api/v1/user/info",
		data: { identity },
	});

	if (!data) throw new Error("user not found");
	return data;
}

/** 获取预设头像列表 */
export async function getPresetAvatars(): Promise<string[]> {
	const data = await apiClient.get<{ avatars: string[] }>({
		url: "/api/v1/user/preset-avatars",
	});

	return data?.avatars || [];
}

export async function updateUserRole(
	userId: number,
	role: string,
): Promise<void> {
	await apiClient.post({
		url: "/api/v1/user/update-role",
		data: { id: userId, role },
	});
}

export async function deleteUser(userId: number): Promise<void> {
	await apiClient.post({
		url: "/api/v1/user/delete",
		data: { id: userId },
	});
}
