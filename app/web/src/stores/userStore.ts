import { createSignal } from "solid-js";
import { getProfile, logout as logoutApi } from "@/api/auth";
import {
	readSessionExpiry,
	recordSessionExpiry,
	refreshSession,
} from "@/api/authTransport";

export interface UserInfo {
	id: number;
	uuid: string;
	name: string;
	display_name: string;
	avatar: string;
	role: string;
	is_guest?: boolean;
	permissions?: string[];
}

const STORAGE_USER = "user";
const EXPIRE_MARGIN_MS = 60_000;
const UNVERIFIED_GRACE_MS = 10 * 60_000;

const [user, setUser] = createSignal<UserInfo | null>(null);
const [userStale, setUserStale] = createSignal(false);

let ensureSessionPromise: Promise<boolean> | null = null;

let sessionExpiresAt: number | null = readSessionExpiry();
let lastVerifiedAt = 0;

function syncSessionExpiry() {
	sessionExpiresAt = readSessionExpiry();
}

function recordSessionExpiryAction(expiresIn: number | null | undefined) {
	recordSessionExpiry(expiresIn);
	sessionExpiresAt = readSessionExpiry();
}

// 只缓存非敏感 user 元数据；access/refresh token 完全由 HttpOnly Cookie 承载。
const cachedUser = localStorage.getItem(STORAGE_USER);
if (cachedUser) {
	try {
		setUser(JSON.parse(cachedUser));
	} catch {
		localStorage.removeItem(STORAGE_USER);
	}
}

async function loginAction(u: UserInfo, expiresIn?: number) {
	localStorage.setItem(STORAGE_USER, JSON.stringify(u));
	setUser(u);
	setUserStale(false);
	recordSessionExpiryAction(expiresIn);
}

async function fetchProfileAction(): Promise<boolean> {
	try {
		const profile = await getProfile();
		const updated = { ...user(), ...profile } as UserInfo;
		localStorage.setItem(STORAGE_USER, JSON.stringify(updated));
		setUser(updated);
		setUserStale(false);
		return true;
	} catch {
		setUserStale(true);
		return false;
	}
}

async function clearAuthAction() {
	localStorage.removeItem(STORAGE_USER);
	recordSessionExpiry(null);
	setUser(null);
	setUserStale(false);
	sessionExpiresAt = null;
	lastVerifiedAt = 0;
}

async function logoutAction() {
	try {
		await logoutApi();
	} catch {
		// 即使服务端登出失败，也继续清除本地状态
	}
	await clearAuthAction();
}

/**
 * 确保会话可用（过期时间驱动的懒续期）：
 * - 有过期时间且距过期 >60s：同步放行，零网络请求
 * - 临近过期/已过期：先 refresh（记录新 expires_in）再重验 profile
 * - 无过期时间（存量/被清）：宽限窗口(10min)内验证过则放行，否则探测 profile 补齐
 * - 无用户元数据：冷启动探测（cookie 可能仍有效）
 * - refresh 被限流(1017)不视为鉴权失败：有缓存会话则沿用，不清不踢
 * - refresh 返回真实 token 错误 → 清缓存返回 false，/login 据此停在登录页
 */
function isRateLimitedError(e: unknown): boolean {
	const code = (e as { response?: { data?: { code?: number } } })?.response
		?.data?.code;
	return code === 1017;
}

async function ensureSessionAction(): Promise<boolean> {
	if (ensureSessionPromise) return ensureSessionPromise;
	// 注意：快速路径（未过期同步放行）会让 async IIFE 同步完成，
	// IIFE 内部的 finally 会在赋值完成前执行并被覆盖，导致 promise 永不清空——
	// 清理必须挂在外层 promise 的 .finally（微任务时机必然晚于赋值）。
	const attempt = (async () => {
		try {
			syncSessionExpiry();
			if (user()) {
				if (sessionExpiresAt) {
					if (Date.now() < sessionExpiresAt - EXPIRE_MARGIN_MS) return true;
					recordSessionExpiryAction(await refreshSession());
					const ok = await fetchProfileAction();
					if (ok) lastVerifiedAt = Date.now();
					return ok;
				}
				if (
					lastVerifiedAt &&
					Date.now() - lastVerifiedAt < UNVERIFIED_GRACE_MS
				) {
					return true;
				}
			}
			if (await fetchProfileAction()) {
				lastVerifiedAt = Date.now();
				if (!sessionExpiresAt) {
					// 后台学习真实过期时间（如 401 已被拦截器消化，这里感知不到 expires_in），
					// 不阻塞导航；失败静默，宽限窗口兜底。
					void refreshSession()
						.then(() => syncSessionExpiry())
						.catch(() => {});
				}
				return true;
			}
			recordSessionExpiryAction(await refreshSession());
			const ok = await fetchProfileAction();
			if (ok) lastVerifiedAt = Date.now();
			return ok;
		} catch (e) {
			if (isRateLimitedError(e) && user()) return true;
			await clearAuthAction();
			return false;
		}
	})();
	const settled = attempt.finally(() => {
		if (ensureSessionPromise === settled) ensureSessionPromise = null;
	});
	ensureSessionPromise = settled;
	return settled;
}

const userStore = {
	user,
	userStale,
	/** 本地有用户元数据；真实会话有效性以服务端 Cookie 鉴权为准 */
	isLoggedIn: () => !!user(),
	hasSession: () => !!user(),
	ensureSession: ensureSessionAction,
	login: loginAction,
	logout: logoutAction,
	clearAuth: clearAuthAction,
	fetchProfile: fetchProfileAction,
	recordSessionExpiry: recordSessionExpiryAction,
};

export default userStore;
