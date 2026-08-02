import { afterEach, describe, expect, it, vi } from "vitest";

const socketMock = vi.hoisted(() => ({
	isConnected: vi.fn(() => true),
	emitFireAndForget: vi.fn(),
	emitAck: vi.fn(() => Promise.resolve({})),
	onServerEvent: vi.fn(() => () => {}),
	offAllServerEvents: vi.fn(),
}));

vi.mock("@/stores/socketStore", () => ({
	socketStore: {
		currentDomainUUID: () => "fallback-domain",
		getSocket: () => socketMock,
	},
}));
vi.mock("@/api/message", () => ({
	listMessages: vi.fn(async () => ({ items: [], has_more: false })),
}));
vi.mock("idb-keyval", () => ({
	get: vi.fn(async () => undefined),
	set: vi.fn(async () => {}),
	del: vi.fn(async () => {}),
}));

import { EVENTS } from "@/socket/events";
import { chatStore } from "./chatStore";

describe("chatStore domain payloads", () => {
	afterEach(() => {
		vi.clearAllMocks();
		chatStore.leaveTextRoom();
	});

	it("sends domain_uuid on room join and message send", async () => {
		await chatStore.joinTextRoom({
			uuid: "room-1",
			name: "text-chat",
			domain_uuid: "domain-1",
		});

		expect(socketMock.emitFireAndForget).toHaveBeenCalledWith(
			EVENTS.ROOM_JOIN,
			expect.objectContaining({ domain_uuid: "domain-1" }),
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
		await chatStore.joinTextRoom({
			uuid: "room-2",
			name: "text-chat-2",
		});

		expect(socketMock.emitFireAndForget).toHaveBeenCalledWith(
			EVENTS.ROOM_JOIN,
			expect.objectContaining({ domain_uuid: "fallback-domain" }),
		);
	});
});
