import type {
	MemberRef,
	MessageEvent,
	PermissionLevel,
	RoomRef,
} from "./types";

export interface Logger {
	debug(...args: unknown[]): void;
	info(...args: unknown[]): void;
	warn(...args: unknown[]): void;
	error(...args: unknown[]): void;
}

export interface ChatClient {
	send(roomId: string, content: string): Promise<void>;
	reply(event: MessageEvent, content: string): Promise<void>;
}

export interface RoomClient {
	listRooms(): Promise<RoomRef[]>;
	getMembers(roomId: string): Promise<MemberRef[]>;
	createRoom(name: string, limit?: number): Promise<RoomRef>;
	join(name: string, opts?: { sfu?: boolean }): Promise<void>;
	leave(name: string): void;
	joined(): string[];
}

export interface VoiceClient {
	muteMember(roomId: string, identity: string, muted: boolean): Promise<void>;
	removeMember(roomId: string, identity: string): Promise<void>;
	setMemberVolume(
		roomId: string,
		identity: string,
		volume: number,
	): Promise<void>;
}

export interface KeyValueStore {
	get<T = unknown>(key: string): Promise<T | undefined>;
	set<T = unknown>(key: string, value: T): Promise<void>;
	delete(key: string): Promise<void>;
}

export interface BotContext {
	readonly logger: Logger;
	readonly config: Record<string, unknown>;
	readonly pluginName: string;
	readonly chat: ChatClient;
	readonly rooms: RoomClient;
	readonly voice: VoiceClient;
	readonly users: {
		getByIdentity(
			identity: string,
		): Promise<{ id: number; name: string; role: string; uuid: string }>;
	};
	readonly kv: KeyValueStore;
	hasPermission(level: PermissionLevel, member?: MemberRef): boolean;
}
