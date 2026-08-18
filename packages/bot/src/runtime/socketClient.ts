export type { SocketClientOptions } from "./socketClientTransport";
import type { Logger } from "../core/context";
import {
	type ActiveSpeakersEvent,
	type BotEvent,
	EventType,
	type MemberRef,
	type MemberStateEvent,
	type RoomEvent,
	type RoomRef,
	type UserMuteEvent,
} from "../core/types";
import { EventAdapter } from "./eventAdapter";
import {
	tryParse,
	fetchWSTicket,
	type PendingAck,
	resolveHttpBase,
	resolveWsUrl,
	type SocketClientOptions,
	type WSMessage,
} from "./socketClientTransport";

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
	private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
	private reconnectAttempts = 0;
	private reconnecting = false;
	private manualDisconnect = false;
	private rejoinRooms: Map<string, { identity: string }> = new Map();
	private readonly maxReconnectDelayMs = 30_000;
	/** Bot identity (set from opts) used for member:speaking self-reports. */
	private _identity?: string;
	/** Latest active-speaker set per room, from room:active-speakers broadcasts. */
	private _speaking = new Map<string, Set<string>>();

	constructor(opts: SocketClientOptions) {
		this.opts = opts;
		this.logger = opts.logger;
		this._identity = opts.identity;
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
		this.manualDisconnect = false;
		this.reconnecting = false;
		if (this.reconnectTimer) {
			clearTimeout(this.reconnectTimer);
			this.reconnectTimer = null;
		}
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
					this.reconnectAttempts = 0;
					this.reconnecting = false;
					for (const [room, info] of this.rejoinRooms) {
						this.joinRoom(room, info.identity);
					}
					this.rejoinRooms.clear();
					this.logger.info("WS connected");
				};

				wsHandle.onmessage = (ev: MessageEvent) => {
					try {
						const msg: WSMessage = JSON.parse(String(ev.data));
						if (!msg.event) return;
						if (msg.id && this.pendingAcks.has(msg.id)) {
							const pending = this.pendingAcks.get(msg.id);
							this.pendingAcks.delete(msg.id);
							if (!pending) return;
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
					this.rejoinRooms = new Map(this.joinedRooms);
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
					this.scheduleReconnect();
				};
			});
		})();
	}

	disconnect(): void {
		this.manualDisconnect = true;
		this.reconnecting = false;
		if (this.reconnectTimer) {
			clearTimeout(this.reconnectTimer);
			this.reconnectTimer = null;
		}
		this.rejoinRooms.clear();
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

	private scheduleReconnect(): void {
		if (this.manualDisconnect || this.reconnecting || this.reconnectTimer)
			return;
		this.reconnecting = true;
		const delay = Math.min(
			1000 * 2 ** this.reconnectAttempts,
			this.maxReconnectDelayMs,
		);
		this.reconnectAttempts += 1;
		this.logger.info(`WS reconnect scheduled in ${delay}ms`);
		this.reconnectTimer = setTimeout(() => {
			this.reconnectTimer = null;
			this.reconnecting = false;
			this.connect().catch((err) => {
				this.logger.error("WS reconnect failed:", err);
				this.scheduleReconnect();
			});
		}, delay);
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
		this._speaking.delete(room);
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

	/** Self-report local speaking state so the server highlights this bot as an active speaker. */
	reportSpeaking(room: string, speaking: boolean): void {
		if (!room) {
			this.logger.warn("reportSpeaking requires a room");
			return;
		}
		this.send(
			GOSpeakSocketClient.buildSpeakingMessage(
				room,
				this._identity ?? "",
				speaking,
			),
		);
	}

	/** Current active speakers in a room, from the latest room:active-speakers broadcast. */
	getSpeakers(room: string): string[] {
		return [...(this._speaking.get(room) ?? [])];
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

	static parseActiveSpeakers(raw: any): ActiveSpeakersEvent | null {
		if (!raw) return null;
		const roomName =
			typeof raw === "string" ? raw : (raw?.room ?? raw?.name ?? "");
		if (!roomName) return null;
		const identities = Array.isArray(raw?.identities)
			? raw.identities.map((x: unknown) => String(x))
			: [];
		return {
			eventType: EventType.OnActiveSpeakers,
			room: { id: String(roomName), name: String(roomName) },
			identities,
			timestamp: Date.now(),
		};
	}

	static buildSpeakingMessage(
		room: string,
		identity: string,
		speaking: boolean,
	): WSMessage {
		return { event: "member:speaking", data: { room, identity, speaking } };
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
			case "room:active-speakers": {
				const ev = GOSpeakSocketClient.parseActiveSpeakers(raw);
				if (ev) {
					this._speaking.set(ev.room.name, new Set(ev.identities));
					this.emit(ev);
				}
				break;
			}
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
