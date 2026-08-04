import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockAdapter } = vi.hoisted(() => {
	return {
		mockAdapter: {
			isConnected: vi.fn(() => false),
			getCurrentUrl: vi.fn(() => ""),
			connect: vi.fn(),
			disconnect: vi.fn(),
			emitFireAndForget: vi.fn(),
			emitAck: vi.fn(),
			onServerEvent: vi.fn(() => () => {}),
			offServerEvent: vi.fn(),
			offAllServerEvents: vi.fn(),
			onConnected: vi.fn(() => () => {}),
			onDisconnected: vi.fn(() => () => {}),
			onConnectError: vi.fn(() => () => {}),
		},
	};
});

vi.mock("@/socket/wsClient", () => ({
	createWSClient: () => mockAdapter,
}));

vi.mock("@/api/ws", () => ({
	getWSTicket: vi.fn(async () => "ticket"),
}));

vi.mock("@/socket/tabLock", () => ({
	createTabLock: () => ({
		claim: vi.fn(async () => true),
		release: vi.fn(),
		isOwner: vi.fn(() => true),
		ensureListening: vi.fn(),
		setOnForeignClaim: vi.fn(),
	}),
}));

vi.mock("@/socket/providerReload", () => ({
	createProviderReloadHandler: () => vi.fn(),
}));

vi.mock("@/components/room/services/loadSfuClient", () => ({
	preloadSfuClient: vi.fn(),
}));

vi.mock("@/handler_audio/speakingStore", () => ({
	setSpeakingIdentities: vi.fn(),
}));

vi.mock("@/socket/events", () => ({ EVENTS: {} }));

vi.mock("@/socket/roomState", () => ({
	addCreatedRoom: vi.fn((prev) => prev),
	applyMemberJoinedShell: vi.fn((prev) => prev),
	applyMemberLeft: vi.fn((prev) => prev),
	applyMemberUpdated: vi.fn((prev) => prev),
	mergeRoomUpdated: vi.fn((prev) => prev),
	upsertRoomMembersFromAck: vi.fn((prev) => prev),
}));

vi.mock("@/stores/chatStore", () => ({
	chatStore: { handlePrivateNew: vi.fn() },
}));

vi.mock("@/stores/userStore", () => ({
	default: {
		accessToken: () => "token",
		user: () => ({ id: "1", name: "alice" }),
	},
}));

vi.mock("@/stores/domainStore", () => ({
	default: { state: { currentDomainUUID: null } },
}));

vi.mock("solid-notifications", () => ({
	showToast: vi.fn(),
}));

import { socketStore } from "./socketStore";

describe("socketStore worker routing", () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mockAdapter.isConnected.mockReturnValue(false);
		mockAdapter.getCurrentUrl.mockReturnValue("");
	});

	it("connectToWorker uses worker URL before default socket URL", async () => {
		const url = await socketStore.connectToWorker("wss://worker-a.example");

		expect(url).toBe("wss://worker-a.example");
		expect(mockAdapter.connect).toHaveBeenCalledWith(
			"wss://worker-a.example",
			"ticket",
		);
	});
});
