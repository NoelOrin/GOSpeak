import { beforeEach, describe, expect, it, vi } from "vitest";
import { CapabilityRouter } from "./capabilityRouter";

// ─── Mocks ───

function createMockApi() {
	return {
		listRooms: vi.fn().mockResolvedValue([]),
		getMembers: vi.fn().mockResolvedValue([]),
		createRoom: vi.fn().mockResolvedValue({ id: "r1", name: "test" }),
		getUserByIdentity: vi
			.fn()
			.mockResolvedValue({ id: 1, uuid: "u1", name: "alice", role: "user" }),
		muteUser: vi.fn().mockResolvedValue(undefined),
		unmuteUser: vi.fn().mockResolvedValue(undefined),
	};
}

function createMockSocket() {
	return {
		sendBotMessage: vi.fn(),
		kickMember: vi.fn(),
	};
}

function createMockRoomJoiner() {
	return {
		joinRoom: vi.fn().mockResolvedValue(undefined),
		leaveRoom: vi.fn(),
		joinedRooms: [] as string[],
	};
}

describe("CapabilityRouter", () => {
	let api: ReturnType<typeof createMockApi>;
	let socket: ReturnType<typeof createMockSocket>;
	let joiner: ReturnType<typeof createMockRoomJoiner>;
	let router: CapabilityRouter;

	beforeEach(() => {
		api = createMockApi();
		socket = createMockSocket();
		joiner = createMockRoomJoiner();
		router = new CapabilityRouter(api as any, socket as any, joiner);
	});

	describe("chat", () => {
		it("send emits bot:message via socket", () => {
			router.send("room-1", "hello");
			expect(socket.sendBotMessage).toHaveBeenCalledWith("room-1", "hello");
		});

		it("reply emits bot:message via socket with room id", () => {
			router.reply(
				{ room: { id: "room-2" }, sender: { identity: "alice" } },
				"reply text",
			);
			expect(socket.sendBotMessage).toHaveBeenCalledWith(
				"room-2",
				"reply text",
			);
		});
	});

	describe("rooms", () => {
		it("listRooms delegates to api", async () => {
			await router.listRooms();
			expect(api.listRooms).toHaveBeenCalled();
		});

		it("getMembers delegates to api", async () => {
			await router.getMembers("room-1");
			expect(api.getMembers).toHaveBeenCalledWith("room-1");
		});

		it("createRoom delegates to api", async () => {
			const result = await router.createRoom("new-room", 10);
			expect(api.createRoom).toHaveBeenCalledWith("new-room", 10);
			expect(result.name).toBe("test");
		});

		it("join delegates to roomJoiner", async () => {
			await router.join("voice-chat", { sfu: true });
			expect(joiner.joinRoom).toHaveBeenCalledWith("voice-chat", {
				sfu: true,
			});
		});

		it("leave delegates to roomJoiner", () => {
			router.leave("voice-chat");
			expect(joiner.leaveRoom).toHaveBeenCalledWith("voice-chat");
		});

		it("joined returns roomJoiner.joinedRooms", () => {
			joiner.joinedRooms = ["room-1", "room-2"];
			expect(router.joined()).toEqual(["room-1", "room-2"]);
		});
	});

	describe("voice", () => {
		it("removeMember emits room:kick via socket", () => {
			router.removeMember("room-1", "alice");
			expect(socket.kickMember).toHaveBeenCalledWith("room-1", "alice");
		});

		it("muteMember looks up user and calls muteUser", async () => {
			await router.muteMember("room-1", "alice", true);
			expect(api.getUserByIdentity).toHaveBeenCalledWith("alice");
			expect(api.muteUser).toHaveBeenCalledWith(
				1,
				0,
				false,
				expect.stringContaining("room-1"),
			);
		});

		it("unmuteMember looks up user and calls unmuteUser", async () => {
			await router.muteMember("room-1", "bob", false);
			expect(api.getUserByIdentity).toHaveBeenCalledWith("bob");
			expect(api.unmuteUser).toHaveBeenCalledWith(1);
		});
	});

	describe("users", () => {
		it("getUserByIdentity delegates to api", async () => {
			const result = await router.getUserByIdentity("alice");
			expect(api.getUserByIdentity).toHaveBeenCalledWith("alice");
			expect(result.name).toBe("alice");
		});
	});
});
