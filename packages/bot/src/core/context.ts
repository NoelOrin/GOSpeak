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
	/** Phase 4: TTS speak into room (optional) */
	speak?(roomId: string, text: string): Promise<void>;
	publishPcm?(
		roomId: string,
		pcm16: Int16Array,
		sampleRate?: number,
	): Promise<void>;
	stopSpeaking?(roomId: string): Promise<void>;
	/** Report local speaking state for active-speaker highlighting (best-effort, socket-only). */
	setSpeaking?(roomId: string, speaking: boolean): void;
}

export interface KeyValueStore {
	get<T = unknown>(key: string): Promise<T | undefined>;
	set<T = unknown>(key: string, value: T): Promise<void>;
	delete(key: string): Promise<void>;
	/** Optional helpers implemented by host stores */
	has?(key: string): Promise<boolean>;
	keys?(prefix?: string): Promise<string[]>;
	clear?(): Promise<void>;
}

/**
 * First-class plugin-to-plugin message bus.
 * Does not go through system EventBus; pure in-process pub/sub.
 */
export interface PluginMessageBus {
	publish<T = unknown>(topic: string, payload?: T): Promise<number>;
	subscribe<T = unknown>(
		topic: string,
		handler: (msg: {
			topic: string;
			payload: T;
			from: string;
			timestamp: number;
		}) => void | Promise<void>,
	): () => void;
	once<T = unknown>(
		topic: string,
		handler: (msg: {
			topic: string;
			payload: T;
			from: string;
			timestamp: number;
		}) => void | Promise<void>,
	): () => void;
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
	/**
	 * Plugin-private KV (namespaced by pluginName).
	 * Other plugins cannot read these keys via their own ctx.kv.
	 */
	readonly kv: KeyValueStore;
	/**
	 * Cross-plugin shared KV (same store for every plugin).
	 * Use for coordination flags, shared counters, handoff state, etc.
	 */
	readonly sharedKv: KeyValueStore;
	/**
	 * First-class plugin message bus for direct inter-plugin events.
	 * publish/subscribe topics without importing other plugins.
	 */
	readonly bus: PluginMessageBus;
	readonly mutes: {
		list(): Promise<unknown[]>;
		status(userId: number): Promise<unknown | null>;
	};
	readonly scheduler: {
		every(id: string, ms: number, fn: () => void | Promise<void>): void;
		once(id: string, ms: number, fn: () => void | Promise<void>): void;
		clear(id: string): void;
		clearAll(): void;
	};
	/** Optional listen registry ops (Phase 2) */
	readonly listen?: {
		add(room: string): boolean;
		remove(room: string): boolean;
		list(): string[];
		clear(): string[];
	};
	hasPermission(level: PermissionLevel, member?: MemberRef): boolean;
}
