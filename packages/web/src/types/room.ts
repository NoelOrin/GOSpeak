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


export type { RoomItemType, RoomMemberInfoType }
