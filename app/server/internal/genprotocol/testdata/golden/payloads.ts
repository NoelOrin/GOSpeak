// GENERATED — 手改无效，跑 pnpm gen:protocol
// WS 事件 payload 类型：来源 app/server/internal/protocol/payloads.go
export interface MemberInfo {
	id: string;
	identity: string;
	name: string;
	displayName: string;
	avatar: string;
	isMuted: boolean;
	isMicMuted: boolean;
	joinedAt: number;
	stream?: string;
}

export interface MuteInfo {
	user_id: number;
	duration: number;
	permanent: boolean;
	reason: string;
	expires_at?: string;
}

export interface RoomInfo {
	id: number;
	uuid: string;
	name: string;
	domain_uuid?: string;
	hasPassword: boolean;
	description: string;
	limit: number;
	type: string;
	audioOnly: boolean;
	allowAudience: boolean;
	members: MemberInfo[];
	count: number;
	createdAt: number;
}

export interface RoomRequest {
	room: string;
	/** DomainUUID 命名空间隔离：同一房间名在不同 Domain 下视为不同房间。 */
	domain_uuid?: string;
	password?: string;
	identity?: string;
	stream?: string;
}

