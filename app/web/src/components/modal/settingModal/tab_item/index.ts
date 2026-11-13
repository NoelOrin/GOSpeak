import about from "./about";
import { assembleTabs } from "./assemble";
import account from "./account";
import appearance from "./appearance";
import audio from "./audio";
import desktop from "./desktop";
import type { SettingTabConfig } from "./types";
import voice from "./voice";

// 条件编译（构建期）：`__GOSPEAK_DESKTOP__` 由 vite define 注入为字面量，
// Web 构建中该条件恒 false——`desktop` 模块的绑定仅存在于死分支，
// 会被摇树整体剔除（desktop.tsx 及其原生能力依赖不进 Web 包）。
// 桌面构建中恒 true，tab 直接注册。
const baseTabs: SettingTabConfig[] = [audio, voice, appearance, account, about];

export const TABS: SettingTabConfig[] = assembleTabs(
	baseTabs,
	desktop,
	__GOSPEAK_DESKTOP__,
);

export type { SettingTabConfig };
