import type {
	ChatClient,
	KeyValueStore,
	Logger,
	RoomClient,
	VoiceClient,
} from "../core/context";
import type { MemberRef, RoomRef } from "../core/types";

export interface ApiClientOptions {
	baseUrl: string;
	accessToken?: string;
	logger: Logger;
}

// ─── Backend response types ───

interface ApiResponse<T> {
	code: number;
	msg: string;
	data: T;
}

interface SFUTokenResult {
	token: string;
	serverUrl: string;
	room: string;
	identity: string;
	provider?: string;
	stream?: string;
	streamToken?: string;
	capabilities?: Record<string, unknown>;
	clientInfo?: Record<string, unknown>;
}

interface RoomCreateResult {
	id: number;
	uuid: string;
	name: string;
	description: string;
	limit: number;
	audio_only: boolean;
	allow_audience: boolean;
	created_by: string;
	created_at: string;
	updated_at: string;
}

interface UserInfoResult {
	id: number;
	uuid: string;
	name: string;
	display_name: string;
	avatar: string;
	email: string;
	role: string;
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
		if (this.opts.accessToken)
			h.Authorization = `Bearer ${this.opts.accessToken}`;
		return h;
	}

	private async request<T>(
		method: string,
		path: string,
		body?: unknown,
	): Promise<T> {
		const url = `${this.opts.baseUrl}${path}`;
		const res = await fetch(url, {
			method,
			headers: this.headers,
			body: body ? JSON.stringify(body) : undefined,
		});
		const json = (await res.json()) as ApiResponse<T>;
		if (json.code !== 0) throw new Error(`API error ${json.code}: ${json.msg}`);
		return json.data;
	}

	// ── ChatClient ──
	// Chat is socket-only via bot:message. These throw to prevent misuse.
	async send(_roomId: string, _content: string): Promise<void> {
		throw new Error(
			"Chat is socket-only — use socketClient or BotRunner instead of apiClient.send",
		);
	}

	async reply(
		_event: { room: { id: string }; sender: { identity: string } },
		_content: string,
	): Promise<void> {
		throw new Error(
			"Chat is socket-only — use socketClient or BotRunner instead of apiClient.reply",
		);
	}

	// ── RoomClient ──

	async listRooms(): Promise<RoomRef[]> {
		const rooms = await this.request<{ name: string; memberCount?: number }[]>(
			"GET",
			"/api/v1/signal/rooms",
		);
		return (rooms ?? []).map((r) => ({
			id: r.name,
			name: r.name,
		}));
	}

	async getMembers(roomId: string): Promise<MemberRef[]> {
		const participants = await this.request<
			{ identity: string; joinedAt?: number }[]
		>("GET", `/api/v1/signal/participants?room=${encodeURIComponent(roomId)}`);
		return (participants ?? []).map((p) => ({
			identity: p.identity,
			name: p.identity,
			role: "member" as MemberRef["role"],
		}));
	}

	async createRoom(
		name: string,
		limit?: number,
		password?: string,
	): Promise<RoomRef> {
		const data = await this.request<RoomCreateResult>(
			"POST",
			"/api/v1/room/create",
			{
				name,
				...(limit ? { limit } : {}),
				...(password ? { password } : {}),
			},
		);
		return { id: data.uuid || String(data.id), name: data.name };
	}

	// join/leave/joined are implemented by BotRunner via socketClient, not here.
	async join(_name: string, _opts?: { sfu?: boolean }): Promise<void> {
		throw new Error(
			"RoomClient.join should be called via BotRunner, not apiClient",
		);
	}
	leave(_name: string): void {
		throw new Error(
			"RoomClient.leave should be called via BotRunner, not apiClient",
		);
	}
	joined(): string[] {
		return [];
	}

	// ── VoiceClient ──

	async muteMember(
		_roomId: string,
		_identity: string,
		_muted: boolean,
	): Promise<void> {
		throw new Error(
			"No REST endpoint for SFU mute — use the mute API (POST /api/v1/mute/create|cancel) or socket signaling",
		);
	}

	async removeMember(_roomId: string, _identity: string): Promise<void> {
		throw new Error(
			"Kick is socket-only via room:kick — use socketClient.kickMember or BotRunner",
		);
	}

	async setMemberVolume(
		_roomId: string,
		_identity: string,
		_volume: number,
	): Promise<void> {
		this.logger.warn(
			"setMemberVolume is a client-local operation, not a server API",
		);
	}

	// ── 用户查询 ──

	async getUserByIdentity(
		identity: string,
	): Promise<{ id: number; name: string; role: string; uuid: string }> {
		const data = await this.request<UserInfoResult>(
			"POST",
			"/api/v1/user/info",
			{ identity },
		);
		return {
			id: data.id,
			uuid: data.uuid,
			name: data.name,
			role: data.role,
		};
	}

	// ── 禁言管理（正确的 REST 路径）──

	async muteUser(
		userId: number,
		duration: number,
		permanent: boolean,
		reason?: string,
	): Promise<void> {
		await this.request("POST", "/api/v1/mute/create", {
			user_id: userId,
			duration,
			permanent,
			reason: reason ?? "",
		});
	}

	async unmuteUser(userId: number): Promise<void> {
		await this.request("POST", "/api/v1/mute/cancel", { user_id: userId });
	}

	async listMutes(): Promise<unknown[]> {
		return this.request("POST", "/api/v1/mute/list");
	}

	async getMuteStatus(userId: number): Promise<unknown | null> {
		return this.request("POST", "/api/v1/mute/status", { user_id: userId });
	}

	// ── SFU Token ──

	async getSFUToken(room: string): Promise<{
		token: string;
		serverUrl: string;
		provider?: string;
		stream?: string;
		streamToken?: string;
	}> {
		const data = await this.request<SFUTokenResult>(
			"POST",
			"/api/v1/signal/token",
			{ room },
		);
		return {
			token: data.token,
			serverUrl: data.serverUrl,
			provider: data.provider,
			stream: data.stream,
			streamToken: data.streamToken,
		};
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
