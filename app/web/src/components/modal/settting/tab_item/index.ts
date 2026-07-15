import account from "./account";
import appearance from "./appearance";
import audio from "./audio";
import type { SettingTabConfig } from "./types";
import voice from "./voice";

export const TABS: SettingTabConfig[] = [audio, voice, appearance, account];

export type { SettingTabConfig };
