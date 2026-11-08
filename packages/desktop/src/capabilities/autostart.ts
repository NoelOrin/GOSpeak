import { bridge, command } from "../bridge";
import { isDesktop } from "../runtime";

/** Control whether GOSpeak launches on OS login. */
export interface AutoStart {
	isEnabled(): Promise<boolean>;
	enable(): Promise<void>;
	disable(): Promise<void>;
}

const webFallback: AutoStart = {
	async isEnabled() {
		return false;
	},
	async enable() {},
	async disable() {},
};

const native: AutoStart = {
	isEnabled: () => bridge.invoke<boolean>(command("AutoStart", "IsEnabled")),
	enable: () => bridge.invoke(command("AutoStart", "Enable")),
	disable: () => bridge.invoke(command("AutoStart", "Disable")),
};

/** Returns native auto-start control inside the desktop host, web fallback otherwise. */
export function createAutoStart(): AutoStart {
	return isDesktop() ? native : webFallback;
}
