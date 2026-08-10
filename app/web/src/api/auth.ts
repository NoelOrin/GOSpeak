import apiClient from "./apiClient";

export interface LoginReq {
	username: string;
	password: string;
}

export interface BackendUser {
	id: number;
	uuid: string;
	name: string;
	display_name: string;
	avatar: string;
	role: string;
	is_bot?: boolean;
	permissions?: string[];
}

export interface LoginData {
	user: BackendUser;
	need_change_password: boolean;
}

export async function login(req: LoginReq): Promise<LoginData> {
	const data = await apiClient.post<LoginData>({
		url: "/api/v1/auth/login",
		data: req,
	});

	if (!data) throw new Error("login data is missing");
	return data;
}

export async function logout(): Promise<void> {
	await apiClient.post({
		url: "/api/v1/auth/logout",
	});
}

export async function changePassword(
	oldPassword: string,
	newPassword: string,
): Promise<void> {
	await apiClient.post({
		url: "/api/v1/auth/change_password",
		data: { old_password: oldPassword, new_password: newPassword },
	});
}

export async function firstChangePassword(
	newPassword: string,
	name?: string,
): Promise<LoginData> {
	const data = await apiClient.post<LoginData>({
		url: "/api/v1/auth/first_change_password",
		data: { new_password: newPassword, name: name || undefined },
	});

	if (!data) throw new Error("login data is missing");
	return data;
}

export async function resetPassword(
	email: string,
	code: string,
	newPassword: string,
): Promise<void> {
	await apiClient.post({
		url: "/api/v1/auth/reset_password",
		data: { email, code, new_password: newPassword },
	});
}

export async function getProfile(): Promise<BackendUser> {
	const data = await apiClient.post<BackendUser>({
		url: "/api/v1/user/profile",
	});

	if (!data) throw new Error("profile data is missing");
	return data;
}
