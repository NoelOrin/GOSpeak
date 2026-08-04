import type { Logger } from "../core/context";

interface WSMessage {
	id?: string;
	event: string;
	data?: unknown;
	error?: { code: number; message: string };
}

interface PendingAck {
	resolve: (value: any) => void;
	reject: (err: Error) => void;
	timer: NodeJS.Timeout;
}

export interface SocketClientOptions {
	url: string;
	token?: string;
	logger: Logger;
	/** Used to resolve relative socket URLs (usually the server base URL). */
	baseUrl?: string;
}

function tryParse(data: unknown): unknown {
	if (typeof data !== "string") return data;
	try {
		return JSON.parse(data);
	} catch {
		return data;
	}
}

function resolveWsUrl(url: string, baseUrl?: string): string {
	let base = url.trim();
	if (!base && baseUrl) base = baseUrl.trim();
	if (!base) throw new Error("socket url is required");
	if (base.startsWith("/")) {
		const origin = baseUrl ? new URL(baseUrl).origin : "http://localhost:8998";
		base = origin + base;
	}
	base = base.replace(/^http:/, "ws:").replace(/^https:/, "wss:");
	const trimmed = base.replace(/\/+$/, "");
	if (/\/socket\.io$/i.test(trimmed)) {
		return trimmed.replace(/\/socket\.io$/i, "/ws");
	}
	if (trimmed.endsWith("/ws")) return trimmed;
	return `${trimmed}/ws`;
}

function resolveHttpBase(url?: string, baseUrl?: string): string {
	const base = baseUrl || url || "http://localhost:8998";
	try {
		return new URL(base).origin;
	} catch {
		return "http://localhost:8998";
	}
}

async function fetchWSTicket(
	base: string,
	token: string,
	logger: Logger,
): Promise<string> {
	const controller = new AbortController();
	const timer = setTimeout(() => controller.abort(), 5000);
	try {
		const res = await fetch(
			`${base.replace(/\/+$/, "")}/api/v1/signal/ws-ticket`,
			{
				headers: { Authorization: `Bearer ${token}` },
				signal: controller.signal,
			},
		);
		if (!res.ok) throw new Error(`ws ticket request failed: ${res.status}`);
		const body = (await res.json()) as { data?: { ticket?: string } };
		if (!body.data?.ticket) throw new Error("ws ticket is missing");
		return body.data.ticket;
	} catch (err) {
		logger.error("Failed to obtain WS ticket:", err);
		throw err;
	} finally {
		clearTimeout(timer);
	}
}

export { tryParse, resolveWsUrl, resolveHttpBase, fetchWSTicket };
export type { PendingAck, WSMessage };
