import apiClient from "./apiClient";

export interface Role {
	id: number;
	name: string;
	created_at: string;
	updated_at: string;
}

export async function listRoles(): Promise<Role[]> {
	return (await apiClient.post<Role[]>({ url: "/api/v1/role/list" })) || [];
}

export async function createRole(name: string): Promise<Role> {
	const data = await apiClient.post<Role>({
		url: "/api/v1/role/create",
		data: { name },
	});
	if (!data) throw new Error("create role response is missing");
	return data;
}

export async function deleteRole(id: number): Promise<void> {
	await apiClient.post({
		url: "/api/v1/role/delete",
		data: { id },
	});
}

export async function getRolePermissions(
	role: string,
): Promise<{ role: string; permissions: string[] }> {
	const data = await apiClient.post<{ role: string; permissions: string[] }>({
		url: "/api/v1/permission/role",
		data: { role },
	});
	if (!data) throw new Error("role permissions response is missing");
	return data;
}

export async function syncRolePermissions(
	role: string,
	permissions: string[],
): Promise<void> {
	await apiClient.post({
		url: "/api/v1/permission/sync",
		data: { role, permissions },
	});
}
