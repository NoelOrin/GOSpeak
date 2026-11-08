import { bridge, command } from "../bridge";
import { isDesktop } from "../runtime";

/**
 * Backend base URL the desktop client targets. The host persists it (user data)
 * so it survives restarts; the web frontend applies it to its API client so all
 * REST calls and WebSocket endpoint resolution hit the right server.
 *
 * JSON shape (keep in sync with app/desktop/server_config.go):
 *   { baseUrl: string }
 */
export interface ServerConfig {
	/** Returns the configured base URL ("" when unset). */
	getBaseUrl(): Promise<string>;
	/** Persists a new base URL (empty clears it). */
	setBaseUrl(url: string): Promise<void>;
}

const webFallback: ServerConfig = {
	async getBaseUrl() {
		return "";
	},
	async setBaseUrl() {},
};

// 命令名与 app/desktop/server_config.go 中 mygo 绑定的方法逐字一致：
// 结构体 `ServerConfig` + 方法 `GetBaseURL`。base URL 是单个位置参数。
const native: ServerConfig = {
	getBaseUrl: () =>
		bridge.invoke<string>(command("ServerConfig", "GetBaseURL")),
	setBaseUrl: (url) =>
		bridge.invoke(command("ServerConfig", "SetBaseURL"), url),
};

/** Returns native server-config controls inside the desktop host, web fallback otherwise. */
export function createServerConfig(): ServerConfig {
	return isDesktop() ? native : webFallback;
}
