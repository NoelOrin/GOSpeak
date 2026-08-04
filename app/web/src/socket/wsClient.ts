import type { SignalSocket } from "@gospeak/sfu-client/types";

interface WSMessage {
	id?: string;
	event: string;
	data?: unknown;
	error?: { code: number; message: string };
}

function tryParse(data: unknown): unknown {
	if (typeof data !== "string") return data;
	try {
		return JSON.parse(data);
	} catch {
		return data;
	}
}

function normalizePayload(payload?: unknown): unknown {
	if (typeof payload !== "string") return payload;
	try {
		return JSON.parse(payload);
	} catch {
		return payload;
	}
}

function resolveWsUrl(url: string): string {
	let base = url.trim();
	if (!base) {
		const protocol =
			typeof window !== "undefined" && window.location.protocol === "https:"
				? "wss:"
				: "ws:";
		return `${protocol}//${window.location.host}/ws`;
	}
	if (base.startsWith("/")) {
		const protocol =
			typeof window !== "undefined" && window.location.protocol === "https:"
				? "wss:"
				: "ws:";
		base = `${protocol}//${window.location.host}${base}`;
	} else {
		base = base.replace(/^http:/, "ws:").replace(/^https:/, "wss:");
	}
	const trimmed = base.replace(/\/+$/, "");
	if (/\/socket\.io$/i.test(trimmed)) {
		return trimmed.replace(/\/socket\.io$/i, "/ws");
	}
	if (trimmed.endsWith("/ws")) return trimmed;
	return `${trimmed}/ws`;
}

export interface WSClientOptions {
	/** 自动重连前重新获取短时 ws ticket，避免复用过期凭证。 */
	refreshTicket?: () => Promise<string>;
}

