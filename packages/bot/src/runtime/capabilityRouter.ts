import type { ChatClient, RoomClient, VoiceClient } from "../core/context";
import type { MemberRef, RoomRef } from "../core/types";
import type { GOSpeakApiClient } from "./apiClient";
import type { GOSpeakSocketClient } from "./socketClient";

export interface SpeakHooks {
	speak?(roomId: string, text: string): Promise<void>;
	publishPcm?(
		roomId: string,
		pcm16: Int16Array,
		sampleRate?: number,
	): Promise<void>;
	stopSpeaking?(roomId: string): Promise<void>;
}

/**
 * CapabilityRouter wires plugin context operations to the correct backend channels:
 * - Chat → socket bot:message (no REST endpoint)
 * - Kick → socket room:kick (no REST endpoint)
 * - Mute → REST mute API (POST /api/v1/mute/create|cancel)
 * - Rooms → REST API + socket join/leave
 * - Users → REST API
 * - Speak/publishPcm → publish adapter (Phase 4)
 *
 * Plugins must NOT fetch arbitrary REST paths — only ctx.* through this router.
 */
export class CapabilityRouter implements ChatClient, RoomClient, VoiceClient {
	constructor(
		private readonly api: GOSpeakApiClient,
		private readonly socket: GOSpeakSocketClient,
		private readonly roomJoiner: {
			joinRoom(name: string, opts?: { sfu?: boolean }): Promise<void>;
			leaveRoom(name: string): void;
			joinedRooms: string[];
		},
		private readonly speakHooks: SpeakHooks = {},
	) {}

	// ── ChatClient (socket-only) ──

	async send(roomId: string, content: string): Promise<void> {
		this.socket.sendBotMessage(roomId, content);
	}

	async reply(
		event: { room: { id: string }; sender: { identity: string } },
		content: string,
	): Promise<void> {
		this.socket.sendBotMessage(event.room.id, content);
	}

	// ── RoomClient (REST + socket) ──

	async listRooms(): Promise<RoomRef[]> {
		return this.api.listRooms();
	}

	async getMembers(roomId: string): Promise<MemberRef[]> {
		return this.api.getMembers(roomId);
	}

	async createRoom(name: string, limit?: number): Promise<RoomRef> {
		return this.api.createRoom(name, limit);
	}

	async join(name: string, opts?: { sfu?: boolean }): Promise<void> {
		await this.roomJoiner.joinRoom(name, opts);
	}

	leave(name: string): void {
		this.roomJoiner.leaveRoom(name);
	}

	joined(): string[] {
		return this.roomJoiner.joinedRooms;
	}

	// ── VoiceClient (kick socket-only; mute via REST API) ──

	async muteMember(
		roomId: string,
		identity: string,
		muted: boolean,
	): Promise<void> {
		// Mute is managed via the REST mute API, not SFU-level mute.
		// This is a degraded path: we look up the user ID and call muteUser.
		// For proper local mic mute, clients should react to the user:muted socket event.
		if (muted) {
			const user = await this.api.getUserByIdentity(identity);
			await this.api.muteUser(
				user.id,
				0,
				false,
				`muted via bot in room ${roomId}`,
			);
		} else {
			const user = await this.api.getUserByIdentity(identity);
			await this.api.unmuteUser(user.id);
		}
	}

	async removeMember(_roomId: string, identity: string): Promise<void> {
		// Kick is socket-only — emit room:kick
		this.socket.kickMember(_roomId, identity);
	}

	async setMemberVolume(
		_roomId: string,
		_identity: string,
		_volume: number,
	): Promise<void> {
		// Client-local operation, no server action
	}

	async speak(roomId: string, text: string): Promise<void> {
		if (!this.speakHooks.speak) {
			throw new Error("speak is not enabled on this bot runtime");
		}
		await this.speakHooks.speak(roomId, text);
	}

	async publishPcm(
		roomId: string,
		pcm16: Int16Array,
		sampleRate?: number,
	): Promise<void> {
		if (!this.speakHooks.publishPcm) {
			throw new Error("publishPcm is not enabled on this bot runtime");
		}
		await this.speakHooks.publishPcm(roomId, pcm16, sampleRate);
	}

	async stopSpeaking(roomId: string): Promise<void> {
		if (!this.speakHooks.stopSpeaking) return;
		await this.speakHooks.stopSpeaking(roomId);
	}

	// ── 发言态回写（socket-only，best-effort）──

	setSpeaking(roomId: string, speaking: boolean): void {
		this.socket.reportSpeaking(roomId, speaking);
	}

	// ── 禁言查询 (REST) ──

	async listMutes(): Promise<unknown[]> {
		return this.api.listMutes();
	}

	async getMuteStatus(userId: number): Promise<unknown | null> {
		return this.api.getMuteStatus(userId);
	}

	// ── 用户查询 (REST) ──

	async getUserByIdentity(
		identity: string,
	): Promise<{ id: number; name: string; role: string; uuid: string }> {
		return this.api.getUserByIdentity(identity);
	}
}
