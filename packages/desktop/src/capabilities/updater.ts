import { bridge, command } from "../bridge";
import { isDesktop } from "../runtime";

export interface UpdateInfo {
	version: string;
	notes?: string;
}

export interface Updater {
	/** Check for a newer release. Resolves null when up to date. */
	check(): Promise<UpdateInfo | null>;
	/** Download + install the pending update (usually triggers a restart). */
	install(): Promise<void>;
	/** Fires as download progress goes 0..1. */
	onProgress(handler: (fraction: number) => void): () => void;
}

const webFallback: Updater = {
	async check() {
		return null;
	},
	async install() {},
	onProgress() {
		return () => {};
	},
};

const native: Updater = {
	check: () => bridge.invoke<UpdateInfo | null>(command("Updater", "Check")),
	install: () => bridge.invoke(command("Updater", "Install")),
	onProgress: (handler) =>
		bridge.listen("updater.progress", (p) => handler(p as number)),
};

/** Returns native updater inside the desktop host, web fallback otherwise. */
export function createUpdater(): Updater {
	return isDesktop() ? native : webFallback;
}
