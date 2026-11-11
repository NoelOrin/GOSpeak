import { create } from "zustand";
import { getProfile, logout as logoutApi } from "@/api/auth";
import {
	isAuthFailure,
	readSessionExpiry,
	recordSessionExpiry as persistSessionExpiry,
	refreshSession,
} from "@/api/authTransport";

export interface UserInfo {
	id: number;
	uuid: string;
	name: string;
	display_name: string;
	avatar: string;
	role: string;
	permissions?: string[];
}

const STORAGE_USER = "user";
const EXPIRE_MARGIN_MS = 60_000;
const UNVERIFIED_GRACE_MS = 10 * 60_000;

interface UserState {
	user: UserInfo | null;
	userStale: boolean;
	login: (u: UserInfo, expiresIn?: number) => void;
	fetchProfile: () => Promise<boolean>;
	clearAuth: () => void;
	logout: () => Promise<void>;
	ensureSession: () => Promise<boolean>;
	recordSessionExpiry: (expiresIn: number | null | undefined) => void;
}

let ensureSessionPromise: Promise<boolean> | null = null;
let sessionExpiresAt: number | null = readSessionExpiry();
let lastVerifiedAt = 0;

function isRateLimitedError(e: unknown): boolean {
	const code = (e as { response?: { data?: { code?: number } } })?.response
		?.data?.code;
	return code === 1017;
}

function readCachedUser(): UserInfo | null {
	try {
		const cached = localStorage.getItem(STORAGE_USER);
		return cached ? (JSON.parse(cached) as UserInfo) : null;
	} catch {
		localStorage.removeItem(STORAGE_USER);
		return null;
	}
}

export const useUserStore = create<UserState>((set, get) => ({
	user: readCachedUser(),
	userStale: false,

	login: (u, expiresIn) => {
		localStorage.setItem(STORAGE_USER, JSON.stringify(u));
		persistSessionExpiry(expiresIn);
		sessionExpiresAt = readSessionExpiry();
		set({ user: u, userStale: false });
	},

	fetchProfile: async () => {
		try {
			const profile = await getProfile();
			const updated = { ...get().user, ...profile } as UserInfo;
			localStorage.setItem(STORAGE_USER, JSON.stringify(updated));
			set({ user: updated, userStale: false });
			return true;
		} catch {
			set({ userStale: true });
			return false;
		}
	},

	clearAuth: () => {
		localStorage.removeItem(STORAGE_USER);
		persistSessionExpiry(null);
		sessionExpiresAt = null;
		lastVerifiedAt = 0;
		set({ user: null, userStale: false });
	},

	logout: async () => {
		try {
			await logoutApi();
		} catch {
			// 即使服务端登出失败，也继续清除本地状态
		}
		get().clearAuth();
	},

	recordSessionExpiry: (expiresIn) => {
		persistSessionExpiry(expiresIn);
		sessionExpiresAt = readSessionExpiry();
	},

	/**
	 * 确保会话可用（过期时间驱动的懒续期）：
	 * - 有过期时间且距过期 >60s：同步放行，零网络请求
	 * - 临近过期/已过期：先 refresh（记录新 expires_in）再重验 profile
	 * - 无过期时间（存量/被清）：宽限窗口(10min)内验证过则放行，否则探测 profile 补齐
	 * - refresh 被限流(1017)不视为鉴权失败：有缓存会话则沿用，不清不踢
	 * 注意：快速路径（未过期同步放行）会让 async IIFE 同步完成，
	 * 清理必须挂在外层 promise 的 .finally（微任务时机必然晚于赋值）。
	 */
	ensureSession: () => {
		if (ensureSessionPromise) return ensureSessionPromise;
		const attempt = (async () => {
			try {
				sessionExpiresAt = readSessionExpiry();
				const { user } = get();
				if (user) {
					if (sessionExpiresAt) {
						if (Date.now() < sessionExpiresAt - EXPIRE_MARGIN_MS) return true;
						get().recordSessionExpiry(await refreshSession());
						const ok = await get().fetchProfile();
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
				if (await get().fetchProfile()) {
					lastVerifiedAt = Date.now();
					if (!sessionExpiresAt) {
						// 后台学习真实过期时间（如 401 已被拦截器消化，这里感知不到 expires_in），
						// 不阻塞导航；失败静默，宽限窗口兜底。
						void refreshSession()
							.then(() => {
								sessionExpiresAt = readSessionExpiry();
							})
							.catch(() => {});
					}
					return true;
				}
				get().recordSessionExpiry(await refreshSession());
				const ok = await get().fetchProfile();
				if (ok) lastVerifiedAt = Date.now();
				return ok;
			} catch (e) {
				if (isRateLimitedError(e) && get().user) return true;
				// 网络中断 / 超时 / 5xx：会话未必失效，保留缓存用户继续用，
				// 下次请求仍会走拦截器重试或刷新。
				if (!isAuthFailure(e) && get().user) return true;
				get().clearAuth();
				return false;
			}
		})();
		const settled = attempt.finally(() => {
			if (ensureSessionPromise === settled) ensureSessionPromise = null;
		});
		ensureSessionPromise = settled;
		return settled;
	},
}));

export const userStore = {
	/** 非 React 侧读取（路由守卫、拦截器等）；组件内请用 useUserStore */
	user: () => useUserStore.getState().user,
	userStale: () => useUserStore.getState().userStale,
	isLoggedIn: () => !!useUserStore.getState().user,
	hasSession: () => !!useUserStore.getState().user,
	ensureSession: () => useUserStore.getState().ensureSession(),
	login: useUserStore.getState().login,
	logout: useUserStore.getState().logout,
	clearAuth: useUserStore.getState().clearAuth,
	fetchProfile: useUserStore.getState().fetchProfile,
	recordSessionExpiry: useUserStore.getState().recordSessionExpiry,
};

export default userStore;
