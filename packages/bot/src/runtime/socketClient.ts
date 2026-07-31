import type { Logger } from "../core/context";
import {
	type BotEvent,
	EventType,
	type MemberRef,
	type MemberStateEvent,
	type RoomEvent,
	type RoomRef,
	type UserMuteEvent,
} from "../core/types";
import { EventAdapter } from "./eventAdapter";

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

/**
 * Native WebSocket client for GOSpeak signaling. Translates incoming WS
 * messages into typed BotEvent objects, aligned with app/server/internal/signal.
 */
export class GOSpeakSocketClient {
	private opts: SocketClientOptions;
	private socket: WebSocket | null = null;
	private onEvent: ((event: BotEvent) => void) | null = null;
	private connected = false;
	private logger: Logger;
	private joinedRooms: Map<string, { identity: string }> = new Map();
	private connectResolve: (() => void) | null = null;
	private connectReject: ((err: Error) => void) | null = null;
	private pendingAcks = new Map<string, PendingAck>();
	private msgId = 0;
	private _adapter = new EventAdapter();

	constructor(opts: SocketClientOptions) {
		this.opts = opts;
		this.logger = opts.logger;
	}

	get isConnected(): boolean {
		return this.connected;
	}

	get rooms(): string[] {
		return [...this.joinedRooms.keys()];
	}

	isInRoom(room: string): boolean {
		return this.joinedRooms.has(room);
	}

	setEventHandler(cb: (event: BotEvent) => void): void {
		this.onEvent = cb;
	}

	connect(): Promise<void> {
		if (this.socket) return Promise.resolve();
		return (async () => {
			let ticket: string | undefined;
			if (this.opts.token) {
				ticket = await fetchWSTicket(
					resolveHttpBase(this.opts.baseUrl, this.opts.url),
					this.opts.token,
					this.logger,
				);
			}
			return new Promise<void>((resolve, reject) => {
				this.connectResolve = resolve;
				this.connectReject = reject;
				let wsHandle: WebSocket;
				try {
					const wsUrl = resolveWsUrl(this.opts.url, this.opts.baseUrl);
					const protocols = ticket ? ["gospeak", ticket] : ["gospeak"];
					wsHandle = new WebSocket(wsUrl, protocols);
					this.socket = wsHandle;
				} catch (err) {
					this.connectResolve = null;
					this.connectReject = null;
					reject(err instanceof Error ? err : new Error(String(err)));
					return;
				}

				wsHandle.onopen = () => {
					this.connected = true;
					this.connectResolve?.();
					this.connectResolve = null;
					this.connectReject = null;
					this.logger.info("WS connected");
				};

				wsHandle.onmessage = (ev: MessageEvent) => {
					try {
						const msg: WSMessage = JSON.parse(String(ev.data));
						if (!msg.event) return;
						if (msg.id && this.pendingAcks.has(msg.id)) {
							const pending = this.pendingAcks.get(msg.id)!;
							this.pendingAcks.delete(msg.id);
							clearTimeout(pending.timer);
							if (msg.error) {
								pending.reject(new Error(msg.error.message));
							} else {
								pending.resolve(tryParse(msg.data));
							}
							return;
						}
						this.dispatchEvent(msg.event, msg.data);
					} catch (err) {
						this.logger.warn("Ignoring malformed WS frame:", err);
					}
				};

				wsHandle.onerror = () => {
					this.logger.error("WS error");
					this.connectReject?.(new Error("websocket error"));
					this.connectReject = null;
				};

				wsHandle.onclose = (ev: CloseEvent) => {
					if (this.socket === wsHandle) this.socket = null;
					this.connected = false;
					this.joinedRooms.clear();
					for (const [_id, pending] of this.pendingAcks) {
						clearTimeout(pending.timer);
						pending.reject(new Error("disconnected"));
					}
					this.pendingAcks.clear();
					const pendingReject = this.connectReject;
					this.connectResolve = null;
					this.connectReject = null;
					if (pendingReject)
						pendingReject(
							new Error(
								`websocket closed before open: ${ev.reason || "closed"}`,
							),
						);
					this.logger.info("WS disconnected:", ev.reason || "closed");
				};
			});
		})();
	}

	disconnect(): void {
		if (this.socket) {
			this.socket.onclose = null;
			this.socket.close();
			this.socket = null;
		}
		this.connected = false;
		this.joinedRooms.clear();
		for (const [_id, pending] of this.pendingAcks) {
			clearTimeout(pending.timer);
			pending.reject(new Error("disconnected"));
		}
		this.pendingAcks.clear();
		this.connectResolve = null;
		this.connectReject = null;
	}

	joinRoom(room: string, identity: string): void {
		if (!this.connected) {
			this.logger.warn("socket not connected; cannot join room");
			return;
		}
		if (this.joinedRooms.has(room)) {
			this.logger.debug(`Already in room ${room}, skipping join`);
			return;
		}
		this.send({ event: "room:join", data: { room, identity } });
		this.joinedRooms.set(room, { identity });
	}

	leaveRoom(room: string): void {
		if (!this.connected) {
			this.joinedRooms.delete(room);
			return;
		}
		this.send({ event: "room:leave", data: { room } });
		this.joinedRooms.delete(room);
	}

	listRooms(): void {
		this.send({ event: "room:list" });
	}

