import type { ComponentType } from "react";

export type SettingTabId =
	| "audio"
	| "voice"
	| "appearance"
	| "account"
	| "desktop"
	| "about";

export interface SettingTabConfig {
	id: SettingTabId;
	label: string;
	/** lucide icon name key used by modal sidebar */
	icon: "mic" | "volume" | "palette" | "user" | "monitor" | "info";
	component: ComponentType;
}
