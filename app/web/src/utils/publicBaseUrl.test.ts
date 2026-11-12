import { beforeEach, describe, expect, it, vi } from "vitest";

/**
 * 分享基址的双实现读路径：
 *   - 首帧同步取 localStorage 镜像（桌面端=宿主地址的镜像，浏览器=唯一存储）；
 *   - `loadPublicBaseUrl()` 再向权威值校正；
 *   - 都没有时退回页面 origin。
 */
const STORAGE_KEY = "gospeak.server.baseUrl";

async function importModule() {
	vi.resetModules();
	return import("./publicBaseUrl");
}

beforeEach(() => {
	localStorage.clear();
});

describe("publicBaseUrl / 本地镜像", () => {
	it("首帧直接从镜像读到已保存的地址，不用等 IPC", async () => {
		localStorage.setItem(STORAGE_KEY, "https://voice.example.com");
		const mod = await importModule();
		expect(mod.publicBaseUrl()).toBe("https://voice.example.com");
	});

	it("未配置时回退到页面 origin", async () => {
		const mod = await importModule();
		expect(mod.publicBaseUrl()).toBe(window.location.origin);
	});

	it("loadPublicBaseUrl 校正缓存（权威值晚到也能追上）", async () => {
		const mod = await importModule();
		expect(mod.publicBaseUrl()).toBe(window.location.origin);

		// 模拟宿主在页面加载后才给出地址：读一次即以它为准。
		localStorage.setItem(STORAGE_KEY, "https://late.example.com/");
		await mod.loadPublicBaseUrl();

		// 末尾斜杠会被去掉，分享链接不会出现双斜杠。
		expect(mod.publicBaseUrl()).toBe("https://late.example.com");
	});
});
