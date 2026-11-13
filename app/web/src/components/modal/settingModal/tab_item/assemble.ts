import type { SettingTabConfig } from "./types";

/**
 * 设置页 tab 表装配（纯函数，便于单测）。
 *
 * `desktopBuild` 在真实的调用点传 `__GOSPEAK_DESKTOP__` 字面量，因此这里
 * 的桌面分支在 Web 构建中会被常量折叠成死分支，`desktopTab` 这个引用随之
 * 被摇树剔除（连同它依赖的原生能力模块）。把规则单独抽出来只是为了让
 * 「哪些 tab 在哪个构建里注册」能在不加载音频/原生依赖的前提下被测到。
 */
export function assembleTabs(
	base: SettingTabConfig[],
	desktopTab: SettingTabConfig,
	desktopBuild: boolean,
): SettingTabConfig[] {
	return desktopBuild ? [...base, desktopTab] : base;
}