export function createWSClient(options: WSClientOptions = {}): SignalSocket & {
	connect: (url: string, token?: string) => void;
	disconnect: () => void;
	getCurrentUrl: () => string;
	offServerEvent: (event: string, cb: (...args: any[]) => void) => void;
	offAllServerEvents: () => void;
	onConnected: (cb: () => void) => () => void;
	onConnectError: (cb: (err: Error) => void) => () => void;
} {
	let ws: WebSocket | null = null;
	let msgIdCounter = 0;
	const pendingAcks = new Map<
		string,
		{
			resolve: (v: unknown) => void;
			reject: (e: Error) => void;
			timer: ReturnType<typeof setTimeout>;
		}
	>();
	const eventHandlers = new Map<string, Set<(...args: any[]) => void>>();
	const connectedCbs: Array<() => void> = [];
	const disconnectedCbs: Array<(reason: string) => void> = [];
	const connectErrorCbs: Array<(err: Error) => void> = [];
	let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
	let currentUrl = "";
	let currentToken = "";
	let shouldReconnect = true;
	let reconnectAttempts = 0;
	const refreshTicket = options.refreshTicket;

	function connect(url: string, token?: string) {
		currentUrl = url;
		currentToken = token || "";
		shouldReconnect = true;
		if (reconnectTimer) {
			clearTimeout(reconnectTimer);
			reconnectTimer = null;
		}
		if (ws?.readyState === WebSocket.OPEN) return;
		if (ws) {
			ws.onclose = null;
			ws.close();
			ws = null;
		}

		const protocols = currentToken ? ["gospeak", currentToken] : ["gospeak"];
		const wsHandle = new WebSocket(resolveWsUrl(url), protocols);
		ws = wsHandle;

		wsHandle.onopen = () => {
			reconnectAttempts = 0;
			for (const cb of connectedCbs) cb();
		};

		wsHandle.onmessage = (ev: MessageEvent) => {
			try {
				const msg: WSMessage = JSON.parse(ev.data as string);
				if (!msg.event) return;

				// ACK responses carry the original request id.
				if (msg.id) {
					const pending = pendingAcks.get(msg.id);
					if (pending) {
						pendingAcks.delete(msg.id);
						clearTimeout(pending.timer);
						if (msg.error) {
							pending.reject(new Error(msg.error.message));
						} else {
							pending.resolve(tryParse(msg.data));
						}
						return;
					}
				}

				// Otherwise it's a server push event.
				const handlers = eventHandlers.get(msg.event);
				if (handlers) {
					for (const handler of handlers) handler(msg.data);
				}
			} catch (err) {
				console.warn("[Socket] ignoring malformed frame:", err);
			}
		};

		wsHandle.onclose = (ev: CloseEvent) => {
			if (ws === wsHandle) ws = null;
			for (const cb of disconnectedCbs) cb(ev.reason || "closed");
			// Clear pending ACKs
			for (const [_id, pending] of pendingAcks) {
				clearTimeout(pending.timer);
				pending.reject(new Error("disconnected"));
			}
			pendingAcks.clear();

			// Auto-reconnect with exponential backoff + jitter.
			if (shouldReconnect && currentUrl) {
				const delay = Math.min(3000 * 2 ** reconnectAttempts, 30000);
				const jitter = Math.floor(Math.random() * 1000);
				reconnectAttempts += 1;
				reconnectTimer = setTimeout(() => {
					void reconnectWithFreshToken();
				}, delay + jitter);
			}
		};

		wsHandle.onerror = () => {
			for (const cb of connectErrorCbs) cb(new Error("websocket error"));
		};
	}

	async function reconnectWithFreshToken() {
		if (!shouldReconnect || !currentUrl) return;
		try {
			const nextToken = refreshTicket ? await refreshTicket() : currentToken;
			connect(currentUrl, nextToken);
		} catch (err) {
			shouldReconnect = false;
			currentUrl = "";
			for (const cb of connectErrorCbs) {
				cb(
					err instanceof Error ? err : new Error("failed to refresh ws ticket"),
				);
			}
		}
	}

	function disconnect() {
		shouldReconnect = false;
		if (reconnectTimer) {
			clearTimeout(reconnectTimer);
			reconnectTimer = null;
		}
		currentUrl = "";
		if (ws) {
			ws.onclose = null;
			ws.close();
			ws = null;
		}
		for (const [_id, pending] of pendingAcks) {
			clearTimeout(pending.timer);
			pending.reject(new Error("disconnected"));
		}
		pendingAcks.clear();
	}

	function emitFireAndForget(event: string, payload?: unknown) {
		if (!ws || ws.readyState !== WebSocket.OPEN) return;
		const msg: WSMessage = { event, data: normalizePayload(payload) };
		ws.send(JSON.stringify(msg));
	}

	function emitAck(event: string, payload?: unknown): Promise<any> {
		return new Promise((resolve, reject) => {
			if (!ws || ws.readyState !== WebSocket.OPEN) {
				reject(new Error("socket not connected"));
				return;
			}
			const id = String(++msgIdCounter);
			const timer = setTimeout(() => {
				if (pendingAcks.has(id)) {
					pendingAcks.delete(id);
					reject(new Error("ack timeout"));
				}
			}, 10000);
			pendingAcks.set(id, { resolve, reject, timer });
			const msg: WSMessage = { id, event, data: normalizePayload(payload) };
			ws.send(JSON.stringify(msg));
		});
	}

	function onServerEvent(
		event: string,
		cb: (...args: any[]) => void,
	): () => void {
		if (!eventHandlers.has(event)) eventHandlers.set(event, new Set());
		eventHandlers.get(event)?.add(cb);
		return () => {
			eventHandlers.get(event)?.delete(cb);
		};
	}

	function offServerEvent(event: string, cb: (...args: any[]) => void) {
		eventHandlers.get(event)?.delete(cb);
	}

	function offAllServerEvents() {
		eventHandlers.clear();
	}

	function isConnected(): boolean {
		return ws?.readyState === WebSocket.OPEN;
	}

	function getCurrentUrl(): string {
		return currentUrl;
	}

	function onConnected(cb: () => void): () => void {
		connectedCbs.push(cb);
		return () => {
			const idx = connectedCbs.indexOf(cb);
			if (idx >= 0) connectedCbs.splice(idx, 1);
		};
	}

	function onDisconnected(cb: (reason: string) => void): () => void {
		disconnectedCbs.push(cb);
		return () => {
			const idx = disconnectedCbs.indexOf(cb);
			if (idx >= 0) disconnectedCbs.splice(idx, 1);
		};
	}

	function onConnectError(cb: (err: Error) => void): () => void {
		connectErrorCbs.push(cb);
		return () => {
			const idx = connectErrorCbs.indexOf(cb);
			if (idx >= 0) connectErrorCbs.splice(idx, 1);
		};
	}

	return {
		connect,
		disconnect,
		getCurrentUrl,
		emitFireAndForget,
		emitAck,
		onServerEvent,
		offServerEvent,
		offAllServerEvents,
		isConnected,
		onConnected,
		onDisconnected,
		onConnectError,
	};
}
