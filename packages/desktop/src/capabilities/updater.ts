import { bridge, command } from "../bridge";
import { isDesktop } from "../runtime";

export interface UpdateInfo {
	/** A newer release was found by the last check. */
	available: boolean;
	/** Version of the newer release ("" when up to date). */
	version?: string;
	/** Release notes in Markdown ("" when up to date). */
	notes?: string;
	/** Version of the running build. */
	current: string;
	/**
	 * This build can self-update: false for dev builds and releases built
	 * before `updates` was configured in the MyGo host (mygo.config.ts).
	 * When false, offer openReleasePage() instead of install().
	 */
	supported: boolean;
}

export interface Updater {
	/** Check the release feed for a newer version. */
	check(): Promise<UpdateInfo>;
	/**
	 * Download, verify and install the update found by check(), then the
	 * host relaunches into the new version. Only valid when the last
	 * check() returned available=true and supported=true.
	 */
	install(): Promise<void>;
	/** Fires as download progress goes 0..1. */
	onProgress(handler: (fraction: number) => void): () => void;
	/** Version of the running build ("dev" when run from source). */
	currentVersion(): Promise<string>;
	/**
	 * Open the GitHub releases page in the system browser — the fallback
	 * for builds that cannot self-update (supported=false).
	 */
	openReleasePage(): Promise<void>;
}

const webFallback: Updater = {
	async check() {
		return {
			available: false,
			current: "dev",
			supported: false,
		};
	},
	async install() {},
	onProgress() {
		return () => {};
	},
	async currentVersion() {
		return "dev";
	},
	async openReleasePage() {},
};

const native: Updater = {
	check: () => bridge.invoke<UpdateInfo>(command("Updater", "Check")),
	install: () => bridge.invoke(command("Updater", "Install")),
	onProgress: (handler) =>
		bridge.listen("updater.progress", (p) => handler(p as number)),
	currentVersion: () =>
		bridge.invoke<string>(command("Updater", "CurrentVersion")),
	openReleasePage: () => bridge.invoke(command("Updater", "OpenReleasePage")),
};

/** Returns native updater inside the desktop host, web fallback otherwise. */
export function createUpdater(): Updater {
	return isDesktop() ? native : webFallback;
}
