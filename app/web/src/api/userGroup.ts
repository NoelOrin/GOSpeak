import type { AxiosResponse } from "axios";
import type { Result } from "./apiClient";
import apiClient from "./apiClient";

export interface UserGroup {
	id: number;
	user_id: number;
	group_name: string;
	created_at: string;
	updated_at: string;
}

export async function listUserGroups(): Promise<UserGroup[]> {
	const res = (await apiClient.post({
		url: "/api/v1/user-group/list",
	})) as AxiosResponse<Result<{ groups: UserGroup[] }>>;

	return (res as any).data.data?.groups ?? [];
}

export async function createUserGroup(group_name: string): Promise<UserGroup> {
	const res = (await apiClient.post({
		url: "/api/v1/user-group/create",
		data: { group_name },
	})) as AxiosResponse<Result<UserGroup>>;

	if (!(res as any).data.data) throw new Error("create failed");
	return (res as any).data.data as UserGroup;
}

export async function renameUserGroup(
	id: number,
	group_name: string,
): Promise<void> {
	await apiClient.post({
		url: "/api/v1/user-group/update",
		data: { id, group_name },
	});
}

export async function deleteUserGroup(id: number): Promise<void> {
	await apiClient.post({
		url: "/api/v1/user-group/delete",
		data: { id },
	});
}
