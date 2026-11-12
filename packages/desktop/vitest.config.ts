import { defineConfig } from "vitest/config";

/**
 * 条件编译常量随 GOSPEAK_TARGET 注入：
 *   vitest run                          → web 目标（默认）
 *   GOSPEAK_TARGET=desktop vitest run   → desktop 目标
 */
export default defineConfig(() => {
	const target = process.env.GOSPEAK_TARGET === "desktop" ? "desktop" : "web";
	return {
		define: {
			__GOSPEAK_TARGET__: JSON.stringify(target),
			__GOSPEAK_DESKTOP__: String(target === "desktop"),
		},
		test: {
			environment: "node",
		},
	};
});
