import type { BotContext, ChatClient, Logger, RoomClient, VoiceClient, KeyValueStore } from "../core/context";
import type { MemberRef, RoomRef } from "../core/types";

export interface ApiClientOptions {
	baseUrl: string;
	accessToken?: string;
	logger: Logger;
}

export class GOSpeakApiClient implements ChatClient, RoomClient, VoiceClient {
	private opts: ApiClientOptions;
	logger: Logger;

	constructor(opts: ApiClientOptions) {
		this.opts = opts;
		this.logger = opts.logger;
	}

	private get headers(): Record<string, string> {
		const h: Record<string, string> = { "Content-Type": "application/json" };
		if (this.opts.accessToken) h["Authorization"] = `Bearer ${this.opts.accessToken}`;
		return h;
	}

	private async request<T>(method: string, path: string, body?: unknown): Promise<T> {
		const url = `${this.opts.baseUrl}${path}`;
		const res = await fetch(url, {
			method,
			headers: this.headers,
			body: body ? JSON.stringify(body) : undefined,
		});
		const json = await res.json() as { code: number; msg: string; data: T };
		if (json.code !== 0) throw new Error(`API error ${json.code}: ${json.msg}`);
		return json.data;
	}

	// ── ChatClient ──
	async send(roomId: string, content: string): Promise<void> {
		await this.request("POST", "/api/v1/chat/send", { roomId, content });
	}

	async reply(event: { room: { id: string }; sender: { identity: string } }, content: string): Promise<void> {
		await this.request("POST", "/api/v1/chat/send", {
			roomId: event.room.id,
			content,
			replyTo: event.sender.identity,
		});
	}

	// ── RoomClient ──
	async listRooms(): Promise<RoomRef[]> {
		const data = await this.request<{ rooms: { id: string; name: string }[] }>("GET", "/api/v1/signal/rooms");
		return (data.rooms ?? []).map((r) => ({ id: r.id, name: r.name }));
	}

	async getMembers(roomId: string): Promise<MemberRef[]> {
		const data = await this.request<{ participants: { identity: string; name: string; role: string }[] }>(
			"GET",
			`/api/v1/signal/participants?room=${encodeURIComponent(roomId)}`,
		);
		return (data.participants ?? []).map((p) => ({
			identity: p.identity,
			name: p.name,
			role: p.role as MemberRef["role"],
		}));
	}

	async createRoom(name: string, _limit?: number): Promise<RoomRef> {
		const data = await this.request<{ id: string; name: string }>("POST", "/api/v1/signal/token", { room: name });
		return { id: data.id, name: data.name };
	}

	// ── VoiceClient ──
	async muteMember(roomId: string, identity: string, muted: boolean): Promise<void> {
		await this.request("POST", "/api/v1/sfu/mute", { room: roomId, identity, muted });
	}

	async removeMember(roomId: string, identity: string): Promise<void> {
		await this.request("POST", "/api/v1/sfu/remove-participant", { room: roomId, identity });
	}

	async setMemberVolume(roomId: string, identity: string, volume: number): Promise<void> {
		this.logger.warn("setMemberVolume is a client-local operation, not a server API");
		// no-op on server side; handled client-local in web
	}

	/** Allow token refresh for long-lived bot sessions */
	setAccessToken(token: string): void {
		this.opts.accessToken = token;
	}
}

export function createKVStore(): KeyValueStore {
	const store = new Map<string, string>();
	return {
		async get<T>(key: string): Promise<T | undefined> {
			const v = store.get(key);
			return v !== undefined ? (JSON.parse(v) as T) : undefined;
		},
		async set<T>(key: string, value: T): Promise<void> {
			store.set(key, JSON.stringify(value));
		},
		async delete(key: string): Promise<void> {
			store.delete(key);
		},
	};
}
