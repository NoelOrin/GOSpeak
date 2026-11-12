import apiClient from "./apiClient";

export interface UserGroup {
	id: number;
	user_id: number;
	group_name: string;
	created_at: string;
	updated_at: string;
}

export async function listUserGroups(): Promise<UserGroup[]> {
	const data = await apiClient.post<{ groups: UserGroup[] }>({
		url: "/api/v1/user-group/list",
	});

	return data?.groups ?? [];
}

export async function createUserGroup(group_name: string): Promise<UserGroup> {
	const data = await apiClient.post<UserGroup>({
		url: "/api/v1/user-group/create",
		data: { group_name },
	});

	if (!data) throw new Error("create failed");
	return data;
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
