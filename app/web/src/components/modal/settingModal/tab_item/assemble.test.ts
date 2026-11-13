import { describe, expect, it } from "vitest";
import { assembleTabs } from "./assemble";
import type { SettingTabConfig } from "./types";

const tab = (id: SettingTabConfig["id"]): SettingTabConfig => ({
	id,
	label: id,
	icon: "info",
	component: () => null,
});

const BASE = [
	tab("audio"),
	tab("voice"),
	tab("appearance"),
	tab("account"),
	tab("about"),
];

describe("assembleTabs", () => {
	it("Web 构建不含桌面端 tab", () => {
		const tabs = assembleTabs(BASE, tab("desktop"), false);
		expect(tabs.map((t) => t.id)).toEqual([
			"audio",
			"voice",
			"appearance",
			"account",
			"about",
		]);
	});

	it("桌面构建在末尾追加桌面端 tab", () => {
		const tabs = assembleTabs(BASE, tab("desktop"), true);
		expect(tabs.at(-1)?.id).toBe("desktop");
	});

	it("两种构建都保留「关于」", () => {
		for (const desktopBuild of [false, true]) {
			const ids = assembleTabs(BASE, tab("desktop"), desktopBuild).map(
				(t) => t.id,
			);
			expect(ids).toContain("about");
		}
	});

	it("不修改传入的 base 数组", () => {
		const base = [tab("audio")];
		assembleTabs(base, tab("desktop"), true);
		expect(base).toHaveLength(1);
	});
});
