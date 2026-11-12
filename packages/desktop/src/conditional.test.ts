import { describe, expect, it } from "vitest";
import {
	BUILD_TARGET,
	DESKTOP_BUILD,
	WEB_BUILD,
	assertDesktop,
	desktopOnly,
	ifDesktop,
	importDesktop,
	importWeb,
	inDesktop,
	webOnly,
} from "./conditional";
import { isDesktop } from "./runtime";

// 测试目标由 vitest.config.ts 依据 GOSPEAK_TARGET 注入（默认 web）。
// 包内不含 @types/node，用 globalThis 取 process，避免 TS2580。
const expectedTarget =
	(globalThis as { process?: { env?: Record<string, string | undefined> } })
		.process?.env?.GOSPEAK_TARGET === "desktop"
		? "desktop"
		: "web";

describe("conditional / 构建目标常量", () => {
	it("BUILD_TARGET 与注入的 GOSPEAK_TARGET 一致", () => {
		expect(BUILD_TARGET).toBe(expectedTarget);
	});

	it("DESKTOP_BUILD / WEB_BUILD 互斥且自洽", () => {
		expect(DESKTOP_BUILD).toBe(expectedTarget === "desktop");
		expect(WEB_BUILD).toBe(!DESKTOP_BUILD);
	});
});

describe("conditional / 运行时兼容", () => {
	it("inDesktop：桌面构建恒 true，否则等于运行时 isDesktop()", () => {
		if (DESKTOP_BUILD) {
			expect(inDesktop()).toBe(true);
		} else {
			// node 测试环境无 window，isDesktop() 为 false
			expect(inDesktop()).toBe(isDesktop());
		}
	});

	it("assertDesktop：Web 构建中抛错，桌面构建中放行", () => {
		if (DESKTOP_BUILD) {
			expect(() => assertDesktop("overlay")).not.toThrow();
		} else {
			expect(() => assertDesktop("overlay")).toThrow(/\[gospeak\/desktop\]/);
		}
	});
});

describe("conditional / 分支工具", () => {
	it("ifDesktop 只执行当前构建目标的分支", () => {
		const result = ifDesktop(
			() => "desktop-branch",
			() => "web-branch",
		);
		expect(result).toBe(DESKTOP_BUILD ? "desktop-branch" : "web-branch");
	});

	it("desktopOnly：Web 构建返回 fallback，桌面构建返回实现", () => {
		const result = desktopOnly(
			() => "native",
			() => "fallback",
		);
		expect(result).toBe(DESKTOP_BUILD ? "native" : "fallback");
		expect(desktopOnly(() => "native")).toBe(
			DESKTOP_BUILD ? "native" : undefined,
		);
	});

	it("webOnly：桌面构建返回 fallback，Web 构建返回实现", () => {
		const result = webOnly(
			() => "browser",
			() => "fallback",
		);
		expect(result).toBe(DESKTOP_BUILD ? "fallback" : "browser");
	});

	it("importDesktop / importWeb：非目标构建不执行 loader", async () => {
		let desktopLoaded = false;
		let webLoaded = false;
		const desktopLoader = async () => {
			desktopLoaded = true;
			return { marker: "desktop" };
		};
		const webLoader = async () => {
			webLoaded = true;
			return { marker: "web" };
		};

		const d = await importDesktop(desktopLoader);
		const w = await importWeb(webLoader);

		if (DESKTOP_BUILD) {
			expect(d).toEqual({ marker: "desktop" });
			expect(desktopLoaded).toBe(true);
			expect(w).toBeNull();
			expect(webLoaded).toBe(false);
		} else {
			expect(d).toBeNull();
			expect(desktopLoaded).toBe(false);
			expect(w).toEqual({ marker: "web" });
			expect(webLoaded).toBe(true);
		}
	});
});
