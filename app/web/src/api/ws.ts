import apiClient from "./apiClient";
import type { WSEndpointInfo } from "@/protocol/ws";

export type { WSEndpointInfo };

/** 解析 WS Worker 节点地址；握手鉴权由 HttpOnly access cookie 自动携带。 */
export async function getWSEndpoint(
	domainUUID?: string,
): Promise<WSEndpointInfo> {
	const data = await apiClient.get<{ url?: string }>({
		url: "/api/v1/signal/ws-endpoint",
		params: domainUUID ? { domain_uuid: domainUUID } : undefined,
	});
	return { url: data?.url || undefined };
}
