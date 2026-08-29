import { beforeEach, describe, expect, it, vi } from "vitest";

const getProfileMock = vi.fn();
const refreshSessionMock = vi.fn();

vi.mock("@/api/auth", () => ({
	getProfile: (...args: unknown[]) => getProfileMock(...args),
	logout: vi.fn(),
}));
vi.mock("@/api/authTransport", () => ({
	refreshSession: (...args: unknown[]) => refreshSessionMock(...args),
	readSessionExpiry: () => {
		const raw = localStorage.getItem("gospeak_session_expires_at");
		const n = raw ? Number(raw) : NaN;
		return Number.isFinite(n) && n > 0 ? n : null;
	},
	recordSessionExpiry: (expiresIn: number | null | undefined) => {
		if (typeof expiresIn === "number" && expiresIn > 0) {
			localStorage.setItem(
				"gospeak_session_expires_at",
				String(Date.now() + expiresIn * 1000),
			);
			return;
		}
		localStorage.removeItem("gospeak_session_expires_at");
	},
}));

import userStore from "./userStore";

const authError = (code: number) => ({ response: { data: { code } } });

const fakeUser = {
	id: 1,
	uuid: "u-1",
	name: "alice",
	display_name: "Alice",
	avatar: "",
	role: "user",
};

const fakeProfile = { display_name: "Alice II" };

describe("userStore.ensureSession", () => {
	beforeEach(() => {
		localStorage.clear();
		userStore.clearAuth();
		getProfileMock.mockReset();
		refreshSessionMock.mockReset();
	});

	it("缓存用户 + profile 成功：true，过期时间后台学习不阻塞", async () => {
		await userStore.login(fakeUser);
		getProfileMock.mockResolvedValue(fakeProfile);
		refreshSessionMock.mockResolvedValue(900);

		await expect(userStore.ensureSession()).resolves.toBe(true);
		expect(getProfileMock).toHaveBeenCalledTimes(1);
		// profile 先行返回（导航不阻塞），随后后台 refresh 学习过期时间
		await Promise.resolve();
		await Promise.resolve();
		expect(refreshSessionMock).toHaveBeenCalledTimes(1);
	});

	it("access 过期且 refresh 被限流(1017)：沿用缓存会话返回 true，不清会话", async () => {
		await userStore.login(fakeUser);
		getProfileMock.mockRejectedValue(authError(1003));
		refreshSessionMock.mockRejectedValue(authError(1017));

		await expect(userStore.ensureSession()).resolves.toBe(true);
		expect(userStore.user()).not.toBeNull();
		expect(localStorage.getItem("user")).not.toBeNull();
	});

	it("会话真死(refresh 也返回 token 错误)：false 并清缓存，防止 /login 反弹循环", async () => {
		await userStore.login(fakeUser);
		getProfileMock.mockRejectedValue(authError(1003));
		refreshSessionMock.mockRejectedValue(authError(1003));

		await expect(userStore.ensureSession()).resolves.toBe(false);
		expect(userStore.user()).toBeNull();
		expect(localStorage.getItem("user")).toBeNull();
	});

	it("无缓存用户 + refresh 成功：true", async () => {
		getProfileMock
			.mockRejectedValueOnce(authError(1001))
			.mockResolvedValueOnce(fakeProfile);
		refreshSessionMock.mockResolvedValue(undefined);

		await expect(userStore.ensureSession()).resolves.toBe(true);
		expect(refreshSessionMock).toHaveBeenCalledTimes(1);
		expect(userStore.user()).not.toBeNull();
	});

	it("无缓存用户且 refresh 被限流(1017)：false", async () => {
		getProfileMock.mockRejectedValue(authError(1003));
		refreshSessionMock.mockRejectedValue(authError(1017));

		await expect(userStore.ensureSession()).resolves.toBe(false);
	});
});

