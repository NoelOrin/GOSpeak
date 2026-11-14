// GENERATED — 手改无效，跑 pnpm gen:protocol
// WS 事件 payload 类型：来源 app/server/internal/protocol/payloads.go
export interface AnnouncementPayload {
	uuid: string;
	scope_type: string;
	scope_id?: string;
	title: string;
	content: string;
	level: string;
	pinned: boolean;
	published_at?: number;
	expires_at?: number;
	created_by_uuid?: string;
	created_by_name: string;
	is_read: boolean;
	created_at: number;
}

export interface MemberInfo {
	id: string;
	identity: string;
	name: string;
	display_name: string;
	avatar: string;
	is_muted: boolean;
	is_mic_muted: boolean;
	joined_at: number;
	stream?: string;
}

export interface MemberLeftPayload {
	room: string;
	domain_uuid?: string;
	identity: string;
	id: string;
	/** Enforcement 仅 member:kicked 后补发的 MemberLeft 事件携带。 */
	enforcement?: string;
}

export interface MemberMutedStatePayload {
	identity: string;
	muted: boolean;
}

export interface MemberUpdatedPayload {
	room: string;
	domain_uuid?: string;
	identity: string;
	is_mic_muted: boolean;
}

export interface MessageDTO {
	uuid: string;
	room_uuid: string;
	author_id: string;
	author_uuid: string;
	author_name: string;
	author_avatar: string;
	content: string;
	reply_to?: string;
	mentions?: string[];
	edited_at?: string;
	deleted: boolean;
	created_at: string;
	client_nonce?: string;
	conversation_id?: string;
	target_identity?: string;
}

export interface MessageMutatePayload {
	room: string;
	domain_uuid?: string;
	message_uuid: string;
	content?: string;
	emoji?: string;
}

export interface MessageReactionPayload {
	action: string;
	message_uuid: string;
	user_id: string;
	emoji: string;
}

export interface MessageSendPayload {
	room: string;
	domain_uuid?: string;
	content: string;
	reply_to?: string;
	mentions?: string[];
	client_nonce?: string;
}

export interface MuteInfo {
	user_id: number;
	duration: number;
	permanent: boolean;
	reason: string;
	expires_at?: string;
}

export interface PrivateSendPayload {
	target_identity: string;
	content: string;
	client_nonce?: string;
}

export interface RoomActiveSpeakersPayload {
	room: string;
	domain_uuid?: string;
	identities: string[];
}

export interface RoomInfo {
	id: number;
	uuid: string;
	name: string;
	domain_uuid?: string;
	has_password: boolean;
	description: string;
	limit: number;
	type: string;
	audio_only: boolean;
	allow_audience: boolean;
	members: MemberInfo[];
	count: number;
	created_at: number;
}

export interface RoomKickedPayload {
	room: string;
	domain_uuid?: string;
	target_identity: string;
	enforcement?: string;
}

export interface RoomLeftPayload {
	room: string;
	domain_uuid?: string;
}

export interface RoomListResultPayload {
	rooms: RoomInfo[];
	count: number;
}

export interface RoomRequest {
	room: string;
	/** DomainUUID 命名空间隔离：同一房间名在不同 Domain 下视为不同房间。 */
	domain_uuid?: string;
	password?: string;
	stream?: string;
}

export interface SFUProviderChangedPayload {
	provider: string;
}

export interface TypingPayload {
	room: string;
	domain_uuid?: string;
	identity: string;
	typing: boolean;
}
