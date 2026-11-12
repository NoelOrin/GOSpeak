import { bridge, command } from "../bridge";
import { isDesktop } from "../runtime";

/** Native appearance of the window chrome, following the app's theme. */
export type Appearance = "light" | "dark" | "system";

/** Native window controls for the desktop client. */
export interface WindowControls {
	minimize(): Promise<void>;
	maximize(): Promise<void>;
	unmaximize(): Promise<void>;
	isMaximized(): Promise<boolean>;
	setAlwaysOnTop(value: boolean): Promise<void>;
	setTitle(title: string): Promise<void>;
	/** 让原生窗口按钮与菜单跟随应用主题（桌面端有效，浏览器为空操作）。 */
	setAppearance(theme: Appearance): Promise<void>;
	close(): Promise<void>;
}

const webFallback: WindowControls = {
	async minimize() {},
	async maximize() {},
	async unmaximize() {},
	async isMaximized() {
		return false;
	},
	async setAlwaysOnTop() {},
	async setTitle(title) {
		document.title = title;
	},
	async setAppearance() {},
	async close() {
		window.close();
	},
};

const native: WindowControls = {
	minimize: () => bridge.invoke(command("Window", "Minimize")),
	maximize: () => bridge.invoke(command("Window", "Maximize")),
	unmaximize: () => bridge.invoke(command("Window", "Unmaximize")),
	isMaximized: () => bridge.invoke<boolean>(command("Window", "IsMaximized")),
	setAlwaysOnTop: (value) =>
		bridge.invoke(command("Window", "SetAlwaysOnTop"), value),
	setTitle: (title) => bridge.invoke(command("Window", "SetTitle"), title),
	setAppearance: (theme) =>
		bridge.invoke(command("Window", "SetAppearance"), theme),
	close: () => bridge.invoke(command("Window", "Close")),
};

/** Returns native window controls inside the desktop host, web fallback otherwise. */
export function createWindowControls(): WindowControls {
	return isDesktop() ? native : webFallback;
}