describe("userStore 懒续期（expires_in 驱动）", () => {
	beforeEach(() => {
		localStorage.clear();
		userStore.clearAuth();
		getProfileMock.mockReset();
		refreshSessionMock.mockReset();
	});

	it("有会话且距过期 >60s：同步放行，零网络请求", async () => {
		await userStore.login(fakeUser, 900);
		expect(
			Number(localStorage.getItem("gospeak_session_expires_at")),
		).toBeGreaterThan(Date.now());
		getProfileMock.mockRejectedValue(new Error("profile should not fire"));
		refreshSessionMock.mockRejectedValue(new Error("refresh should not fire"));

		await expect(userStore.ensureSession()).resolves.toBe(true);
		expect(getProfileMock).not.toHaveBeenCalled();
		expect(refreshSessionMock).not.toHaveBeenCalled();
	});

	it("临近过期(<60s)：先 refresh 再 profile，并记录新过期时间", async () => {
		await userStore.login(fakeUser, 30);
		refreshSessionMock.mockResolvedValue(900);
		getProfileMock.mockResolvedValue(fakeProfile);

		await expect(userStore.ensureSession()).resolves.toBe(true);
		expect(refreshSessionMock).toHaveBeenCalledTimes(1);
		expect(getProfileMock).toHaveBeenCalledTimes(1);
		expect(
			Number(localStorage.getItem("gospeak_session_expires_at")),
		).toBeGreaterThan(Date.now());

		await expect(userStore.ensureSession()).resolves.toBe(true);
		expect(refreshSessionMock).toHaveBeenCalledTimes(1);
	});

	it("无过期时间的存量会话：宽限窗口内验证过则放行", async () => {
		getProfileMock.mockResolvedValue(fakeProfile);
		refreshSessionMock.mockRejectedValue(new Error("refresh should not fire"));

		await expect(userStore.ensureSession()).resolves.toBe(true);
		getProfileMock.mockClear();
		await expect(userStore.ensureSession()).resolves.toBe(true);
		expect(getProfileMock).not.toHaveBeenCalled();
	});

	it("临近过期且 refresh 返回 expires_in：新过期时间持久化", async () => {
		await userStore.login(fakeUser, 30);
		refreshSessionMock.mockResolvedValue(900);
		getProfileMock.mockResolvedValue(fakeProfile);

		await userStore.ensureSession();
		const stored = localStorage.getItem("gospeak_session_expires_at");
		expect(stored).not.toBeNull();
		expect(Number(stored)).toBeGreaterThan(Date.now() + 600_000);
	});

	it("logout 清除过期时间", async () => {
		await userStore.login(fakeUser, 900);
		expect(localStorage.getItem("gospeak_session_expires_at")).not.toBeNull();

		await userStore.logout();
		expect(localStorage.getItem("gospeak_session_expires_at")).toBeNull();
	});

	it("冷启动探测成功且无过期时间：后台 refresh 学习过期时间且不阻塞", async () => {
		getProfileMock.mockResolvedValue(fakeProfile);
		refreshSessionMock.mockImplementation(async () => {
			localStorage.setItem(
				"gospeak_session_expires_at",
				String(Date.now() + 900_000),
			);
			return 900;
		});

		await expect(userStore.ensureSession()).resolves.toBe(true);
		expect(getProfileMock).toHaveBeenCalledTimes(1);
		await Promise.resolve();
		await Promise.resolve();
		expect(refreshSessionMock).toHaveBeenCalledTimes(1);
		expect(
			Number(localStorage.getItem("gospeak_session_expires_at")),
		).toBeGreaterThan(Date.now());

		// 过期时间学到手：后续导航走快速路径，零请求
		getProfileMock.mockClear();
		refreshSessionMock.mockClear();
		await expect(userStore.ensureSession()).resolves.toBe(true);
		expect(getProfileMock).not.toHaveBeenCalled();
		expect(refreshSessionMock).not.toHaveBeenCalled();
	});

	it("拦截器路径 refresh 写入的过期时间：ensureSession 能读到并快速放行", async () => {
		await userStore.login(fakeUser);
		// 模拟 apiClient 拦截器内部 refreshSession 写入持久化（mock 不经过 ensureSession）
		localStorage.setItem(
			"gospeak_session_expires_at",
			String(Date.now() + 900_000),
		);
		getProfileMock.mockRejectedValue(new Error("profile should not fire"));
		refreshSessionMock.mockRejectedValue(new Error("refresh should not fire"));

		await expect(userStore.ensureSession()).resolves.toBe(true);
		expect(getProfileMock).not.toHaveBeenCalled();
		expect(refreshSessionMock).not.toHaveBeenCalled();
	});
});
