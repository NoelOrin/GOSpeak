import apiClient from "./apiClient";

export interface WSTicketInfo {
	/** Worker 节点地址；后端未返回时由 wsClient 沿用当前连接地址。 */
	url?: string;
	token: string;
}

export async function getWSTicket(): Promise<WSTicketInfo> {
	const data = await apiClient.get<{ ticket: string; url?: string }>({
		url: "/api/v1/signal/ws-ticket",
	});
	const ticket = data?.ticket;
	if (!ticket) throw new Error("ws ticket is missing");
	return { url: data.url || undefined, token: ticket };
}
