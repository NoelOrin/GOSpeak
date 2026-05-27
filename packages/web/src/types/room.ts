type RoomItemType = {
	id: string | number;
	name: string;
    limit: number;
    current: number;
    memberInfoList?: RoomMemberInfoType[],
}

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
    limit: number;
    members: MemberInfo[];
    count: number;
    createdAt: number;
};

export type { RoomItemType, RoomMemberInfoType, MemberInfo, RoomInfo }
