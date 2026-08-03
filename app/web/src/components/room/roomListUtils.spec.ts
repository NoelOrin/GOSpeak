import { describe, expect, it } from "vitest";
import type { RoomInfo } from "@/socket/types";
import {
	canDeleteRoomItem,
	canEditRoomItem,
	toEditRoomRecord,
	visibleRoomsForDomain,
} from "./roomListUtils";

const room: RoomInfo = {
	id: 1,
	uuid: "room-1",
	name: "lobby",
	domain_uuid: "domain-1",
	hasPassword: true,
	description: "主厅",
	limit: 12,
	audioOnly: false,
	allowAudience: true,
	type: "voice",
	members: [],
	count: 0,
	createdAt: 0,
};

describe("canEditRoomItem", () => {
	it("grants edit access to domain owners", () => {
		expect(
			canEditRoomItem(
				{ uuid: "user-1" },
				{ owner_uuid: "user-1" },
				"member",
				false,
			),
		).toBe(true);
	});

	it("grants edit access to domain admins", () => {
		expect(
			canEditRoomItem(
				{ uuid: "user-1" },
				{ owner_uuid: "user-2" },
				"admin",
				false,
			),
		).toBe(true);
	});

	it("grants edit access to users with room:update", () => {
		expect(canEditRoomItem({ uuid: "user-1" }, null, "member", true)).toBe(
			true,
		);
	});

	it("denies ordinary members without room:update", () => {
		expect(
			canEditRoomItem(
				{ uuid: "user-1" },
				{ owner_uuid: "user-2" },
				"member",
				false,
			),
		).toBe(false);
	});
});

describe("canDeleteRoomItem", () => {
	it("grants delete access to domain owners", () => {
		expect(
			canDeleteRoomItem(
				{ uuid: "user-1" },
				{ owner_uuid: "user-1" },
				"member",
				false,
			),
		).toBe(true);
	});

	it("grants delete access to domain admins", () => {
		expect(
			canDeleteRoomItem(
				{ uuid: "user-1" },
				{ owner_uuid: "user-2" },
				"admin",
				false,
			),
		).toBe(true);
	});

	it("grants delete access to users with room:delete", () => {
		expect(canDeleteRoomItem({ uuid: "user-1" }, null, "member", true)).toBe(
			true,
		);
	});

	it("denies ordinary members without room:delete", () => {
		expect(
			canDeleteRoomItem(
				{ uuid: "user-1" },
				{ owner_uuid: "user-2" },
				"member",
				false,
			),
		).toBe(false);
	});
});

describe("toEditRoomRecord", () => {
	it("maps socket room data to the edit API payload shape", () => {
		expect(toEditRoomRecord(room)).toEqual({
			id: 1,
			uuid: "room-1",
			name: "lobby",
			description: "主厅",
			limit: 12,
			audio_only: false,
			allow_audience: true,
			type: "voice",
			domain_uuid: "domain-1",
		});
	});

	it("defaults missing media flags for older room payloads", () => {
		const legacy: RoomInfo = {
			id: 2,
			uuid: "room-2",
			name: "text",
			hasPassword: false,
			limit: 10,
			type: "text",
			members: [],
			count: 0,
			createdAt: 0,
		};

		expect(toEditRoomRecord(legacy)).toMatchObject({
			audio_only: true,
			allow_audience: true,
			type: "text",
		});
	});
});

describe("visibleRoomsForDomain", () => {
	const orphan: RoomInfo = {
		id: 2,
		uuid: "orphan-1",
		name: "orphan",
		domain_uuid: "",
		hasPassword: false,
		limit: 10,
		type: "voice",
		members: [],
		count: 0,
		createdAt: 0,
	};

	it("returns no rooms before a domain is selected", () => {
		expect(visibleRoomsForDomain([room, orphan], undefined)).toEqual([]);
	});

	it("only returns rooms that belong to the selected domain", () => {
		expect(visibleRoomsForDomain([room, orphan], "domain-1")).toEqual([room]);
	});
});
