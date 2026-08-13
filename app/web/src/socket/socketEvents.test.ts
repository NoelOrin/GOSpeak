import { beforeEach, describe, expect, it, vi } from "vitest";
import { EVENTS } from "@/socket/events";
import type { SocketEventDeps } from "@/socket/socketEvents";

vi.mock("@/handler_audio", () => ({
	setServerMutedByIdentity: vi.fn(),
}));

type Handler = (data: any) => void;

function createFakeAdapter() {
	const handlers = new Map<string, Handler>();
	return {
		onServerEvent(event: string, handler: Handler) {
			handlers.set(event, handler);
		},
		emit(event: string, data: any) {
			handlers.get(event)?.(data);
		},
		disconnect() {},
	};
}

function fakeDeps(): SocketEventDeps {
	return {
		setRooms: vi.fn(),
		setCurrentRoom: vi.fn(),
		setSelectedRoomInfo: vi.fn(),
		setActiveSFUProvider: vi.fn(),
		setConnecting: vi.fn(),
		setConnected: vi.fn(),
		setSpeechRestricted: vi.fn(),
		setSpeechRestrictionInfo: vi.fn(),
		setSpeakingIdentities: vi.fn(),
		currentRoom: () => null,
		currentDomainUUID: () => null,
		emitActivity: vi.fn(),
		emitPresence: vi.fn(),
		emitKicked: vi.fn(),
		handleProviderChanged: vi.fn(),
		handlePrivateNew: vi.fn(),
		currentUserName: () => "",
		currentUserID: () => undefined,
	};
}

describe("server mute socket events", () => {
	beforeEach(async () => {
		const { setServerMutedByIdentity } = await import("@/handler_audio");
		vi.mocked(setServerMutedByIdentity).mockClear();
	});

	it("member:muted / member:unmuted drive server mute by identity", async () => {
		const { setServerMutedByIdentity } = await import("@/handler_audio");
		const { bindServerEvents } = await import("@/socket/socketEvents");
		const adapter = createFakeAdapter();
		bindServerEvents(adapter as any, fakeDeps());

		adapter.emit(EVENTS.MEMBER_MUTED, { identity: "alice", muted: true });
		expect(setServerMutedByIdentity).toHaveBeenCalledWith("alice", true);

		adapter.emit(EVENTS.MEMBER_UNMUTED, { identity: "alice", muted: false });
		expect(setServerMutedByIdentity).toHaveBeenCalledWith("alice", false);
	});

	it("ignores events without identity", async () => {
		const { setServerMutedByIdentity } = await import("@/handler_audio");
		const { bindServerEvents } = await import("@/socket/socketEvents");
		const adapter = createFakeAdapter();
		bindServerEvents(adapter as any, fakeDeps());

		adapter.emit(EVENTS.MEMBER_MUTED, {});
		expect(setServerMutedByIdentity).not.toHaveBeenCalled();
	});

	it("member:muted honors payload muted flag", async () => {
		const { setServerMutedByIdentity } = await import("@/handler_audio");
		vi.mocked(setServerMutedByIdentity).mockClear();
		const { bindServerEvents } = await import("@/socket/socketEvents");
		const adapter = createFakeAdapter();
		bindServerEvents(adapter as any, fakeDeps());

		adapter.emit(EVENTS.MEMBER_MUTED, { identity: "bob", muted: false });
		expect(setServerMutedByIdentity).toHaveBeenCalledWith("bob", false);
	});

	it("member:unmuted honors payload muted flag", async () => {
		const { setServerMutedByIdentity } = await import("@/handler_audio");
		vi.mocked(setServerMutedByIdentity).mockClear();
		const { bindServerEvents } = await import("@/socket/socketEvents");
		const adapter = createFakeAdapter();
		bindServerEvents(adapter as any, fakeDeps());

		adapter.emit(EVENTS.MEMBER_UNMUTED, { identity: "bob", muted: true });
		expect(setServerMutedByIdentity).toHaveBeenCalledWith("bob", true);
	});
});
