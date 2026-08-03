import type { RoomRecord } from "@/api/room";
import type { RoomInfo } from "@/socket/types";

export function visibleRoomsForDomain(
	rooms: RoomInfo[],
	domainUUID: string | null | undefined,
): RoomInfo[] {
	if (!domainUUID) return [];
	return rooms.filter((room) => room.domain_uuid === domainUUID);
}

export function canEditRoomItem(
	currentUser: { uuid?: string } | null,
	domain: { owner_uuid?: string } | null,
	memberRole: string | null | undefined,
	hasRoomUpdatePermission: boolean,
) {
	return (
		hasRoomUpdatePermission ||
		(!!currentUser?.uuid && domain?.owner_uuid === currentUser.uuid) ||
		memberRole === "admin"
	);
}

export function canDeleteRoomItem(
	currentUser: { uuid?: string } | null,
	domain: { owner_uuid?: string } | null,
	memberRole: string | null | undefined,
	hasRoomDeletePermission: boolean,
) {
	return (
		hasRoomDeletePermission ||
		(!!currentUser?.uuid && domain?.owner_uuid === currentUser.uuid) ||
		memberRole === "admin"
	);
}

export function toEditRoomRecord(room: RoomInfo): RoomRecord {
	return {
		id: room.id,
		uuid: room.uuid,
		name: room.name,
		description: room.description ?? "",
		limit: room.limit,
		audio_only: room.audioOnly ?? true,
		allow_audience: room.allowAudience ?? true,
		type:
			room.type === "text"
				? "text"
				: room.type === "voice"
					? "voice"
					: undefined,
		domain_uuid: room.domain_uuid,
	};
}
