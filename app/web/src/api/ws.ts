import apiClient from "./apiClient";
import type { WSTicketInfo } from "@/protocol/ws";

export type { WSTicketInfo };

export async function getWSTicket(domainUUID?: string): Promise<WSTicketInfo> {
	const data = await apiClient.get<{ ticket: string; url?: string }>({
		url: "/api/v1/signal/ws-ticket",
		params: domainUUID ? { domain_uuid: domainUUID } : undefined,
	});
	const ticket = data?.ticket;
	if (!ticket) throw new Error("ws ticket is missing");
	return { url: data.url || undefined, token: ticket };
}
