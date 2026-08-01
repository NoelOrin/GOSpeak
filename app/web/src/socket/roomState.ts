import type { MemberInfo, RoomInfo } from "./types";

interface RoomIdentity {
	name: string;
	guild_uuid?: string;
}

function sameRoom(a: RoomIdentity, b: RoomIdentity): boolean {
	return a.name === b.name && (a.guild_uuid || "") === (b.guild_uuid || "");
}

function matchesRoom(
	r: RoomIdentity,
	roomName: string,
	guild_uuid?: string,
): boolean {
	return sameRoom(r, { name: roomName, guild_uuid });
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
				guild_uuid: room.guild_uuid,
				hasPassword: room.hasPassword,
				description: room.description,
				limit: room.limit ?? 0,
				audioOnly: room.audioOnly,
				allowAudience: room.allowAudience,
				type: room.type,
				members: room.members ?? [],
				count: room.count ?? room.members?.length ?? 0,
				createdAt: room.createdAt ?? Date.now(),
			},
		];
	}
	return prev.map((r) =>
		sameRoom(r, room)
			? {
					...r,
					name: room.name,
					guild_uuid: room.guild_uuid ?? r.guild_uuid,
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
	data: {
		room: string;
		identity: string;
		id: string;
		stream?: string;
		guild_uuid?: string;
	},
): RoomInfo[] {
	return prev.map((r) => {
		if (!matchesRoom(r, data.room, data.guild_uuid)) return r;
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
	data: { room: string; identity: string; id: string; guild_uuid?: string },
): RoomInfo[] {
	return prev.map((r) => {
		if (!matchesRoom(r, data.room, data.guild_uuid)) return r;
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
		isMicMuted: boolean;
		guild_uuid?: string;
	},
): RoomInfo[] {
	return prev.map((r) => {
		if (!matchesRoom(r, data.room, data.guild_uuid)) return r;
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
	guildUUID: string | undefined,
	ackMembers: MemberInfo[],
): RoomInfo[] {
	const identity: RoomIdentity = { name: roomName, guild_uuid: guildUUID };
	const exists = prev.some((r) => sameRoom(r, identity));
	if (!exists) {
		return [
			...prev,
			{
				id: 0,
				uuid: "",
				name: roomName,
				guild_uuid: guildUUID,
				hasPassword: false,
				limit: 0,
				members: ackMembers,
				count: ackMembers.length,
				createdAt: Date.now(),
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
