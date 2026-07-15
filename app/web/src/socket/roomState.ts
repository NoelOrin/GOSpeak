import type { MemberInfo, RoomInfo } from "./types";

export function mergeRoomUpdated(prev: RoomInfo[], room: RoomInfo): RoomInfo[] {
	if (!room?.name) return prev;
	const idx = prev.findIndex((r) => r.name === room.name);
	if (idx < 0) {
		return [
			...prev,
			{
				id: room.id ?? 0,
				uuid: room.uuid ?? "",
				name: room.name,
				hasPassword: room.hasPassword,
				description: room.description,
				limit: room.limit ?? 0,
				audioOnly: room.audioOnly,
				allowAudience: room.allowAudience,
				members: room.members ?? [],
				count: room.count ?? room.members?.length ?? 0,
				createdAt: room.createdAt ?? Date.now(),
			},
		];
	}
	return prev.map((r) =>
		r.name === room.name
			? {
					...r,
					name: room.name,
					hasPassword: room.hasPassword,
					members: room.members ?? [],
					count: room.count ?? room.members?.length ?? 0,
					createdAt: room.createdAt ?? r.createdAt,
				}
			: r,
	);
}

export function applyMemberJoinedShell(
	prev: RoomInfo[],
	data: { room: string; identity: string; id: string; stream?: string },
): RoomInfo[] {
	return prev.map((r) => {
		if (r.name !== data.room) return r;
		return {
			...r,
			count: r.count + 1,
			members: r.members.some((m) => m.id === data.id)
				? r.members
				: [
						...r.members,
						{
							id: data.id,
							identity: data.identity,
							name: "",
							displayName: "",
							avatar: "",
							isMuted: false,
							isMicMuted: false,
							joinedAt: Date.now(),
							stream: data.stream,
						},
					],
		};
	});
}

export function applyMemberLeft(
	prev: RoomInfo[],
	data: { room: string; identity: string; id: string },
): RoomInfo[] {
	return prev.map((r) => {
		if (r.name !== data.room) return r;
		return {
			...r,
			count: Math.max(0, r.count - 1),
			members: r.members.filter((m) => m.id !== data.id),
		};
	});
}

export function applyMemberUpdated(
	prev: RoomInfo[],
	data: { room: string; identity: string; isMicMuted: boolean },
): RoomInfo[] {
	return prev.map((r) => {
		if (r.name !== data.room) return r;
		return {
			...r,
			members: r.members.map((m) =>
				m.identity === data.identity
					? { ...m, isMicMuted: data.isMicMuted }
					: m,
			),
		};
	});
}

export function upsertRoomMembersFromAck(
	prev: RoomInfo[],
	roomName: string,
	ackMembers: MemberInfo[],
): RoomInfo[] {
	const exists = prev.some((r) => r.name === roomName);
	if (!exists) {
		return [
			...prev,
			{
				id: 0,
				uuid: "",
				name: roomName,
				hasPassword: false,
				limit: 0,
				members: ackMembers,
				count: ackMembers.length,
				createdAt: Date.now(),
			},
		];
	}
	return prev.map((r) =>
		r.name === roomName
			? { ...r, members: ackMembers, count: ackMembers.length }
			: r,
	);
}

export function addCreatedRoom(prev: RoomInfo[], room: RoomInfo): RoomInfo[] {
	if (prev.some((r) => r.name === room.name)) return prev;
	return [...prev, room];
}
