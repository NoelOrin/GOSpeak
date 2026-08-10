import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockAdapter, mockTabLock } = vi.hoisted(() => {
	return {
		mockAdapter: {
			isConnected: vi.fn(() => false),
			getCurrentUrl: vi.fn(() => ""),
			getState: vi.fn(() => "new"),
			connect: vi.fn(),
			disconnect: vi.fn(),
			emitFireAndForget: vi.fn(),
			emitAck: vi.fn(() => Promise.resolve({})),
			onServerEvent: vi.fn(() => () => {}),
			offServerEvent: vi.fn(),
			offAllServerEvents: vi.fn(),
			onConnected: vi.fn<(cb: () => void) => () => void>(() => () => {}),
			onDisconnected: vi.fn(() => () => {}),
			onConnectError: vi.fn(() => () => {}),
			onStateChange: vi.fn(
				(_cb: (prev: string, next: string) => void) => () => {},
			),
		},
		mockTabLock: {
			claim: vi.fn(async () => true),
			release: vi.fn(),
			isOwner: vi.fn(() => true),
			ensureListening: vi.fn(),
			setOnForeignClaim: vi.fn(),
		},
	};
});

vi.mock("@/socket/wsClient", () => ({
	createWSClient: () => mockAdapter,
}));

vi.mock("@/api/ws", () => ({
	getWSEndpoint: vi.fn(async () => ({})),
}));

vi.mock("@/socket/tabLock", () => ({
	createTabLock: () => mockTabLock,
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

vi.mock("@/socket/events", () => ({ EVENTS: { ROOM_LEAVE: "room:leave" } }));

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
		isLoggedIn: () => true,
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

const onConnectedCb = mockAdapter.onConnected.mock.calls[0]?.[0] as
	| (() => void)
	| undefined;

const onStateChangeCb = mockAdapter.onStateChange.mock.calls[0]?.[0] as
	| ((prev: string, next: string) => void)
	| undefined;

describe("socketStore worker routing", () => {
	beforeEach(() => {
		socketStore.disconnect();
		vi.clearAllMocks();
		mockAdapter.isConnected.mockReturnValue(false);
		mockAdapter.getCurrentUrl.mockReturnValue("");
		mockTabLock.claim.mockResolvedValue(true);
	});

	it("connectToWorker claims the tab lock and connects to worker URL", async () => {
		const url = await socketStore.connectToWorker("wss://worker-a.example");

		expect(url).toBe("wss://worker-a.example");
		expect(mockAdapter.connect).toHaveBeenCalledWith("wss://worker-a.example");
		expect(mockTabLock.claim).toHaveBeenCalledTimes(1);
		expect(mockAdapter.onServerEvent.mock.calls.length).toBeGreaterThan(0);
	});

	it("connectToWorker rejects when another tab owns the lock", async () => {
		mockTabLock.claim.mockResolvedValueOnce(false);

		await expect(
			socketStore.connectToWorker("wss://worker-a.example"),
		).rejects.toThrow("已在其他标签页连接");
		expect(mockAdapter.connect).not.toHaveBeenCalled();
		expect(mockAdapter.onServerEvent).not.toHaveBeenCalled();
	});

	it("connectToWorker binds server events only once", async () => {
		await socketStore.connectToWorker("wss://worker-a.example");
		const boundCount = mockAdapter.onServerEvent.mock.calls.length;
		expect(boundCount).toBeGreaterThan(0);

		onConnectedCb?.();
		await socketStore.connectToWorker("wss://worker-b.example");

		expect(mockAdapter.connect).toHaveBeenCalledTimes(2);
		expect(mockAdapter.onServerEvent.mock.calls.length).toBe(boundCount);
	});

	it("leaveRoom sends explicit domain uuid", async () => {
		await socketStore.leaveRoom("lobby", "domain-a");

		expect(mockAdapter.emitAck).toHaveBeenCalledWith(
			"room:leave",
			expect.objectContaining({ room: "lobby", domain_uuid: "domain-a" }),
		);
	});

	it("socketState follows adapter state machine", () => {
		expect(socketStore.socketState()).toBe("new");
		onStateChangeCb?.("new", "connecting");
		expect(socketStore.socketState()).toBe("connecting");
		onStateChangeCb?.("connecting", "open");
		expect(socketStore.socketState()).toBe("open");
		onStateChangeCb?.("open", "closing");
		expect(socketStore.socketState()).toBe("closing");
	});
});
