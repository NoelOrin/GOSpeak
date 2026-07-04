import { del, get, set } from "idb-keyval";
import { createSignal } from "solid-js";
import { bindAPIClientAuth } from "@/api/apiClientAuth";
import { getProfile, logout as logoutApi } from "@/api/auth";

export interface UserInfo {
	id: number;
	uuid: string;
	name: string;
	display_name: string;
	avatar: string;
	role: string;
}

const STORAGE_USER = "user";
const STORAGE_ACCESS_TOKEN = "accessToken";
const DB_REFRESH_TOKEN = "refreshToken";

const [user, setUser] = createSignal<UserInfo | null>(null);
const [userStale, setUserStale] = createSignal(false);
const [accessToken, setAccessToken] = createSignal("");
const [refreshToken, setRefreshToken] = createSignal("");

// 同步从 localStorage 恢复 user 和 accessToken，保证路由守卫立即可用
const cachedUser = localStorage.getItem(STORAGE_USER);
const cachedToken = localStorage.getItem(STORAGE_ACCESS_TOKEN);
if (cachedUser) setUser(JSON.parse(cachedUser));
if (cachedToken) setAccessToken(cachedToken);

// refreshToken 在 IndexedDB，异步恢复即可
get<string>(DB_REFRESH_TOKEN).then((rt) => {
	if (rt) setRefreshToken(rt);
});

// 解码 JWT payload，返回任意字段；解析失败返回 null
function decodeJWTPayload<T = Record<string, any>>(token: string): T | null {
	try {
		const payload = token.split(".")[1];
		return JSON.parse(atob(payload)) as T;
	} catch {
		return null;
	}
}

function isTokenExpired(token: string): boolean {
	const payload = decodeJWTPayload<{ exp?: number }>(token);
	if (!payload || typeof payload.exp !== "number") return false;
	return Date.now() >= payload.exp * 1000;
}

async function loginAction(u: UserInfo, at: string, rt: string) {
	localStorage.setItem(STORAGE_USER, JSON.stringify(u));
	localStorage.setItem(STORAGE_ACCESS_TOKEN, at);
	await set(DB_REFRESH_TOKEN, rt);
	setUser(u);
	setUserStale(false);
	setAccessToken(at);
	setRefreshToken(rt);
}

async function updateAccessTokenAction(token: string) {
	localStorage.setItem(STORAGE_ACCESS_TOKEN, token);
	setAccessToken(token);
}

async function fetchProfileAction() {
	try {
		const profile = await getProfile();
		const updated = { ...user(), ...profile } as UserInfo;
		localStorage.setItem(STORAGE_USER, JSON.stringify(updated));
		setUser(updated);
		setUserStale(false);
	} catch {
		setUserStale(true);
	}
}

async function clearAuthAction() {
	localStorage.removeItem(STORAGE_USER);
	localStorage.removeItem(STORAGE_ACCESS_TOKEN);
	await del(DB_REFRESH_TOKEN);
	setUser(null);
	setUserStale(false);
	setAccessToken("");
	setRefreshToken("");
}

async function logoutAction() {
	try {
		if (accessToken()) {
			await logoutApi(refreshToken() || undefined);
		}
	} catch {
		// 即使服务端登出失败，也继续清除本地状态
	}
	await clearAuthAction();
}

bindAPIClientAuth({
	getAccessToken: accessToken,
	getRefreshToken: refreshToken,
	updateAccessToken: updateAccessTokenAction,
	clearAuth: clearAuthAction,
});

const userStore = {
	user,
	userStale,
	accessToken,
	refreshToken,
	isLoggedIn: () => {
		const token = accessToken();
		return !!token && !isTokenExpired(token);
	},
	login: loginAction,
	logout: logoutAction,
	clearAuth: clearAuthAction,
	updateAccessToken: updateAccessTokenAction,
	fetchProfile: fetchProfileAction,
};

export default userStore;
