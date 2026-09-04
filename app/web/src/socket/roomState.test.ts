import { describe, expect, it } from "vitest";
import {
	addCreatedRoom,
	applyMemberJoinedShell,
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
	displayName: identity,
	avatar: "",
	isMuted: false,
	isMicMuted: false,
	joinedAt: 1,
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
	hasPassword: false,
	description: "desc",
	limit: 10,
	audioOnly: true,
	allowAudience: false,
	members,
	count: members.length,
	createdAt: 100,
});

describe("roomState reducers", () => {
	it("mergeRoomUpdated preserves DB fields and overwrites live fields", () => {
		const prev = [room("lobby", [member("a")])];
		const incoming = {
			name: "lobby",
			hasPassword: true,
			members: [member("a"), member("b")],
			count: 2,
			createdAt: 200,
			// 这些零值不应覆盖本地 DB 字段
			id: 0,
			uuid: "",
			description: undefined,
			limit: 0,
		} as RoomInfo;

		const next = mergeRoomUpdated(prev, incoming);
		expect(next[0].description).toBe("desc");
		expect(next[0].limit).toBe(10);
		expect(next[0].audioOnly).toBe(true);
		expect(next[0].hasPassword).toBe(true);
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
			hasPassword: false,
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

	it("applyMemberJoinedShell appends shell member without wiping existing ones", () => {
		const prev = [room("lobby", [member("a", { displayName: "Alice" })])];
		const next = applyMemberJoinedShell(prev, {
			room: "lobby",
			identity: "b",
			id: "id-b",
			stream: "s1",
		});
		expect(next[0].members.map((m) => m.identity)).toEqual(["a", "b"]);
		expect(next[0].members[0].displayName).toBe("Alice");
		expect(next[0].members[1].name).toBe("");
		expect(next[0].members[1].displayName).toBe("");
		expect(next[0].members[1].stream).toBe("s1");
		expect(next[0].count).toBe(2);
	});

	it("applyMemberJoinedShell only updates the matching domain room", () => {
		const prev = [room("lobby", [], "domain-a"), room("lobby", [], "domain-b")];
		const next = applyMemberJoinedShell(prev, {
			room: "lobby",
			domain_uuid: "domain-b",
			identity: "b",
			id: "id-b",
		});
		expect(next[0].count).toBe(0);
		expect(next[1].count).toBe(1);
		expect(next[1].members[0].identity).toBe("b");
	});

	it("applyMemberJoinedShell is idempotent for same id but still bumps count like current store", () => {
		const prev = [room("lobby", [member("a", { id: "id-a" })])];
		const once = applyMemberJoinedShell(prev, {
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
			isMicMuted: true,
		});
		expect(next[0].members.find((m) => m.identity === "b")?.isMicMuted).toBe(
			true,
		);
		expect(next[0].members.find((m) => m.identity === "a")?.isMicMuted).toBe(
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
			isMicMuted: true,
		});
		expect(next[0].members[0].isMicMuted).toBe(true);
		expect(next[1].members[0].isMicMuted).toBe(false);
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
