import { describe, expect, it } from "vitest";
import {
	addCreatedRoom,
	applyMemberJoined,
	applyMemberLeft,
	applyMemberUpdated,
	mergeRoomUpdated,
	upsertRoomMembersFromAck,
} from "./roomState";
import type { MemberInfo, RoomInfo } from "./types";

const member = (
	identity: string,
	extra: Partial<MemberInfo> = {},
): MemberInfo => ({
	id: identity,
	identity,
	name: identity,
	display_name: identity,
	avatar: "",
	is_muted: false,
	is_mic_muted: false,
	joined_at: 1,
	...extra,
});

const room = (
	name: string,
	members: MemberInfo[] = [],
	domain_uuid?: string,
): RoomInfo => ({
	id: 1,
	uuid: "u1",
	name,
	domain_uuid,
	has_password: false,
	description: "desc",
	limit: 10,
	audio_only: true,
	allow_audience: false,
	members,
	count: members.length,
	created_at: 100,
});

describe("roomState reducers", () => {
	it("mergeRoomUpdated preserves DB fields and overwrites live fields", () => {
		const prev = [room("lobby", [member("a")])];
		const incoming = {
			name: "lobby",
			has_password: true,
			members: [member("a"), member("b")],
			count: 2,
			created_at: 200,
			// 这些零值不应覆盖本地 DB 字段
			id: 0,
			uuid: "",
			description: undefined,
			limit: 0,
		} as RoomInfo;

		const next = mergeRoomUpdated(prev, incoming);
		expect(next[0].description).toBe("desc");
		expect(next[0].limit).toBe(10);
		expect(next[0].audio_only).toBe(true);
		expect(next[0].has_password).toBe(true);
		expect(next[0].members.map((m) => m.identity)).toEqual(["a", "b"]);
		expect(next[0].count).toBe(2);
		expect(next[0].id).toBe(1);
		expect(next[0].uuid).toBe("u1");
	});

	it("mergeRoomUpdated keeps same room names isolated by domain", () => {
		const prev = [room("lobby", [member("a")], "domain-a")];
		const next = mergeRoomUpdated(prev, {
			...room("lobby", [member("b")], "domain-b"),
			id: 2,
			uuid: "u2",
		});

		expect(next).toHaveLength(2);
		expect(next[0].domain_uuid).toBe("domain-a");
		expect(next[0].members.map((m) => m.identity)).toEqual(["a"]);
		expect(next[1].domain_uuid).toBe("domain-b");
		expect(next[1].members.map((m) => m.identity)).toEqual(["b"]);
	});

	it("mergeRoomUpdated inserts missing room shell", () => {
		const next = mergeRoomUpdated([], {
			name: "new-room",
			has_password: false,
			members: [member("x")],
			count: 1,
		} as RoomInfo);
		expect(next).toHaveLength(1);
		expect(next[0].name).toBe("new-room");
		expect(next[0].members).toHaveLength(1);
	});

	it("mergeRoomUpdated ignores rooms without name", () => {
		const prev = [room("lobby")];
		const next = mergeRoomUpdated(prev, { name: "" } as RoomInfo);
		expect(next).toBe(prev);
	});

	it("applyMemberJoined only bumps count and never fabricates member shells", () => {
		const prev = [room("lobby", [member("a", { display_name: "Alice" })])];
		const next = applyMemberJoined(prev, {
			room: "lobby",
			identity: "b",
			id: "id-b",
			stream: "s1",
		});
		// 成员明细不本地造壳：等 room:updated / join ack 全量补齐
		expect(next[0].members.map((m) => m.identity)).toEqual(["a"]);
		expect(next[0].members[0].display_name).toBe("Alice");
		expect(next[0].count).toBe(2);
	});

	it("applyMemberJoined only updates the matching domain room", () => {
		const prev = [room("lobby", [], "domain-a"), room("lobby", [], "domain-b")];
		const next = applyMemberJoined(prev, {
			room: "lobby",
			domain_uuid: "domain-b",
			identity: "b",
			id: "id-b",
		});
		expect(next[0].count).toBe(0);
		expect(next[1].count).toBe(1);
		expect(next[1].members).toHaveLength(0);
	});

	it("applyMemberJoined bumps count even for an already present member", () => {
		const prev = [room("lobby", [member("a", { id: "id-a" })])];
		const once = applyMemberJoined(prev, {
			room: "lobby",
			identity: "a",
			id: "id-a",
		});
		expect(once[0].members).toHaveLength(1);
		// preserve existing store behavior: count always increments
		expect(once[0].count).toBe(2);
	});

	it("applyMemberLeft removes member by id and updates count", () => {
		const prev = [
			room("lobby", [member("a", { id: "id-a" }), member("b", { id: "id-b" })]),
		];
		const next = applyMemberLeft(prev, {
			room: "lobby",
			identity: "a",
			id: "id-a",
		});
		expect(next[0].members.map((m) => m.identity)).toEqual(["b"]);
		expect(next[0].count).toBe(1);
	});

	it("applyMemberLeft only removes from the matching domain room", () => {
		const prev = [
			room("lobby", [member("a", { id: "id-a" })], "domain-a"),
			room("lobby", [member("a", { id: "id-a" })], "domain-b"),
		];
		const next = applyMemberLeft(prev, {
			room: "lobby",
			domain_uuid: "domain-b",
			identity: "a",
			id: "id-a",
		});
		expect(next[0].members).toHaveLength(1);
		expect(next[1].members).toHaveLength(0);
	});

	it("applyMemberLeft never drives count below zero", () => {
		const prev = [{ ...room("lobby", []), count: 0 }];
		const next = applyMemberLeft(prev, {
			room: "lobby",
			identity: "ghost",
			id: "missing",
		});
		expect(next[0].count).toBe(0);
	});

	it("applyMemberUpdated patches mic flags by identity", () => {
		const prev = [room("lobby", [member("a"), member("b")])];
		const next = applyMemberUpdated(prev, {
			room: "lobby",
			identity: "b",
			is_mic_muted: true,
		});
		expect(next[0].members.find((m) => m.identity === "b")?.is_mic_muted).toBe(
			true,
		);
		expect(next[0].members.find((m) => m.identity === "a")?.is_mic_muted).toBe(
			false,
		);
	});

	it("applyMemberUpdated only updates the matching domain room", () => {
		const prev = [
			room("lobby", [member("a")], "domain-a"),
			room("lobby", [member("a")], "domain-b"),
		];
		const next = applyMemberUpdated(prev, {
			room: "lobby",
			domain_uuid: "domain-a",
			identity: "a",
			is_mic_muted: true,
		});
		expect(next[0].members[0].is_mic_muted).toBe(true);
		expect(next[1].members[0].is_mic_muted).toBe(false);
	});

	it("upsertRoomMembersFromAck replaces members for room and inserts if missing", () => {
		const prev = [room("lobby", [member("old")])];
		const next = upsertRoomMembersFromAck(prev, "lobby", "", [
			member("new1"),
			member("new2"),
		]);
		expect(next[0].members.map((m) => m.identity)).toEqual(["new1", "new2"]);
		expect(next[0].count).toBe(2);

		const created = upsertRoomMembersFromAck([], "r2", "", [member("x")]);
		expect(created[0].name).toBe("r2");
		expect(created[0].members).toHaveLength(1);
	});

	it("upsertRoomMembersFromAck keeps domain rooms isolated by domain_uuid", () => {
		const prev = [room("lobby", [member("old")], "domain-a")];
		const next = upsertRoomMembersFromAck(prev, "lobby", "domain-b", [
			member("new"),
		]);
		expect(next).toHaveLength(2);
		expect(next[0].members.map((m) => m.identity)).toEqual(["old"]);
		expect(next[1].domain_uuid).toBe("domain-b");
		expect(next[1].members.map((m) => m.identity)).toEqual(["new"]);
	});

	it("addCreatedRoom appends unique rooms only", () => {
		const prev = [room("lobby")];
		const created = room("new");
		expect(addCreatedRoom(prev, created)).toHaveLength(2);
		expect(addCreatedRoom(prev, room("lobby"))).toBe(prev);
	});

	it("addCreatedRoom treats same room name in different domains as distinct", () => {
		const prev = [room("lobby", [], "domain-a")];
		const next = addCreatedRoom(prev, room("lobby", [], "domain-b"));
		expect(next).toHaveLength(2);
	});
});
