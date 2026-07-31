import type { AxiosResponse } from "axios";
import type { Result } from "./apiClient";
import apiClient from "./apiClient";

export async function getWSTicket(): Promise<string> {
	const res = (await apiClient.get({
		url: "/api/v1/signal/ws-ticket",
	})) as AxiosResponse<Result<{ ticket: string }>>;
	const ticket = (res as any).data?.data?.ticket as string | undefined;
	if (!ticket) throw new Error("ws ticket is missing");
	return ticket;
}
