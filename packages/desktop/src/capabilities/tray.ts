import { bridge, command } from "../bridge";
import { isDesktop } from "../runtime";

export interface TrayMenuItem {
	id: string;
	label: string;
	/** Sepator entry when true; `id`/`label` ignored. */
	separator?: boolean;
	disabled?: boolean;
}

/** System tray integration for the desktop client. */
export interface SystemTray {
	setTooltip(tooltip: string): Promise<void>;
	setMenu(items: TrayMenuItem[]): Promise<void>;
	/** Fires with the `id` of the clicked menu item. */
	onMenuClick(handler: (id: string) => void): () => void;
	show(): Promise<void>;
	hide(): Promise<void>;
}

const noopUnsub = () => {};

const webFallback: SystemTray = {
	async setTooltip() {},
	async setMenu() {},
	onMenuClick() {
		return noopUnsub;
	},
	async show() {},
	async hide() {},
};

const native: SystemTray = {
	setTooltip: (tooltip) =>
		bridge.invoke(command("Tray", "SetTooltip"), tooltip),
	setMenu: (items) => bridge.invoke(command("Tray", "SetMenu"), items),
	onMenuClick: (handler) =>
		bridge.listen("tray.menuClick", (p) => handler(p as string)),
	show: () => bridge.invoke(command("Tray", "Show")),
	hide: () => bridge.invoke(command("Tray", "Hide")),
};

/** Returns native tray controls inside the desktop host, web fallback otherwise. */
export function createSystemTray(): SystemTray {
	return isDesktop() ? native : webFallback;
}
