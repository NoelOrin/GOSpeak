import { beforeEach, describe, expect, it, vi } from "vitest";

const get = vi.hoisted(() => vi.fn());

vi.mock("./apiClient", () => ({
	default: { get },
}));

import { checkServerUpdate, getServerVersion } from "./system";

beforeEach(() => {
	get.mockReset();
});

describe("checkServerUpdate", () => {
	it("返回后端 payload", async () => {
		const payload = {
			current: "v1.2.0",
			latest: "v1.3.0",
			available: true,
			url: "https://github.com/gospeak/releases/tag/v1.3.0",
		};
		get.mockResolvedValue(payload);

		await expect(checkServerUpdate()).resolves.toEqual(payload);
		expect(get).toHaveBeenCalledWith(
			expect.objectContaining({ url: "/api/v1/system/update/check" }),
		);
	});

	/**
	 * 回归：以前缺失 payload 会被兜底成 { available: false }，监控卡片于是渲染
	 * 「已是最新版本」的成功态——把一次失败的更新检查伪装成「一切正常」。
	 */
	it("payload 缺失时抛错，不伪装成「已是最新」", async () => {
		for (const missing of [null, undefined]) {
			get.mockResolvedValue(missing);
			await expect(checkServerUpdate()).rejects.toThrow();
		}
	});
});

describe("getServerVersion", () => {
	it("返回后端 payload", async () => {
		get.mockResolvedValue({ version: "v1.2.0", commit: "abc1234" });
		await expect(getServerVersion()).resolves.toEqual({
			version: "v1.2.0",
			commit: "abc1234",
		});
	});

	it("payload 缺失时回退成 unknown（版本展示非关键路径）", async () => {
		get.mockResolvedValue(null);
		await expect(getServerVersion()).resolves.toEqual({
			version: "unknown",
			commit: "unknown",
		});
	});
});
