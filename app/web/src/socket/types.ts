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

export interface RoomInfo {
	id: number;
	uuid: string;
	name: string;
	guild_uuid?: string;
	hasPassword: boolean;
	description?: string;
	limit: number;
	audioOnly?: boolean;
	allowAudience?: boolean;
	type?: string;
	members: MemberInfo[];
	count: number;
	createdAt: number;
	/** @internal 临时传递密码，不从服务器获取 */
	_password?: string;
}

export interface MuteEvent {
	user_id: number;
	permanent: boolean;
	expires_at: string | null;
	reason: string;
}

export interface UnmuteEvent {
	user_id: number;
}

export interface ActivityEvent {
	type: "member_joined" | "member_left" | "room_joined" | "room_left";
	room: string;
	guild_uuid?: string;
	identity?: string;
	timestamp: number;
}

export interface RoomPresenceEvent {
	type: "member_joined" | "member_left";
	room: string;
	guild_uuid?: string;
	identity: string;
	timestamp: number;
}
