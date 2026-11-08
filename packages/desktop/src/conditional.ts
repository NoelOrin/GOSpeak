import { isDesktop } from "./runtime";

/**
 * 构建期条件编译工具集。
 *
 * 两个层次，按需选用：
 *
 * 1. **编译期常量**（保证摇树）：`__GOSPEAK_DESKTOP__` / `__GOSPEAK_TARGET__`
 *    由打包器 `define` 注入字面量，在**调用处**写 `if` 时同一模块内即可常量折叠，
 *    死分支连同其中的 `import()` / 未使用静态 import 一并被剔除——
 *    桌面独享模块在 Web 构建里不进包、不产出 chunk。
 * 2. **运行时检测**（兼容兜底）：`isDesktop()` 检测 MyGo 注入的 `window.mygo`，
 *    用于「Web 构建被桌面宿主打开」的 dev 场景与能力 SDK 的浏览器降级。
 *
 * `inDesktop()` 是两者的合成：编译目标为桌面 → 恒 true（打包后折叠）；
 * 否则回退到运行时检测。UI 条件渲染用它，模块级裁剪用 token。
 */

export type BuildTarget = "web" | "desktop";

/**
 * 当前构建目标。未注入 token 的环境（tsc / vitest / Node 直跑源码）按 `"web"` 兜底，
 * 因此本包在任何非打包上下文中都安全可用。
 */
export const BUILD_TARGET: BuildTarget =
	typeof __GOSPEAK_TARGET__ === "undefined" ? "web" : __GOSPEAK_TARGET__;

/** 是否为桌面构建（编译期常量；配合 define 可被折叠为 true/false 字面量）。 */
export const DESKTOP_BUILD: boolean = BUILD_TARGET === "desktop";

/** 是否为 Web 构建（`!DESKTOP_BUILD` 的语义化别名）。 */
export const WEB_BUILD: boolean = !DESKTOP_BUILD;

/**
 * 综合判断当前是否处于桌面环境：桌面构建恒 true；Web 构建回退到运行时
 * MyGo 检测（dev server 被宿主打开时依然成立）。用于 UI 条件渲染。
 */
export function inDesktop(): boolean {
	if (DESKTOP_BUILD) return true;
	return isDesktop();
}

/**
 * 开发期守卫：在 Web 构建中调用桌面独享能力时抛出可读错误，
 * 把「模块裁剪遗漏」暴露在开发阶段而不是静默降级。
 */
export function assertDesktop(feature: string): void {
	if (!inDesktop()) {
		throw new Error(
			`[gospeak/desktop] "${feature}" 仅在桌面端可用（当前为 Web 构建且无 MyGo 宿主）。` +
				`请用 __GOSPEAK_DESKTOP__ / inDesktop() 守卫调用点，或改用桌面构建（GOSPEAK_TARGET=desktop）。`,
		);
	}
}

/**
 * 同步二选一。两个分支都会出现在调用处，但死分支**不会执行**；
 * 若需要死分支代码不进包（含其中的 import），请在调用处直接写
 * `if (__GOSPEAK_DESKTOP__)` / `if (DESKTOP_BUILD)`，或内联三元。
 */
export function ifDesktop<T>(desktop: () => T, web: () => T): T {
	return __GOSPEAK_DESKTOP__ ? desktop() : web();
}

/**
 * 桌面独享取值：Web 构建中返回 fallback（未提供则为 undefined），桌面分支不执行。
 */
export function desktopOnly<T>(
	desktop: () => T,
	fallback?: () => T,
): T | undefined {
	return __GOSPEAK_DESKTOP__ ? desktop() : fallback?.();
}

/**
 * Web 独享取值：桌面构建中返回 fallback（未提供则为 undefined），Web 分支不执行。
 */
export function webOnly<T>(web: () => T, fallback?: () => T): T | undefined {
	return __GOSPEAK_DESKTOP__ ? fallback?.() : web();
}

/**
 * 桌面独享的动态导入。Web 构建中**不会执行** loader，返回 null；
 * 注意：要让对应 chunk 在 Web 构建中不产出，请直接在调用处写
 * `__GOSPEAK_DESKTOP__ ? import("./x") : null`（import() 表达式须位于死分支内）。
 */
export async function importDesktop<M>(
	loader: () => Promise<M>,
): Promise<M | null> {
	if (!__GOSPEAK_DESKTOP__) return null;
	return loader();
}

/**
 * Web 独享的动态导入。桌面构建中不会执行 loader，返回 null。
 */
export async function importWeb<M>(
	loader: () => Promise<M>,
): Promise<M | null> {
	if (__GOSPEAK_DESKTOP__) return null;
	return loader();
}
