import apiClient from "./apiClient";

export interface PermissionItem {
	id: number;
	code: string;
	name: string;
	description: string;
	created_at: string;
}

export async function listPermissions(): Promise<PermissionItem[]> {
	const data = await apiClient.post<PermissionItem[]>({
		url: "/api/v1/permission/list",
	});

	return data || [];
}
