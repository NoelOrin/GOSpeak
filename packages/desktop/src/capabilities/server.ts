import { bridge, command } from "../bridge";
import { isDesktop } from "../runtime";

/**
 * Backend base URL the client targets.
 *
 * It is persisted in **two** places, with a strict order of authority:
 *
 * 1. **Desktop host file** (`app/desktop/server_config.go`, user data directory)
 *    is the source of truth: the host needs it before any page exists (the
 *    loopback proxy forwards `/api`, `/ws`, `/uploads` to it), so it can never
 *    live in the page.
 * 2. **`localStorage` mirror** (`gospeak.server.baseUrl`) is written by both
 *    platforms: it gives the page a *synchronous* read (`getBaseUrlSync`) so the
 *    login field, share links and the settings tab render the right address on
 *    the first paint instead of waiting for an IPC round-trip, and it is the
 *    only store available in a plain browser.
 *
 * The mirror is never trusted blindly: `getBaseUrl()` always re-reads the
 * authority (host on desktop, the mirror itself on web) and repairs the mirror
 * when they disagree — e.g. after the host swapped servers, or the file was
 * edited by hand.
 *
 * JSON shape (keep in sync with app/desktop/server_config.go):
 *   { baseUrl: string }
 */
export interface ServerConfig {
	/**
	 * Authoritative value: the host's persisted URL in the desktop client
	 * (repairing the mirror on the way), the mirror in a browser. "" when unset.
	 */
	getBaseUrl(): Promise<string>;
	/**
	 * Synchronous mirror read. Same value as {@link getBaseUrl} in the common
	 * case, possibly one change behind until the host answers. Safe to call
	 * during render; "" when unavailable (SSR, storage disabled).
	 */
	getBaseUrlSync(): string;
	/**
	 * Persists a new URL ("" clears it). On desktop this also retargets the
	 * loopback proxy and reloads the windows, so the call may never resolve.
	 */
	setBaseUrl(url: string): Promise<void>;
}

/** localStorage slot shared by both platforms. */
export const SERVER_BASE_URL_KEY = "gospeak.server.baseUrl";

/** Reads the mirror. Guarded: SSR, SSR-like tests and private mode throw. */
function readMirror(): string {
	try {
		if (typeof localStorage === "undefined") return "";
		return (localStorage.getItem(SERVER_BASE_URL_KEY) ?? "").trim();
	} catch {
		return "";
	}
}

/** Writes (or clears) the mirror. Failures are ignored: it is a cache. */
function writeMirror(url: string): void {
	try {
		if (typeof localStorage === "undefined") return;
		const next = url.trim();
		if (next === "") localStorage.removeItem(SERVER_BASE_URL_KEY);
		else localStorage.setItem(SERVER_BASE_URL_KEY, next);
	} catch {
		// Private mode / quota: the cache simply stays cold.
	}
}

/**
 * Browser implementation: the mirror is all there is. The page is served by the
 * backend itself, so the API client keeps talking to its own origin; storing the
 * address still matters for share links / diagnostics, and keeps one code path
 * with the desktop client.
 */
const webStorage: ServerConfig = {
	async getBaseUrl() {
		return readMirror();
	},
	getBaseUrlSync: readMirror,
	async setBaseUrl(url) {
		writeMirror(url);
	},
};

// 命令名与 app/desktop/server_config.go 中 mygo 绑定的方法逐字一致：
// 结构体 `ServerConfig` + 方法 `GetBaseURL` / `SetBaseURL`。base URL 是单个位置参数。
const native: ServerConfig = {
	async getBaseUrl() {
		const url = await bridge.invoke<string>(
			command("ServerConfig", "GetBaseURL"),
		);
		const next = typeof url === "string" ? url.trim() : "";
		// Host wins: repair the mirror when it drifted (manual edit, failed switch).
		if (next !== readMirror()) writeMirror(next);
		return next;
	},
	getBaseUrlSync: readMirror,
	async setBaseUrl(url) {
		const next = url.trim();
		const previous = readMirror();
		// Write the mirror first: the host reloads every window as part of this
		// call, so the "after" code below may never run — but the next boot will
		// reconcile against the host anyway.
		writeMirror(next);
		try {
			await bridge.invoke(command("ServerConfig", "SetBaseURL"), next);
		} catch (error) {
			// The host kept the old URL (e.g. invalid input): drop the optimistic
			// value so the UI does not show a server that is not in effect.
			writeMirror(previous);
			throw error;
		}
	},
};

/** Returns native server-config controls inside the desktop host, storage otherwise. */
export function createServerConfig(): ServerConfig {
	return isDesktop() ? native : webStorage;
}
