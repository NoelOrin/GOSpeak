import io from "socket.io-client";

export function createSocketClient() {
	let socket: ReturnType<typeof io> | null = null;
	const connectedCbs: Array<() => void> = [];
	const serverEventCleanups: Array<() => void> = [];
	const disconnectedCbs: Array<(reason: string) => void> = [];
	const connectErrorCbs: Array<(err: Error) => void> = [];

	function connect(url: string, token?: string) {
		if (socket?.connected) return;
		if (socket) {
			(socket as any).io.reconnection(false);
			socket.disconnect();
			socket = null;
		}
		const opts: Record<string, unknown> = { transports: ["websocket"] };
		if (token) {
			opts.query = { token };
			// biome-ignore lint/suspicious/noDocumentCookie: token persistence for socket auth
			document.cookie = `gospeak_token=${token}; path=/; SameSite=Lax; max-age=3600`;
		}
		socket = io(url, opts);
		socket.on("connect", () => {
			for (const cb of connectedCbs) cb();
		});

		socket.on("disconnect", (reason: string) => {
			for (const cb of disconnectedCbs) cb(reason);
		});

		socket.on("connect_error", (err: Error) => {
			for (const cb of connectErrorCbs) cb(err);
		});
	}

	function disconnect() {
		offAllServerEvents();
		if (socket) {
			socket.disconnect();
			socket = null;
		}
	}

	function emitFireAndForget(event: string, payload?: unknown) {
		socket?.emit(event, payload ? JSON.stringify(payload) : undefined);
	}

	function emitAck(
		event: string,
		payload?: Record<string, unknown>,
	): Promise<any> {
		return new Promise((resolve, reject) => {
			if (!socket?.connected) {
				reject(new Error("socket not connected"));
				return;
			}
			socket.emit(
				event,
				payload ? JSON.stringify(payload) : undefined,
				(resp?: string) => {
					if (!resp) {
						resolve({ ok: true });
						return;
					}
					try {
						const parsed = JSON.parse(resp);
						if (parsed?.error) reject(new Error(parsed.error));
						else resolve(parsed);
					} catch {
						reject(new Error(`invalid response: ${resp}`));
					}
				},
			);
		});
	}

	function onServerEvent(
		event: string,
		cb: (...args: any[]) => void,
	): () => void {
		socket?.on(event, cb);
		const cleanup = () => {
			socket?.off(event, cb);
		};
		serverEventCleanups.push(cleanup);
		return cleanup;
	}

	function offAllServerEvents() {
		for (const cleanup of serverEventCleanups) cleanup();
		serverEventCleanups.length = 0;
	}

	function getSocket() {
		return socket;
	}

	function isConnected() {
		return socket?.connected ?? false;
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
