import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { SRSSFUClient } from "./srs-client";
import type { JoinParams } from "./types";

// RTCPeerConnection mock
function makeMockPc() {
	const handlers: Record<string, ((...args: unknown[]) => void)[]> = {};
	let onTrackHandler: ((ev: unknown) => void) | null = null;
	const mockTrack = { kind: "audio", id: "mock-track", addEventListener: vi.fn() };
	const mockStream = { id: "mock-stream" };
	const pc: Record<string, unknown> = {
		iceConnectionState: "connected",
		addTrack: vi.fn(),
		addTransceiver: vi.fn(),
		createOffer: vi.fn().mockResolvedValue({ type: "offer", sdp: "v=0\r\n" }),
		setLocalDescription: vi.fn().mockResolvedValue(undefined),
		setRemoteDescription: vi.fn((desc: { type: string }) => {
			if (desc.type === "answer") {
				setTimeout(() => {
					if (onTrackHandler) {
						onTrackHandler({ track: mockTrack, streams: [mockStream] });
					}
				}, 0);
			}
			return Promise.resolve();
		}),
		addEventListener: vi.fn((ev: string, cb: (...args: unknown[]) => void) => {
			(handlers[ev] ||= []).push(cb);
		}),
		removeEventListener: vi.fn(),
		close: vi.fn(),
		get ontrack() {
			return onTrackHandler;
		},
		set ontrack(h: ((ev: unknown) => void) | null) {
			onTrackHandler = h;
		},
		fireIce: (state: string) => {
			pc.iceConnectionState = state;
			(handlers.iceconnectionstatechange || []).forEach((cb) => cb());
		},
	};
	return pc;
}

beforeEach(() => {
	(globalThis as any).RTCPeerConnection = vi.fn(makeMockPc);
	Object.defineProperty(globalThis, "navigator", {
		value: {
			mediaDevices: {
				getUserMedia: vi.fn().mockResolvedValue({
					getAudioTracks: () => [{ kind: "audio", stop: vi.fn() }],
					getTracks: () => [{ kind: "audio", stop: vi.fn() }],
				}),
			},
		},
		writable: true,
		configurable: true,
	});
	(globalThis as any).AudioContext = vi.fn(() => ({
		createMediaStreamSource: vi.fn(),
		createAnalyser: vi.fn(() => ({ fftSize: 0, getByteFrequencyData: () => {} })),
		close: vi.fn(),
	}));
	vi.useRealTimers();
});

afterEach(() => {
	vi.useRealTimers();
	vi.restoreAllMocks();
});

function makeFetch(whipOk: boolean, whepOk: boolean) {
	return vi.fn(async (url: string) => {
		const isWhep = String(url).includes("/whep/");
		const ok = isWhep ? whepOk : whipOk;
		if (!ok) {
			throw new Error("Failed to fetch");
		}
		return {
			ok: true,
			status: 201,
			headers: {
				get: (k: string) =>
					k === "Location"
						? "/rtc/v1/whip/?action=delete&token=t"
						: "",
			},
			text: async () => "v=0\r\n",
		};
	});
}

