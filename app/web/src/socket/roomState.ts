import type { MemberInfo, RoomInfo } from "./types";

interface RoomIdentity {
	name: string;
	domain_uuid?: string;
}

function sameRoom(a: RoomIdentity, b: RoomIdentity): boolean {
	return a.name === b.name && (a.domain_uuid || "") === (b.domain_uuid || "");
}

function matchesRoom(
	r: RoomIdentity,
	roomName: string,
	domain_uuid?: string,
): boolean {
	return sameRoom(r, { name: roomName, domain_uuid });
}

export function mergeRoomUpdated(prev: RoomInfo[], room: RoomInfo): RoomInfo[] {
	if (!room?.name) return prev;
	const idx = prev.findIndex((r) => sameRoom(r, room));
	if (idx < 0) {
		return [
			...prev,
			{
				id: room.id ?? 0,
				uuid: room.uuid ?? "",
				name: room.name,
				domain_uuid: room.domain_uuid,
				has_password: room.has_password,
				description: room.description,
				limit: room.limit ?? 0,
				audio_only: room.audio_only,
				allow_audience: room.allow_audience,
				type: room.type,
				members: room.members ?? [],
				count: room.count ?? room.members?.length ?? 0,
				created_at: room.created_at,
			},
		];
	}
	return prev.map((r) =>
		sameRoom(r, room)
			? {
					...r,
					name: room.name,
					domain_uuid: room.domain_uuid ?? r.domain_uuid,
					has_password: room.has_password,
					members: room.members ?? [],
					count: room.count ?? room.members?.length ?? 0,
					created_at: room.created_at ?? r.created_at,
				}
			: r,
	);
}

export function applyMemberJoined(
	prev: RoomInfo[],
	data: {
		room: string;
		identity?: string;
		id?: string;
		stream?: string;
		domain_uuid?: string;
	},
): RoomInfo[] {
	// 只做计数增量：成员明细不本地造壳（name/avatar 缺失），由 join ack /
	// room:updated 全量补齐，服务端在 member:joined 后紧跟广播 room:updated。
	return prev.map((r) =>
		matchesRoom(r, data.room, data.domain_uuid)
			? { ...r, count: r.count + 1 }
			: r,
	);
}

export function applyMemberLeft(
	prev: RoomInfo[],
	data: { room: string; identity: string; id: string; domain_uuid?: string },
): RoomInfo[] {
	return prev.map((r) => {
		if (!matchesRoom(r, data.room, data.domain_uuid)) return r;
		return {
			...r,
			count: Math.max(0, r.count - 1),
			members: r.members.filter((m) => m.id !== data.id),
		};
	});
}

export function applyMemberUpdated(
	prev: RoomInfo[],
	data: {
		room: string;
		identity: string;
		is_mic_muted: boolean;
		domain_uuid?: string;
	},
): RoomInfo[] {
	return prev.map((r) => {
		if (!matchesRoom(r, data.room, data.domain_uuid)) return r;
		return {
			...r,
			members: r.members.map((m) =>
				m.identity === data.identity
					? { ...m, is_mic_muted: data.is_mic_muted }
					: m,
			),
		};
	});
}

export function upsertRoomMembersFromAck(
	prev: RoomInfo[],
	roomName: string,
	domainUUID: string | undefined,
	ackMembers: MemberInfo[],
): RoomInfo[] {
	const identity: RoomIdentity = { name: roomName, domain_uuid: domainUUID };
	const exists = prev.some((r) => sameRoom(r, identity));
	if (!exists) {
		return [
			...prev,
			{
				id: 0,
				uuid: "",
				name: roomName,
				domain_uuid: domainUUID,
				has_password: false,
				limit: 0,
				members: ackMembers,
				count: ackMembers.length,
			},
		];
	}
	return prev.map((r) =>
		sameRoom(r, identity)
			? { ...r, members: ackMembers, count: ackMembers.length }
			: r,
	);
}

export function addCreatedRoom(prev: RoomInfo[], room: RoomInfo): RoomInfo[] {
	if (prev.some((r) => sameRoom(r, room))) return prev;
	return [...prev, room];
}
