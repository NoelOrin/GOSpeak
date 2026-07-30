interface WSMessage {
	id?: string;
	event: string;
	data?: unknown;
	error?: { code: number; message: string };
}

export function createWSClient() {
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
	const eventHandlers = new Map<string, Set<(data: unknown) => void>>();
	const connectedCbs: Array<() => void> = [];
	const disconnectedCbs: Array<(reason: string) => void> = [];
	const connectErrorCbs: Array<(err: Error) => void> = [];
	let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
	let currentUrl = "";
	let currentToken = "";
	let shouldReconnect = true;

	function connect(url: string, token?: string) {
		currentUrl = url;
		currentToken = token || "";
		shouldReconnect = true;
		if (ws?.readyState === WebSocket.OPEN) return;
		if (ws) {
			ws.onclose = null;
			ws.close();
		}

		const wsUrl = token ? `${url}?token=${encodeURIComponent(token)}` : url;
		ws = new WebSocket(wsUrl);

		ws.onopen = () => {
			for (const cb of connectedCbs) cb();
		};

		ws.onmessage = (ev: MessageEvent) => {
			try {
				const msg: WSMessage = JSON.parse(ev.data as string);
				if (!msg.event) return;

				// Check if it's an ACK response
				if (msg.id) {
					const pending = pendingAcks.get(msg.id);
					if (pending) {
						pendingAcks.delete(msg.id);
						clearTimeout(pending.timer);
						if (msg.error) {
							pending.reject(new Error(msg.error.message));
						} else {
							pending.resolve(msg.data);
						}
						return;
					}
				}

				// Otherwise it's a server push event
				const handlers = eventHandlers.get(msg.event);
				if (handlers) {
					for (const handler of handlers) handler(msg.data);
				}
			} catch {
				// Ignore malformed messages
			}
		};

		ws.onclose = (ev: CloseEvent) => {
			for (const cb of disconnectedCbs) cb(ev.reason || "closed");
			// Clear pending ACKs
			for (const [_id, pending] of pendingAcks) {
				clearTimeout(pending.timer);
				pending.reject(new Error("disconnected"));
			}
			pendingAcks.clear();

			// Auto-reconnect with backoff
			if (shouldReconnect && currentUrl) {
				reconnectTimer = setTimeout(() => {
					connect(currentUrl, currentToken);
				}, 3000);
			}
		};

		ws.onerror = () => {
			for (const cb of connectErrorCbs) cb(new Error("websocket error"));
		};
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
		const msg: WSMessage = { event, data: payload };
		ws.send(JSON.stringify(msg));
	}

	function emitAck(event: string, payload?: unknown): Promise<unknown> {
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
			const msg: WSMessage = { id, event, data: payload };
			ws.send(JSON.stringify(msg));
		});
	}

	function onServerEvent(
		event: string,
		cb: (data: unknown) => void,
	): () => void {
		if (!eventHandlers.has(event)) eventHandlers.set(event, new Set());
		eventHandlers.get(event)?.add(cb);
		return () => {
			eventHandlers.get(event)?.delete(cb);
		};
	}

	function offAllServerEvents() {
		eventHandlers.clear();
	}

	function getSocket(): WebSocket | null {
		return ws;
	}

	function isConnected(): boolean {
		return ws?.readyState === WebSocket.OPEN;
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
		emitFireAndForget,
		emitAck,
		onServerEvent,
		offAllServerEvents,
		getSocket,
		isConnected,
		onConnected,
		onDisconnected,
		onConnectError,
	};
}
