import type { AxiosResponse } from "axios";
import type { Result } from "./apiClient";
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
): Promise<{ users: BackendUser[]; total: number }> {
	const res = (await apiClient.post({
		url: "/api/v1/user/list",
		data: { page, page_size: pageSize },
	})) as AxiosResponse<Result<UserListResponse>>;

	const result = res.data;
	if (result.code !== 0) throw new Error(result.msg);
	return { users: result.data?.list || [], total: result.data?.total || 0 };
}

/** 更新当前用户资料 */
export async function updateProfile(data: {
	display_name: string;
	avatar: string;
}): Promise<BackendUser> {
	const res = (await apiClient.post({
		url: "/api/v1/user/update-profile",
		data,
	})) as AxiosResponse<Result<BackendUser>>;

	const result = res.data;
	if (result.code !== 0) throw new Error(result.msg);
	if (!result.data) throw new Error("profile data is missing");
	return result.data;
}

export async function fetchUserInfo(identity: string): Promise<BackendUser> {
	const res = (await apiClient.post({
		url: "/api/v1/user/info",
		data: { identity },
	})) as AxiosResponse<Result<BackendUser>>;

	const result = res.data;
	if (result.code !== 0) throw new Error(result.msg);
	if (!result.data) throw new Error("user not found");
	return result.data;
}

/** 获取预设头像列表 */
export async function getPresetAvatars(): Promise<string[]> {
	const res = (await apiClient.get({
		url: "/api/v1/user/preset-avatars",
	})) as AxiosResponse<Result<{ avatars: string[] }>>;

	const result = res.data;
	if (result.code !== 0) throw new Error(result.msg);
	return result.data?.avatars || [];
}

export async function updateUserRole(
	userId: number,
	role: string,
): Promise<void> {
	const res = (await apiClient.post({
		url: "/api/v1/user/update-role",
		data: { id: userId, role },
	})) as AxiosResponse<Result>;
	const result = res.data;
	if (result.code !== 0) throw new Error(result.msg);
}

export async function deleteUser(userId: number): Promise<void> {
	const res = (await apiClient.post({
		url: "/api/v1/user/delete",
		data: { id: userId },
	})) as AxiosResponse<Result>;
	const result = res.data;
	if (result.code !== 0) throw new Error(result.msg);
}
