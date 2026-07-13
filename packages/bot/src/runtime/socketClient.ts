import type { Logger } from "../core/context";
import {
	type BotEvent,
	EventType,
	type MemberRef,
	type MemberStateEvent,
	type RoomEvent,
	type RoomRef,
} from "../core/types";

type SocketIONamespace = any;

export interface SocketClientOptions {
	url: string;
	token?: string;
	logger: Logger;
}

/**
 * Wraps a Socket.IO connection and translates GOSpeak signaling events
 * into typed BotEvent objects, aligned with the server's actual emit payloads.
 *
 * Server emit formats (from app/server/internal/signal/hub.go):
 *   room:created     → RoomInfo { id, uuid, name, members[], count, ... }
 *   room:left        → { room: string }
 *   room:updated     → RoomInfo
 *   member:joined    → { room, identity, id, stream? }
 *   member:left      → { room, identity, id }
 *   member:updated   → { room, identity, isMicMuted }
 *   user:muted       → { user_id, duration, permanent, reason, expires_at? }
 *   user:unmuted     → { user_id }
 *   room:kicked      → { room, targetIdentity }
 *   room:list:result → { rooms: RoomInfo[] }
 */
export class GOSpeakSocketClient {
	private opts: SocketClientOptions;
	private socket: SocketIONamespace | null = null;
	private onEvent: ((event: BotEvent) => void) | null = null;
	private connected = false;
	private logger: Logger;
	private joinedRooms: Map<string, { identity: string }> = new Map();
	private connectResolve: ((value: void | PromiseLike<void>) => void) | null =
		null;

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
		return new Promise((resolve, reject) => {
			this.connectResolve = resolve;
			void import("socket.io-client")
				.then(({ io }) => {
					const opts: Record<string, unknown> = {};
					if (this.opts.token) opts.query = { token: this.opts.token };

					this.socket = io(this.opts.url, opts) as SocketIONamespace;
					this.setupListeners();
				})
				.catch((err) => {
					this.logger.error("Failed to load socket.io-client:", err);
					this.connectResolve = null;
					reject(err);
				});
		});
	}

	disconnect(): void {
		if (this.socket) {
			this.socket.disconnect();
			this.socket = null;
		}
		this.connected = false;
		this.joinedRooms.clear();
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
		this.socket?.emit("room:join", { room, identity });
		this.joinedRooms.set(room, { identity });
	}

	leaveRoom(room: string): void {
		if (!this.connected) {
			this.joinedRooms.delete(room);
			return;
		}
		this.socket?.emit("room:leave", { room });
		this.joinedRooms.delete(room);
	}

	listRooms(): void {
		this.socket?.emit("room:list");
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
		return new Promise((resolve, reject) => {
			if (!this.socket) {
				reject(new Error("socket not connected"));
				return;
			}
			this.socket.emit(
				"room:join:sfu",
				{ room, identity, stream },
				(ack: unknown) => {
					const resp = ack as Record<string, unknown>;
					if (resp?.error) {
						reject(new Error(String(resp.error)));
					} else {
						this.joinedRooms.set(room, { identity });
						resolve(resp as { ok: boolean; members: unknown[] });
					}
				},
			);
		});
	}

	kickMember(room: string, targetIdentity: string): void {
		if (!this.connected) {
			this.logger.warn("socket not connected; cannot kick member");
			return;
		}
		this.socket?.emit("room:kick", { room, targetIdentity });
	}

	private emit(event: BotEvent): void {
		this.onEvent?.(event);
	}

	private static parseRoomRef(raw: any): RoomRef {
		// room:created/updated deliver a full RoomInfo; member events deliver {room: string}
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

	private setupListeners(): void {
		if (!this.socket) return;

		this.socket.on("connect", () => {
			this.connected = true;
			this.connectResolve?.();
			this.connectResolve = null;
			this.logger.info("Socket.IO connected");
		});

		this.socket.on("disconnect", (reason: string) => {
			this.connected = false;
			this.joinedRooms.clear();
			this.logger.info("Socket.IO disconnected:", reason);
		});

		// room:created → RoomInfo (full struct)
		this.socket.on("room:created", (raw: any) => {
			if (raw?.error) {
				this.logger.warn("room:created error:", raw.error);
				return;
			}
			this.emit({
				eventType: EventType.OnRoomCreated,
				room: GOSpeakSocketClient.parseRoomRef(raw),
				timestamp: Date.now(),
			} as RoomEvent);
		});

		// room:updated → RoomInfo (member count changed)
		this.socket.on("room:updated", (raw: any) => {
			this.emit({
				eventType: EventType.OnRoomUpdated,
				room: GOSpeakSocketClient.parseRoomRef(raw),
				timestamp: Date.now(),
			} as RoomEvent);
		});

		// room:left → { room: string }
		this.socket.on("room:left", (raw: any) => {
			// clear room from joinedRooms on server confirmation
			const roomName = typeof raw === "string" ? raw : raw?.room;
			if (roomName) this.joinedRooms.delete(roomName);
			this.emit({
				eventType: EventType.OnRoomLeft,
				room: GOSpeakSocketClient.parseRoomRef(raw),
				timestamp: Date.now(),
			} as RoomEvent);
		});

		// member:joined → { room, identity, id, stream? }
		this.socket.on("member:joined", (raw: any) => {
			this.emit({
				eventType: EventType.OnRoomJoined,
				room: GOSpeakSocketClient.parseRoomRef(raw),
				actor: GOSpeakSocketClient.parseMemberRef(raw),
				timestamp: Date.now(),
			} as RoomEvent);
		});

		// member:left → { room, identity, id }
		this.socket.on("member:left", (raw: any) => {
			this.emit({
				eventType: EventType.OnRoomLeft,
				room: GOSpeakSocketClient.parseRoomRef(raw),
				actor: GOSpeakSocketClient.parseMemberRef(raw),
				timestamp: Date.now(),
			} as RoomEvent);
		});

		// member:updated → { room, identity, isMicMuted }
		this.socket.on("member:updated", (raw: any) => {
			this.emit({
				eventType: EventType.OnMemberStateChanged,
				room: GOSpeakSocketClient.parseRoomRef(raw),
				member: GOSpeakSocketClient.parseMemberRef(raw),
				muted: Boolean(raw?.isMicMuted),
				timestamp: Date.now(),
			} as MemberStateEvent);
		});

		// user:muted → { user_id, duration, permanent, reason, expires_at? }
		this.socket.on("user:muted", (raw: any) => {
			this.emit({
				eventType: EventType.OnMemberStateChanged,
				room: { id: "", name: "" },
				member: {
					identity: String(raw?.user_id ?? ""),
					name: String(raw?.user_id ?? ""),
					role: "member",
				},
				muted: true,
				timestamp: Date.now(),
			} as MemberStateEvent);
		});

		// user:unmuted → { user_id }
		this.socket.on("user:unmuted", (raw: any) => {
			this.emit({
				eventType: EventType.OnMemberStateChanged,
				room: { id: "", name: "" },
				member: {
					identity: String(raw?.user_id ?? ""),
					name: String(raw?.user_id ?? ""),
					role: "member",
				},
				muted: false,
				timestamp: Date.now(),
			} as MemberStateEvent);
		});

		// room:kicked → { room, targetIdentity }
		this.socket.on("room:kicked", (raw: any) => {
			const roomName = typeof raw === "string" ? raw : raw?.room;
			if (roomName) this.joinedRooms.delete(roomName);
			this.emit({
				eventType: EventType.OnRoomLeft,
				room: GOSpeakSocketClient.parseRoomRef(raw),
				actor: {
					identity: String(raw?.targetIdentity ?? ""),
					name: String(raw?.targetIdentity ?? ""),
					role: "member",
				},
				timestamp: Date.now(),
			} as RoomEvent);
		});

		this.socket.on("error", (err: Error) => {
			this.logger.error("Socket.IO error:", err);
		});
	}
}
