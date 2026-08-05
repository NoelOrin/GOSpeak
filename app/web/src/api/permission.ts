import apiClient from "./apiClient";

export interface PermissionItem {
	id: number;
	code: string;
	name: string;
	description: string;
	created_at: string;
}

export interface RoleItem {
	id: number;
	name: string;
	created_at: string;
	updated_at: string;
}

export interface RolePermissionsData {
	role: string;
	permissions: string[] | null;
}

export async function listPermissions(): Promise<PermissionItem[]> {
	const data = await apiClient.post<PermissionItem[]>({
		url: "/api/v1/permission/list",
	});

	return data || [];
}

export async function listRoles(): Promise<RoleItem[]> {
	const data = await apiClient.post<RoleItem[]>({
		url: "/api/v1/role/list",
	});

	return data || [];
}

export async function getRolePermissions(
	role: string,
): Promise<RolePermissionsData> {
	const data = await apiClient.post<RolePermissionsData>({
		url: "/api/v1/permission/role",
		data: { role },
	});

	return {
		role,
		permissions: data?.permissions || [],
	};
}

export async function syncRolePermissions(
	role: string,
	permissions: string[],
): Promise<RolePermissionsData> {
	const data = await apiClient.post<RolePermissionsData>({
		url: "/api/v1/permission/sync",
		data: { role, permissions },
	});

	if (!data) throw new Error("role permissions data is missing");
	return data;
}

export async function createRole(name: string): Promise<RoleItem> {
	const data = await apiClient.post<RoleItem>({
		url: "/api/v1/role/create",
		data: { name },
	});

	if (!data) throw new Error("role data is missing");
	return data;
}

export async function deleteRole(id: number): Promise<void> {
	await apiClient.post({
		url: "/api/v1/role/delete",
		data: { id },
	});
}

export async function updateRole(id: number, name: string): Promise<RoleItem> {
	const data = await apiClient.post<RoleItem>({
		url: "/api/v1/role/update",
		data: { id, name },
	});

	if (!data) throw new Error("role data is missing");
	return data;
}
