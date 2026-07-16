import { beforeEach, describe, expect, it, vi } from "vitest";
import { MockListenAdapter } from "./adapters/mockListenAdapter";
import { ListenRoomRegistry } from "./listenRegistry";
import { MediaListenService } from "./listenService";
import { PcmStreamHub } from "./pcmStream";
import { SFUListenRouter } from "./sfuListenRouter";
import type { AudioFrameEvent } from "./types";

const logger = {
	debug: vi.fn(),
	info: vi.fn(),
	warn: vi.fn(),
	error: vi.fn(),
};

describe("MediaListenService", () => {
	let registry: ListenRoomRegistry;
	let hub: PcmStreamHub;
	let mockAdapter: MockListenAdapter;
	let joinSignaling: ReturnType<typeof vi.fn>;
	let leaveSignaling: ReturnType<typeof vi.fn>;
	let getSFUToken: ReturnType<typeof vi.fn>;
	let service: MediaListenService;

	beforeEach(() => {
		registry = new ListenRoomRegistry();
		hub = new PcmStreamHub();
		mockAdapter = new MockListenAdapter();
		joinSignaling = vi.fn().mockResolvedValue(undefined);
		leaveSignaling = vi.fn();
		getSFUToken = vi.fn().mockResolvedValue({
			token: "t",
			serverUrl: "ws://lk",
			provider: "livekit",
		});
		service = new MediaListenService({
			registry,
			pcmSink: hub,
			logger: logger as any,
			identity: "bot",
			getSFUToken: getSFUToken as any,
			joinSignaling: joinSignaling as any,
			leaveSignaling: leaveSignaling as any,
			router: new SFUListenRouter({
				createAdapter: () => mockAdapter,
			}),
		});
	});

	it("joins desired rooms and leaves removed ones", async () => {
		await service.start();
		registry.add("lobby");
		await service.reconcile();
		expect(joinSignaling).toHaveBeenCalledWith("lobby");
		expect(getSFUToken).toHaveBeenCalledWith("lobby");
		expect(service.activeRooms).toEqual(["lobby"]);

		registry.remove("lobby");
		await service.reconcile();
		expect(leaveSignaling).toHaveBeenCalledWith("lobby");
		expect(service.activeRooms).toEqual([]);
		await service.stop();
	});

	it("publishes frames to pcm hub and filters bot identity", async () => {
		const frames: AudioFrameEvent[] = [];
		hub.subscribe((f) => frames.push(f));
		await service.start();
		registry.add("r1");
		await service.reconcile();

		mockAdapter.injectFrame({
			room: "r1",
			identity: "alice",
			pcm16: new Int16Array([1, 2, 3]),
			sampleRate: 16000,
			channels: 1,
			timestamp: Date.now(),
			mediaProvider: "livekit",
		});
		mockAdapter.injectFrame({
			room: "r1",
			identity: "bot",
			pcm16: new Int16Array([9]),
			sampleRate: 16000,
			channels: 1,
			timestamp: Date.now(),
			mediaProvider: "livekit",
		});

		expect(frames).toHaveLength(1);
		expect(frames[0].identity).toBe("alice");
		await service.stop();
	});
});
