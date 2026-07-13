import type { AxiosResponse } from "axios";
import type { Result } from "./apiClient";
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
	const res = (await apiClient.post({
		url: "/api/v1/permission/list",
	})) as AxiosResponse<Result<PermissionItem[]>>;

	return (res as any).data.data || [];
}

export async function listRoles(): Promise<RoleItem[]> {
	const res = (await apiClient.post({
		url: "/api/v1/role/list",
	})) as AxiosResponse<Result<RoleItem[]>>;

	return (res as any).data.data || [];
}

export async function getRolePermissions(
	role: string,
): Promise<RolePermissionsData> {
	const res = (await apiClient.post({
		url: "/api/v1/permission/role",
		data: { role },
	})) as AxiosResponse<Result<RolePermissionsData>>;

	return {
		role,
		permissions: (res as any).data.data?.permissions || [],
	};
}

export async function syncRolePermissions(
	role: string,
	permissions: string[],
): Promise<RolePermissionsData> {
	const res = (await apiClient.post({
		url: "/api/v1/permission/sync",
		data: { role, permissions },
	})) as AxiosResponse<Result<RolePermissionsData>>;

	if (!(res as any).data.data)
		throw new Error("role permissions data is missing");
	return (res as any).data.data;
}

export async function createRole(name: string): Promise<RoleItem> {
	const res = (await apiClient.post({
		url: "/api/v1/role/create",
		data: { name },
	})) as AxiosResponse<Result<RoleItem>>;

	if (!(res as any).data.data) throw new Error("role data is missing");
	return (res as any).data.data;
}

export async function deleteRole(id: number): Promise<void> {
	await apiClient.post({
		url: "/api/v1/role/delete",
		data: { id },
	});
}

export async function updateRole(id: number, name: string): Promise<RoleItem> {
	const res = (await apiClient.post({
		url: "/api/v1/role/update",
		data: { id, name },
	})) as AxiosResponse<Result<RoleItem>>;

	if (!(res as any).data.data) throw new Error("role data is missing");
	return (res as any).data.data;
}
