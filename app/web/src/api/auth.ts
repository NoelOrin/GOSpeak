import apiClient from "./apiClient";
import { requestAccessTokenByRefreshToken } from "./authTransport";

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
}

export interface LoginData {
	access_token: string;
	refresh_token: string;
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

export async function refreshToken(refreshToken: string): Promise<string> {
	return requestAccessTokenByRefreshToken(refreshToken);
}

export async function logout(refreshToken?: string): Promise<void> {
	await apiClient.post({
		url: "/api/v1/auth/logout",
		data: { refresh_token: refreshToken || "" },
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
