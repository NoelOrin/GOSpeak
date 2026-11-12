import { afterEach, describe, expect, it, vi } from "vitest";

const socketMock = vi.hoisted(() => ({
	isConnected: vi.fn(() => true),
	emitFireAndForget: vi.fn(),
	emitAck: vi.fn(() => Promise.resolve({})),
	onServerEvent: vi.fn(() => () => {}),
	offAllServerEvents: vi.fn(),
}));

const storeMock = vi.hoisted(() => ({
	joinRoom: vi.fn((..._args: unknown[]) => Promise.resolve({})),
}));

const domainState = vi.hoisted(() => ({ currentDomain: "fallback-domain" }));

vi.mock("@/stores/socketStore", () => ({
	socketStore: {
		currentDomainUUID: () => domainState.currentDomain,
		getSocket: () => socketMock,
		// room:join 现在走 socketStore.joinRoom 的 ack 路径，而不是
		// fire-and-forget —— 后者会吞掉密码错 / 房间满等拒绝。
		joinRoom: storeMock.joinRoom,
	},
	setPrivateNewHandler: vi.fn(),
	setMessageCreatedHandler: vi.fn(),
}));
vi.mock("@/api/message", () => ({
	listMessages: vi.fn(async () => ({ items: [], has_more: false })),
}));
vi.mock("idb-keyval", () => ({
	get: vi.fn(async () => undefined),
	set: vi.fn(async () => {}),
	del: vi.fn(async () => {}),
}));

import { listMessages } from "@/api/message";
import { EVENTS } from "@/socket/events";
import { chatStore, useChatStore } from "./chatStore";

describe("chatStore domain payloads", () => {
	afterEach(() => {
		vi.clearAllMocks();
		domainState.currentDomain = "fallback-domain";
		socketMock.isConnected.mockReturnValue(true);
		storeMock.joinRoom.mockResolvedValue({});
		chatStore.leaveTextRoom();
	});

	it("sends domain_uuid on room join and message send", async () => {
		domainState.currentDomain = "domain-1";
		await chatStore.joinTextRoom({
			uuid: "room-1",
			name: "text-chat",
			domain_uuid: "domain-1",
		});

		expect(storeMock.joinRoom).toHaveBeenCalledWith(
			"text-chat",
			"",
			undefined,
			"domain-1",
		);

		chatStore.send("hello");
		expect(socketMock.emitAck).toHaveBeenCalledWith(
			EVENTS.MESSAGE_SEND,
			expect.objectContaining({ domain_uuid: "domain-1" }),
		);

		chatStore.deleteMessage("msg-1");
		expect(socketMock.emitFireAndForget).toHaveBeenCalledWith(
			EVENTS.MESSAGE_DELETE,
			expect.objectContaining({ domain_uuid: "domain-1" }),
		);
	});

	it("falls back to the current domain uuid", async () => {
		domainState.currentDomain = "fallback-domain";
		await chatStore.joinTextRoom({
			uuid: "room-2",
			name: "text-chat-2",
		});

		expect(storeMock.joinRoom).toHaveBeenCalledWith(
			"text-chat-2",
			"",
			undefined,
			"fallback-domain",
		);
	});

	it("leaveTextRoom emits room leave with saved domain", async () => {
		domainState.currentDomain = "domain-1";
		await chatStore.joinTextRoom({
			uuid: "room-1",
			name: "text-chat",
			domain_uuid: "domain-1",
		});
		socketMock.emitFireAndForget.mockClear();

		chatStore.leaveTextRoom();

		expect(socketMock.emitFireAndForget).toHaveBeenCalledWith(
			EVENTS.ROOM_LEAVE,
			expect.objectContaining({ room: "text-chat", domain_uuid: "domain-1" }),
		);
	});

	it("gives a clear error instead of waiting for an ack while offline", async () => {
		socketMock.isConnected.mockReturnValue(false);
		await chatStore.joinTextRoom({
			uuid: "room-3",
			name: "text-chat-3",
			domain_uuid: "domain-1",
		});

		expect(storeMock.joinRoom).not.toHaveBeenCalled();
		expect(listMessages).not.toHaveBeenCalled();
		expect(useChatStore.getState().textRoomError).toBe(
			"尚未连接服务器，请稍后重试",
		);
		expect(useChatStore.getState().loading).toBe(false);
	});

	it("surfaces non-password join failures without loading history", async () => {
		storeMock.joinRoom.mockRejectedValueOnce(new Error("room is full"));
		await chatStore.joinTextRoom({
			uuid: "room-4",
			name: "text-chat-4",
			domain_uuid: "domain-1",
		});

		expect(listMessages).not.toHaveBeenCalled();
		expect(useChatStore.getState().textRoomError).toBeTruthy();
		expect(useChatStore.getState().textRoomNeedsPassword).toBe(false);
	});

	it("passes a new password on retry so protected history can be read", async () => {
		await chatStore.joinTextRoom({
			uuid: "room-5",
			name: "text-chat-5",
			domain_uuid: "domain-1",
		});
		storeMock.joinRoom.mockClear();

		await chatStore.retryTextRoomWithPassword("secret");

		expect(storeMock.joinRoom).toHaveBeenCalledWith(
			"text-chat-5",
			"",
			"secret",
			"domain-1",
		);
		expect(listMessages).toHaveBeenCalledWith(
			"room-5",
			undefined,
			expect.any(Number),
			"secret",
		);
	});
});
