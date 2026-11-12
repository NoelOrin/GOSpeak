import { describe, expect, it } from "vitest";
import { resolveRouteTitle } from "./routeTitle";

const match = (title?: string) => ({
	staticData: title ? { title } : undefined,
});

describe("resolveRouteTitle", () => {
	it("取路由链上最近一级的 staticData.title", () => {
		expect(
			resolveRouteTitle([match("管理"), match("域管理")], "/manage/domains"),
		).toBe("域管理");
	});

	it("staticData 缺失时回退到路径段映射", () => {
		expect(resolveRouteTitle([], "/manage/users")).toBe("用户管理");
	});

	it("动态段不作为标题，回退到父级路径段", () => {
		expect(
			resolveRouteTitle(
				[],
				"/manage/domains/3f2a9c1e-0000-4000-8000-000000000000",
			),
		).toBe("域管理");
	});

	it("完全未知的路径回退到 GOSpeak", () => {
		expect(resolveRouteTitle([], "/unknown-path")).toBe("GOSpeak");
		expect(resolveRouteTitle([], "/")).toBe("GOSpeak");
	});
});
