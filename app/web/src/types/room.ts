type RoomItemType = {
	id: string | number;
	name: string;
	description?: string;
	limit: number;
	audioOnly?: boolean;
	allowAudience?: boolean;
	current: number;
	memberInfoList?: RoomMemberInfoType[];
};

type RoomMemberInfoType = {
	id: string | number;
	name: string;
	avatar: string;
};

// Socket.IO 信令相关类型
type MemberInfo = {
	id: string;
	identity: string;
	joinedAt: number;
};

type RoomInfo = {
	id: number;
	uuid: string;
	name: string;
	hasPassword: boolean;
	description?: string;
	limit: number;
	audioOnly?: boolean;
	allowAudience?: boolean;
	members: MemberInfo[];
	count: number;
	createdAt: number;
};

export type { MemberInfo, RoomInfo, RoomItemType, RoomMemberInfoType };
