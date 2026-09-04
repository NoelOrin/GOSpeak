import type { Component } from "solid-js";

export type SettingTabId = "audio" | "voice" | "appearance" | "account";

export interface SettingTabConfig {
	id: SettingTabId;
	label: string;
	/** lucide icon name key used by modal sidebar */
	icon: "mic" | "volume" | "palette" | "user";
	component: Component;
}
