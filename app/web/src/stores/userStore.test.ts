import { beforeEach, describe, expect, it, vi } from "vitest";

const getProfileMock = vi.fn();
const refreshSessionMock = vi.fn();

vi.mock("@/api/auth", () => ({
	getProfile: (...args: unknown[]) => getProfileMock(...args),
	logout: vi.fn(),
}));
vi.mock("@/api/authTransport", () => ({
	refreshSession: (...args: unknown[]) => refreshSessionMock(...args),
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

	it("缓存用户 + profile 成功：true，且不发 refresh", async () => {
		await userStore.login(fakeUser);
		getProfileMock.mockResolvedValue(fakeProfile);
		refreshSessionMock.mockRejectedValue(new Error("refresh should not fire"));

		await expect(userStore.ensureSession()).resolves.toBe(true);
		expect(getProfileMock).toHaveBeenCalledTimes(1);
		expect(refreshSessionMock).not.toHaveBeenCalled();
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
});
