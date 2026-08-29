import { beforeEach, describe, expect, it, vi } from "vitest";

const postMock = vi.fn();
vi.mock("axios", () => ({
	default: {
		create: () => ({ post: (...args: unknown[]) => postMock(...args) }),
	},
}));

describe("refreshSession", () => {
	beforeEach(() => {
		vi.resetModules();
		postMock.mockReset();
		localStorage.clear();
	});

	it("透传响应体 expires_in", async () => {
		postMock.mockResolvedValue({
			data: { code: 0, data: { access_token: "x", expires_in: 900 } },
		});
		const { refreshSession } = await import("@/api/authTransport");
		await expect(refreshSession()).resolves.toBe(900);
	});

	it("响应体缺 expires_in：返回 null", async () => {
		postMock.mockResolvedValue({ data: { code: 0, data: {} } });
		const { refreshSession } = await import("@/api/authTransport");
		await expect(refreshSession()).resolves.toBeNull();
	});

	it("并发调用复用同一请求（去重保留）", async () => {
		postMock.mockResolvedValue({
			data: { code: 0, data: { expires_in: 900 } },
		});
		const { refreshSession } = await import("@/api/authTransport");
		const [a, b] = await Promise.all([refreshSession(), refreshSession()]);
		expect(postMock).toHaveBeenCalledTimes(1);
		expect(a).toBe(900);
		expect(b).toBe(900);
	});

	it("refresh 成功持久化 expires_in 到 localStorage", async () => {
		postMock.mockResolvedValue({
			data: { code: 0, data: { expires_in: 900 } },
		});
		const { refreshSession } = await import("@/api/authTransport");
		await refreshSession();
		expect(
			Number(localStorage.getItem("gospeak_session_expires_at")),
		).toBeGreaterThan(Date.now());
	});

	it("响应体缺 expires_in：清除持久化的过期时间", async () => {
		localStorage.setItem(
			"gospeak_session_expires_at",
			String(Date.now() + 900_000),
		);
		postMock.mockResolvedValue({ data: { code: 0, data: {} } });
		const { refreshSession } = await import("@/api/authTransport");
		await refreshSession();
		expect(localStorage.getItem("gospeak_session_expires_at")).toBeNull();
	});
});
