import WebSocket from "ws";
import { getBaseURL } from "./api";

export interface WSClientOptions {
	token: string;
	url?: string;
}

export interface WSAck {
	success?: boolean;
	error?: string;
	code?: number;
	data?: unknown;
	[key: string]: unknown;
}

interface ServerMessage {
	id?: string;
	event?: string;
	data?: unknown;
	error?: { code: number; message: string };
}

/** 服务端 ACK 的 data 是 handler 返回的 JSON 字符串（或结构体），归一化为对象。 */
function normalizeAck(msg: ServerMessage): WSAck {
	const ack: WSAck = {};
	if (msg.error) {
		ack.error = msg.error.message;
		ack.code = msg.error.code;
	}
	if (typeof msg.data === "string") {
		try {
			Object.assign(ack, JSON.parse(msg.data));
		} catch {
			ack.data = msg.data;
		}
	} else if (msg.data != null) {
		Object.assign(ack, msg.data as Record<string, unknown>);
	}
	return ack;
}

/**
 * 集成测试 WS 客户端：HttpOnly access cookie 鉴权 + 子协议 gospeak。
 * 服务端 WS 与 REST 同源（默认 8998）。
 */
export class TestWSClient {
	private ws: WebSocket;
	private seq = 0;
	private pending = new Map<
		string,
		{ resolve: (ack: WSAck) => void; reject: (err: Error) => void }
	>();
	private serverEvents: Array<{ event: string; data: unknown }> = [];
	private waiters: Array<{
		event: string;
		resolve: (data: unknown) => void;
	}> = [];

	constructor(options: WSClientOptions) {
		const base = (options.url ?? getBaseURL()).replace(/^http/, "ws");
		const url = base.endsWith("/ws") ? base : `${base}/ws`;
		this.ws = new WebSocket(url, ["gospeak"], {
			headers: { Cookie: `gospeak_token=${options.token}` },
		});
		this.ws.on("message", (raw) => {
			let msg: ServerMessage;
			try {
				msg = JSON.parse(String(raw));
			} catch {
				return;
			}
			if (msg.id != null && this.pending.has(msg.id)) {
				const entry = this.pending.get(msg.id);
				this.pending.delete(msg.id);
				if (entry) entry.resolve(normalizeAck(msg));
				return;
			}
			if (typeof msg.event === "string" && msg.id == null) {
				this.serverEvents.push({ event: msg.event, data: msg.data });
				this.waiters = this.waiters.filter((w) => {
					if (w.event === msg.event) {
						w.resolve(msg.data);
						return false;
					}
					return true;
				});
			}
		});
	}

	/** 等待连接建立（握手鉴权失败会 reject）。 */
	open(timeoutMs = 5000): Promise<void> {
		return new Promise((resolve, reject) => {
			const timer = setTimeout(
				() => reject(new Error("ws open timeout")),
				timeoutMs,
			);
			this.ws.once("open", () => {
				clearTimeout(timer);
				resolve();
			});
			this.ws.once("error", (err) => {
				clearTimeout(timer);
				reject(err);
			});
			this.ws.once("unexpected-response", (_req, res) => {
				clearTimeout(timer);
				reject(new Error(`ws handshake failed: status ${res.statusCode}`));
			});
		});
	}

	/** 发送事件并等待 ACK（服务端带 ack_id 回包）。 */
	emitAck(event: string, payload: unknown, timeoutMs = 5000): Promise<WSAck> {
		const id = String(++this.seq);
		return new Promise((resolve, reject) => {
			const timer = setTimeout(() => {
				this.pending.delete(id);
				reject(new Error(`emitAck timeout: ${event}`));
			}, timeoutMs);
			this.pending.set(id, {
				resolve: (ack) => {
					clearTimeout(timer);
					resolve(ack);
				},
				reject: (err) => {
					clearTimeout(timer);
					reject(err);
				},
			});
			this.ws.send(JSON.stringify({ id, event, data: payload }));
		});
	}

	/** 等待一条服务端推送事件。 */
	waitForEvent(event: string, timeoutMs = 5000): Promise<unknown> {
		const existing = this.serverEvents.find((e) => e.event === event);
		if (existing) return Promise.resolve(existing.data);
		return new Promise((resolve, reject) => {
			const timer = setTimeout(
				() => reject(new Error(`waitForEvent timeout: ${event}`)),
				timeoutMs,
			);
			this.waiters.push({
				event,
				resolve: (data) => {
					clearTimeout(timer);
					resolve(data);
				},
			});
		});
	}

	close(): void {
		this.ws.close();
	}
}

export async function connectWS(token: string): Promise<TestWSClient> {
	const client = new TestWSClient({ token });
	await client.open();
	return client;
}