	joinRoomSFU(
		room: string,
		identity: string,
		stream?: string,
	): Promise<{ ok: boolean; members: unknown[] }> {
		if (this.joinedRooms.has(room)) {
			this.logger.debug(`Already in room ${room}, skipping SFU join`);
			return Promise.reject(new Error(`already in room ${room}`));
		}
		if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
			return Promise.reject(new Error("socket not connected"));
		}
		return new Promise((resolve, reject) => {
			const id = String(++this.msgId);
			const timer = setTimeout(() => {
				if (this.pendingAcks.has(id)) {
					this.pendingAcks.delete(id);
					reject(new Error("ack timeout"));
				}
			}, 10000);
			this.pendingAcks.set(id, { resolve, reject, timer });
			this.send({
				id,
				event: "room:join:sfu",
				data: { room, identity, stream },
			});
		});
	}

	sendBotMessage(room: string, content: string): void {
		if (!this.connected) {
			this.logger.warn("socket not connected; cannot send bot message");
			return;
		}
		this.send({ event: "bot:message", data: { room, content } });
	}

	kickMember(room: string, targetIdentity: string): void {
		if (!this.connected) {
			this.logger.warn("socket not connected; cannot kick member");
			return;
		}
		this.send({ event: "room:kick", data: { room, targetIdentity } });
	}

	private send(msg: WSMessage): void {
		if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
			this.logger.warn("socket not connected; message dropped");
			return;
		}
		this.socket.send(JSON.stringify(msg));
	}

	private emit(event: BotEvent): void {
		this.onEvent?.(event);
	}

	private static parseRoomRef(raw: any): RoomRef {
		if (typeof raw === "string") return { id: raw, name: raw };
		const name = raw?.name ?? raw?.room ?? "";
		const id = raw?.uuid ?? raw?.id ?? raw?.room ?? name;
		return { id: String(id), name: String(name) };
	}

	private static parseMemberRef(raw: any): MemberRef {
		const identity = raw?.identity ?? raw?.targetIdentity ?? "";
		const name = raw?.name ?? raw?.displayName ?? identity;
		const role = (raw?.role ?? "member") as MemberRef["role"];
		return { identity: String(identity), name: String(name), role };
	}

	private dispatchEvent(event: string, raw: any): void {
		switch (event) {
			case "room:created":
				if (raw?.error) {
					this.logger.warn("room:created error:", raw.error);
					return;
				}
				this.emit({
					eventType: EventType.OnRoomCreated,
					room: GOSpeakSocketClient.parseRoomRef(raw),
					timestamp: Date.now(),
				} as RoomEvent);
				break;
			case "room:updated":
				this.emit({
					eventType: EventType.OnRoomUpdated,
					room: GOSpeakSocketClient.parseRoomRef(raw),
					timestamp: Date.now(),
				} as RoomEvent);
				break;
			case "room:left":
				{
					const roomName = typeof raw === "string" ? raw : raw?.room;
					if (roomName) this.joinedRooms.delete(roomName);
					this.emit({
						eventType: EventType.OnRoomLeft,
						room: GOSpeakSocketClient.parseRoomRef(raw),
						timestamp: Date.now(),
					} as RoomEvent);
				}
				break;
			case "member:joined":
				this.emit({
					eventType: EventType.OnMemberJoined,
					room: GOSpeakSocketClient.parseRoomRef(raw),
					actor: GOSpeakSocketClient.parseMemberRef(raw),
					timestamp: Date.now(),
				} as RoomEvent);
				break;
			case "member:left":
				this.emit({
					eventType: EventType.OnMemberLeft,
					room: GOSpeakSocketClient.parseRoomRef(raw),
					actor: GOSpeakSocketClient.parseMemberRef(raw),
					timestamp: Date.now(),
				} as RoomEvent);
				break;
			case "member:updated":
				this.emit({
					eventType: EventType.OnMemberStateChanged,
					room: GOSpeakSocketClient.parseRoomRef(raw),
					member: GOSpeakSocketClient.parseMemberRef(raw),
					muted: Boolean(raw?.isMicMuted),
					timestamp: Date.now(),
				} as MemberStateEvent);
				break;
			case "user:muted":
				this.emit({
					eventType: EventType.OnUserMuted,
					userId: Number(raw?.user_id ?? 0),
					duration: raw?.duration,
					permanent: raw?.permanent,
					reason: raw?.reason,
					expiresAt: raw?.expires_at,
					timestamp: Date.now(),
				} as UserMuteEvent);
				break;
			case "user:unmuted":
				this.emit({
					eventType: EventType.OnUserUnmuted,
					userId: Number(raw?.user_id ?? 0),
					timestamp: Date.now(),
				} as UserMuteEvent);
				break;
			case "room:kicked":
				{
					const roomName = typeof raw === "string" ? raw : raw?.room;
					if (roomName) this.joinedRooms.delete(roomName);
					this.emit({
						eventType: EventType.OnMemberKicked,
						room: GOSpeakSocketClient.parseRoomRef(raw),
						actor: {
							identity: String(raw?.targetIdentity ?? ""),
							name: String(raw?.targetIdentity ?? ""),
							role: "member",
						},
						timestamp: Date.now(),
					} as RoomEvent);
				}
				break;
			case "bot:command":
			case "bot:message": {
				const events = this._adapter.adaptBotMessage(
					raw,
					EventType.AdapterMessage,
				);
				for (const ev of events) this.emit(ev);
				break;
			}
		}
	}
}
