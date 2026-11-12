import apiClient from "./apiClient";
import type { WSEndpointInfo } from "@/protocol/ws";

export type { WSEndpointInfo };

/**
 * 把 SFU/信令 worker 节点的绝对地址改写为同源的 `/wsworker?target=…` 转发路径，
 * 由 Go 宿主（本地回环代理）携带会话 Cookie 转发到真实 worker。
 *
 * 构建期裁剪：`__GOSPEAK_DESKTOP__` 由 vite define 注入字面量（见
 * app/web/vite.config.ts，随 GOSPEAK_TARGET 注入），Web 构建中该分支被常量
 * 折叠整体剔除，不进包。桌面构建内保留的运行时判断仅用于区分宿主形态：
 * release 窗口加载回环代理（hostname 127.0.0.1）才走同源转发；dev 窗口
 * （GOSPEAK_DESKTOP_DEV=1）加载 Vite dev server，代理未启动，保持直连，
 * 与浏览器行为一致。
 */
function toSameOriginWorkerUrl(url: string): string {
	if (!__GOSPEAK_DESKTOP__) return url;
	if (
		typeof window === "undefined" ||
		window.location.hostname !== "127.0.0.1"
	) {
		return url;
	}
	const proto = window.location.protocol === "https:" ? "wss" : "ws";
	return `${proto}://${window.location.host}/wsworker?target=${encodeURIComponent(url)}`;
}

/** 解析 WS Worker 节点地址；握手鉴权由 HttpOnly access cookie 自动携带。 */
export async function getWSEndpoint(
	domainUUID?: string,
): Promise<WSEndpointInfo> {
	const data = await apiClient.get<{ url?: string }>({
		url: "/api/v1/signal/ws-endpoint",
		params: domainUUID ? { domain_uuid: domainUUID } : undefined,
	});
	const url = data?.url ? toSameOriginWorkerUrl(data.url) : undefined;
	return { url };
}