describe("SRSSFUClient subscribePeers", () => {
	it("subscribes peer and emits onRemoteAudioTrack with real identity", async () => {
		(globalThis as any).fetch = makeFetch(true, true);
		const client = new SRSSFUClient({});
		const onTrack = vi.fn();
		client.onRemoteAudioTrack(onTrack);
		const joinParams: JoinParams = {
			token: "tok",
			serverUrl: "http://h:1985/rtc/v1/whip/",
			identity: "alice",
			room: "room1",
			stream: "gs-alice",
			streamToken: "st",
		};
		await client.joinRoom(joinParams);
		(client as any).subscribePeers([{ identity: "bob", stream: "gs-bob" }]);
		await new Promise((r) => setTimeout(r, 50));
		expect(onTrack).toHaveBeenCalledWith(
			expect.objectContaining({ identity: "bob" }),
		);
		await client.leaveRoom();
	});

	it("retries WHEP on failure then removes peer after exhaustion", async () => {
		vi.useFakeTimers();
		try {
			(globalThis as any).fetch = makeFetch(true, false);
			const client = new SRSSFUClient({});
			const removed = vi.fn();
			client.onRemoteAudioTrackRemoved(removed);
			const joinParams: JoinParams = {
				token: "tok",
				serverUrl: "http://h:1985/rtc/v1/whip/",
				identity: "alice",
				room: "room1",
				stream: "gs-alice",
				streamToken: "st",
			};
			await client.joinRoom(joinParams);
			(client as any).subscribePeers([
				{ identity: "bob", stream: "gs-bob" },
			]);
			await vi.runAllTimersAsync();
			expect(removed).toHaveBeenCalledWith("bob");
		} finally {
			vi.useRealTimers();
		}
	});

	it("unsubscribePeer closes pc and emits onRemoteAudioTrackRemoved", async () => {
		(globalThis as any).fetch = makeFetch(true, true);
		const client = new SRSSFUClient({});
		const removed = vi.fn();
		client.onRemoteAudioTrackRemoved(removed);
		const joinParams: JoinParams = {
			token: "tok",
			serverUrl: "http://h:1985/rtc/v1/whip/",
			identity: "alice",
			room: "room1",
			stream: "gs-alice",
			streamToken: "st",
		};
		await client.joinRoom(joinParams);
		(client as any).subscribePeers([{ identity: "bob", stream: "gs-bob" }]);
		await new Promise((r) => setTimeout(r, 50));
		(client as any).unsubscribePeer("bob");
		expect(removed).toHaveBeenCalledWith("bob");
		await client.leaveRoom();
	});

	it("routes WHIP/WHEP through same-origin reverse proxy (relative serverUrl)", async () => {
		const fetchMock = makeFetch(true, true);
		(globalThis as any).fetch = fetchMock;
		const client = new SRSSFUClient({});
		const onTrack = vi.fn();
		client.onRemoteAudioTrack(onTrack);
		const joinParams: JoinParams = {
			token: "tok",
			serverUrl: "/rtc/v1/whip/",
			identity: "alice",
			room: "room1",
			stream: "gs-alice",
			streamToken: "st",
		};
		await client.joinRoom(joinParams);
		(client as any).subscribePeers([{ identity: "bob", stream: "gs-bob" }]);
		await new Promise((r) => setTimeout(r, 50));

		const whipCall = fetchMock.mock.calls.find((c: unknown[]) =>
			String(c[0]).includes("/rtc/v1/whip/"),
		);
		const whepCall = fetchMock.mock.calls.find((c: unknown[]) =>
			String(c[0]).includes("/rtc/v1/whep/"),
		);
		expect(whipCall).toBeTruthy();
		expect(whepCall).toBeTruthy();
		expect(String(whipCall![0])).not.toMatch(/^https?:\/\//);
		expect(String(whepCall![0])).not.toMatch(/^https?:\/\//);
		expect(String(whepCall![0])).toContain("/rtc/v1/whep/");
		expect(onTrack).toHaveBeenCalledWith(
			expect.objectContaining({ identity: "bob" }),
		);
		await client.leaveRoom();
	});

	it("does not subscribe self stream", async () => {
		(globalThis as any).fetch = makeFetch(true, true);
		const client = new SRSSFUClient({});
		const onTrack = vi.fn();
		client.onRemoteAudioTrack(onTrack);
		const joinParams: JoinParams = {
			token: "tok",
			serverUrl: "http://h:1985/rtc/v1/whip/",
			identity: "alice",
			room: "room1",
			stream: "gs-alice",
			streamToken: "st",
		};
		await client.joinRoom(joinParams);
		(client as any).subscribePeers([
			{ identity: "alice", stream: "gs-alice" },
		]);
		await new Promise((r) => setTimeout(r, 50));
		expect(onTrack).not.toHaveBeenCalled();
		await client.leaveRoom();
	});
});
