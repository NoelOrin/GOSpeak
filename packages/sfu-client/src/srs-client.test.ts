import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { SRSSFUClient } from "./srs-client";
import { isWhipBusyError } from "./srs-stream-gate";
import type { JoinParams, SignalSocket } from "./types";

describe("isWhipBusyError", () => {
	it("treats 5020/stream busy as busy", () => {
		expect(isWhipBusyError(new Error("SRS WHIP request failed: 5020"))).toBe(true);
		expect(isWhipBusyError(new Error("stream busy"))).toBe(true);
	});

	it("never treats 403 publish denied as busy", () => {
		expect(isWhipBusyError(new Error("SRS WHIP request failed: 403"))).toBe(false);
		expect(isWhipBusyError(new Error("publish denied"))).toBe(false);
	});

	it("treats codes containing 403 (e.g. 4032) as busy, not publish denied", () => {
		expect(
			isWhipBusyError(new Error("SRS WHIP request failed: 4032")),
		).toBe(true);
	});
});

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
				getUserMedia: vi.fn().mockImplementation(async () => {
					const track = { kind: "audio", enabled: true, stop: vi.fn() };
					return {
						getAudioTracks: () => [track],
						getTracks: () => [track],
					};
				}),
			},
		},
		writable: true,
		configurable: true,
	});
	class MockAudioContext {
		createMediaStreamSource = vi.fn(() => ({ connect: vi.fn() }));
		createAnalyser = vi.fn(() => ({
			fftSize: 0,
			frequencyBinCount: 256,
			getByteFrequencyData: () => {},
		}));
		close() {
			return Promise.resolve();
		}
	}
	(globalThis as any).AudioContext = MockAudioContext;
	(globalThis as any).requestAnimationFrame = (cb: FrameRequestCallback) =>
		setTimeout(() => cb(performance.now()), 16);
	(globalThis as any).cancelAnimationFrame = (id: number) => clearTimeout(id);
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
			text: async (): Promise<string> => "v=0\r\n",
		} as unknown as Response;
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

	it("accepts domain-scoped member event for composite sfuRoom", async () => {
		(globalThis as any).fetch = makeFetch(true, true);
		const handlers: Record<string, Function[]> = {};
		const socket: SignalSocket = {
			isConnected: () => true,
			emitAck: vi.fn(),
			emitFireAndForget: vi.fn(),
			onServerEvent: (ev: string, cb: Function) => {
				(handlers[ev] ||= []).push(cb);
				return () => {};
			},
			onDisconnected: vi.fn(),
		};
		const client = new SRSSFUClient({ socket });
		const onTrack = vi.fn();
		client.onRemoteAudioTrack(onTrack);
		await client.joinRoom({
			token: "tok",
			serverUrl: "/rtc/v1/whip/",
			identity: "alice",
			room: "domain-a:room-a",
			stream: "gs-alice",
			streamToken: "st",
		});

		handlers["member:joined"][0]({
			room: "room-a",
			domain_uuid: "domain-a",
			identity: "bob",
			stream: "gs-bob",
		});
		await new Promise((r) => setTimeout(r, 50));
		expect(onTrack).toHaveBeenCalledWith(
			expect.objectContaining({ identity: "bob" }),
		);
		await client.leaveRoom();
	});

	it("retries WHEP on failure without dropping membership", async () => {
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
			await vi.advanceTimersByTimeAsync(20_000);
			expect(removed).not.toHaveBeenCalled();
			expect((client as any).peerSubs.has("bob")).toBe(true);
			await client.leaveRoom();
		} finally {
			vi.useRealTimers();
		}
	});

describe("SRSSFUClient subscribe retry exhaustion", () => {
	it("exhausts after MAX_SUBSCRIBE_RETRIES and emits onRemoteAudioTrackRemoved", async () => {
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
			(client as any).peerSubs.set("bob", {
				identity: "bob",
				stream: "gs-bob",
				pc: null,
				resourceUrl: "",
				retryCount: SRSSFUClient.MAX_SUBSCRIBE_RETRIES - 1,
				retryTimer: null,
				connecting: false,
			});
			(client as any).subscribePeer("bob", "gs-bob");
			await vi.advanceTimersByTimeAsync(10_000);
			expect(removed).toHaveBeenCalledWith("bob");
			expect((client as any).peerSubs.has("bob")).toBe(false);
			await client.leaveRoom();
		} finally {
			vi.useRealTimers();
		}
	});
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

describe("SRSSFUClient room isolation", () => {
	it("filters member events by room", async () => {
		(globalThis as any).fetch = makeFetch(true, true);
		const handlers: Record<string, Function[]> = {};
		const socket: SignalSocket = {
			isConnected: () => true,
			emitAck: vi.fn(),
			emitFireAndForget: vi.fn(),
			onServerEvent: (ev: string, cb: Function) => {
				(handlers[ev] ||= []).push(cb);
				return () => {};
			},
			onDisconnected: vi.fn(),
		};
		const client = new SRSSFUClient({ socket });
		const onTrack = vi.fn();
		client.onRemoteAudioTrack(onTrack);
		await client.joinRoom({
			token: "tok",
			serverUrl: "/rtc/v1/whip/",
			identity: "alice",
			room: "room-a",
			stream: "gs-alice",
			streamToken: "st",
		});

		// other room should be ignored
		handlers["member:joined"][0]({
			room: "room-b",
			identity: "bob",
			stream: "gs-bob",
		});
		await new Promise((r) => setTimeout(r, 20));
		expect(onTrack).not.toHaveBeenCalled();

		// same room should subscribe
		handlers["member:joined"][0]({
			room: "room-a",
			identity: "bob",
			stream: "gs-bob",
		});
		await new Promise((r) => setTimeout(r, 50));
		expect(onTrack).toHaveBeenCalledWith(
			expect.objectContaining({ identity: "bob" }),
		);
		await client.leaveRoom();
	});
});


describe("SRSSFUClient setMicEnabled", () => {
	it("setMicEnabled(false) stops local tracks and keeps WHIP", async () => {
		const fetchMock = makeFetch(true, true);
		(globalThis as any).fetch = fetchMock;
		const client = new SRSSFUClient({});
		await client.joinRoom({
			token: "tok",
			serverUrl: "/rtc/v1/whip/",
			identity: "alice",
			room: "room1",
			stream: "gs-alice",
			streamToken: "st",
		});
		const local = (client as any).localStream as {
			getAudioTracks: () => Array<{ stop: ReturnType<typeof vi.fn>; enabled: boolean; readyState: string }>;
		};
		const tracks = local.getAudioTracks();
		for (const t of tracks) t.readyState = "live";
		await client.setMicEnabled(false);
		const del = fetchMock.mock.calls.find(
			(c: unknown[]) => c[1] && (c[1] as any).method === "DELETE",
		);
		expect(del).toBeFalsy();
		expect((client as any).publishPc).toBeTruthy();
		for (const t of tracks) {
			expect(t.enabled).toBe(false);
			expect(t.stop).toHaveBeenCalled();
		}
		expect(client.isConnected()).toBe(true);
		await client.leaveRoom();
	});

	it("setMicEnabled(true) replaces stopped tracks without new WHIP", async () => {
		const fetchMock = makeFetch(true, true);
		(globalThis as any).fetch = fetchMock;
		const replaceTrack = vi.fn().mockResolvedValue(undefined);
		(globalThis as any).RTCPeerConnection = vi.fn(function () {
			const pc = makeMockPc();
			(pc as any).getSenders = () => [{ track: { kind: "audio" }, replaceTrack }];
			return pc;
		});
		const client = new SRSSFUClient({});
		await client.joinRoom({
			token: "tok",
			serverUrl: "/rtc/v1/whip/",
			identity: "alice",
			room: "room1",
			stream: "gs-alice",
			streamToken: "st",
		});
		const whipBefore = fetchMock.mock.calls.filter((c: unknown[]) =>
			String(c[0]).includes("/rtc/v1/whip/") &&
			(!(c[1] as any)?.method || (c[1] as any).method === "POST"),
		).length;
		const tracks = (client as any).localStream.getAudioTracks();
		for (const t of tracks) t.readyState = "ended";
		await client.setMicEnabled(false);
		await client.setMicEnabled(true);
		const whipAfter = fetchMock.mock.calls.filter((c: unknown[]) =>
			String(c[0]).includes("/rtc/v1/whip/") &&
			(!(c[1] as any)?.method || (c[1] as any).method === "POST"),
		).length;
		expect(whipAfter).toBe(whipBefore);
		expect(replaceTrack).toHaveBeenCalled();
		expect((client as any).micEnabled).toBe(true);
		await client.leaveRoom();
	});
});

describe("SRSSFUClient setMicEnabled with PC disconnected", () => {
	it("setMicEnabled(true) rebuilds WHIP when PC is failed/closed", async () => {
		const fetchMock = makeFetch(true, true);
		(globalThis as any).fetch = fetchMock;
		const client = new SRSSFUClient({});
		await client.joinRoom({
			token: "tok",
			serverUrl: "/rtc/v1/whip/",
			identity: "alice",
			room: "room1",
			stream: "gs-alice",
			streamToken: "st",
		});
		const whipBefore = fetchMock.mock.calls.filter((c: unknown[]) =>
			String(c[0]).includes("/rtc/v1/whip/") &&
			(!(c[1] as any)?.method || (c[1] as any).method === "POST"),
		).length;
		// Simulate PC failure via connectionState
		const pc = (client as any).publishPc;
		Object.defineProperty(pc, "connectionState", { value: "failed" });
		const tracks = (client as any).localStream.getAudioTracks();
		for (const t of tracks) t.readyState = "ended";
		await client.setMicEnabled(false);
		await client.setMicEnabled(true);
		const whipAfter = fetchMock.mock.calls.filter((c: unknown[]) =>
			String(c[0]).includes("/rtc/v1/whip/") &&
			(!(c[1] as any)?.method || (c[1] as any).method === "POST"),
		).length;
		expect(whipAfter).toBeGreaterThan(whipBefore);
		expect((client as any).publishPc).toBeTruthy();
		expect((client as any).micEnabled).toBe(true);
		await client.leaveRoom();
	});
});



describe("SRSSFUClient leave during in-flight WHIP", () => {
	it("leaveRoom aborts in-flight WHIP before stream sticks", async () => {
		const fetchMock = vi.fn((_url: string, init?: RequestInit) => {
			if (init?.method === "DELETE") {
				return Promise.resolve({
					ok: true,
					status: 200,
					headers: { get: () => "" } as any,
					text: async (): Promise<string> => "",
				});
			}
			return new Promise((_, reject) => {
				const signal = init?.signal;
				if (!signal) return;
				if (signal.aborted) {
					reject(new DOMException("Aborted", "AbortError"));
					return;
				}
				signal.addEventListener(
					"abort",
					() => reject(new DOMException("Aborted", "AbortError")),
					{ once: true },
				);
			});
		});
		(globalThis as any).fetch = fetchMock;

		const client = new SRSSFUClient({});
		const joinPromise = client.joinRoom({
			token: "tok",
			serverUrl: "/rtc/v1/whip/",
			identity: "alice",
			room: "room1",
			stream: "gs-alice",
			streamToken: "st",
		});

		for (let i = 0; i < 50 && fetchMock.mock.calls.length === 0; i++) {
			await new Promise((r) => setTimeout(r, 0));
		}
		expect(fetchMock.mock.calls.length).toBeGreaterThan(0);

		await client.leaveRoom();
		expect(client.isConnected()).toBe(false);
		await expect(joinPromise).rejects.toThrow();
		expect(client.isConnected()).toBe(false);
		expect((client as any).publishPc).toBeNull();
		expect((client as any).localStream).toBeNull();
	});
});


describe("SRSSFUClient stream queue", () => {
	it("serializes same-stream join after leave so second waits for first teardown", async () => {
		const events: string[] = [];
		let active = 0;
		let maxActive = 0;
		const fetchMock = vi.fn(async (url: string, init?: RequestInit): Promise<Response> => {
			if (init?.method === "DELETE") {
				events.push("DELETE");
				return {
					ok: true,
					status: 200,
					headers: { get: () => "" } as any,
					text: async (): Promise<string> => "",
				} as unknown as Response;
			}
			active++;
			maxActive = Math.max(maxActive, active);
			events.push("WHIP_START");
			await new Promise((r) => setTimeout(r, 30));
			active--;
			events.push("WHIP_END");
			return {
				ok: true,
				status: 201,
				headers: {
					get: (k: string) =>
						k === "Location" ? "/rtc/v1/whip/?action=delete&token=t" : "",
				},
				text: async (): Promise<string> => "v=0\r\n",
			} as unknown as Response;
		});
		(globalThis as any).fetch = fetchMock;

		const a = new SRSSFUClient({});
		const b = new SRSSFUClient({});
		const params = {
			token: "tok",
			serverUrl: "/rtc/v1/whip/",
			identity: "alice",
			room: "room1",
			stream: "gs-alice",
			streamToken: "st",
		};

		const p1 = a.joinRoom(params);
		// 模拟 orchestrator abort 后立刻再 join：leave 与新 join 必须串行
		const pLeave = a.leaveRoom();
		const p2 = b.joinRoom(params);
		await Promise.all([p1.catch(() => undefined), pLeave, p2]);

		expect(maxActive).toBe(1);
		const whipStarts = events.filter((e) => e === "WHIP_START").length;
		expect(whipStarts).toBeGreaterThanOrEqual(1);
		// 第二次 join 前必须先 DELETE/拆掉
		const firstWhipEnd = events.indexOf("WHIP_END");
		const lastWhipStart = events.lastIndexOf("WHIP_START");
		if (whipStarts >= 2) {
			expect(events.slice(0, lastWhipStart).includes("DELETE")).toBe(true);
			expect(firstWhipEnd).toBeLessThan(lastWhipStart);
		}
		await b.leaveRoom();
	});
});

describe("SRSSFUClient stale stream gate recovery", () => {
	it("recovers from leaked stream holder without permanent hang", async () => {
		const fetchMock = makeFetch(true, true);
		(globalThis as any).fetch = fetchMock;
		const a = new SRSSFUClient({});
		const b = new SRSSFUClient({});
		const params = {
			token: "tok",
			serverUrl: "/rtc/v1/whip/",
			identity: "alice",
			room: "room1",
			stream: "gs-alice",
			streamToken: "st",
		};
		await a.joinRoom(params);
		// 模拟异常路径：holder 未释放但 client 已 leaving。
		(a as any).leaving = true;
		(a as any).hasJoined = false;
		(a as any).publishPc = null;
		(a as any).publishResourceUrl = "";
		const started = Date.now();
		await expect(b.joinRoom(params)).resolves.toBeUndefined();
		expect(Date.now() - started).toBeLessThan(2_000);
		expect(b.isConnected()).toBe(true);
		await b.leaveRoom();
	});
});

describe("SRSSFUClient setMicEnabled no rebuild on transient PC state", () => {
	it("does not rebuild WHIP when PC is only disconnected", async () => {
		const fetchMock = makeFetch(true, true);
		(globalThis as any).fetch = fetchMock;
		const replaceTrack = vi.fn().mockResolvedValue(undefined);
		(globalThis as any).RTCPeerConnection = vi.fn(function () {
			const pc = makeMockPc();
			(pc as any).getSenders = () => [{ track: { kind: "audio" }, replaceTrack }];
			(pc as any).connectionState = "connected";
			return pc;
		});
		const client = new SRSSFUClient({});
		await client.joinRoom({
			token: "tok",
			serverUrl: "/rtc/v1/whip/",
			identity: "alice",
			room: "room1",
			stream: "gs-alice",
			streamToken: "st",
		});
		const whipBefore = fetchMock.mock.calls.filter((c: unknown[]) =>
			String(c[0]).includes("/rtc/v1/whip/") &&
			(!(c[1] as any)?.method || (c[1] as any).method === "POST"),
		).length;
		const tracks = (client as any).localStream.getAudioTracks();
		for (const t of tracks) t.readyState = "ended";
		await client.setMicEnabled(false);
		Object.defineProperty((client as any).publishPc, "connectionState", {
			value: "disconnected",
			configurable: true,
		});
		await client.setMicEnabled(true);
		const whipAfter = fetchMock.mock.calls.filter((c: unknown[]) =>
			String(c[0]).includes("/rtc/v1/whip/") &&
			(!(c[1] as any)?.method || (c[1] as any).method === "POST"),
		).length;
		expect(whipAfter).toBe(whipBefore);
		expect(replaceTrack).toHaveBeenCalled();
		await client.leaveRoom();
	});
});

describe("SRSSFUClient live holder takeover", () => {
	it("waits for live holder leave without stealing", async () => {
		const events: string[] = [];
		const fetchMock = vi.fn(async (url: string, init?: RequestInit): Promise<Response> => {
			if (init?.method === "DELETE") {
				events.push("DELETE");
				return {
					ok: true,
					status: 200,
					headers: { get: () => "" } as any,
					text: async (): Promise<string> => "",
				} as unknown as Response;
			}
			events.push("WHIP");
			return {
				ok: true,
				status: 201,
				headers: {
					get: (k: string) =>
						k === "Location" ? "/rtc/v1/whip/?action=delete&token=t" : "",
				},
				text: async (): Promise<string> => "v=0\r\n",
			} as unknown as Response;
		});
		(globalThis as any).fetch = fetchMock;
		const a = new SRSSFUClient({});
		const b = new SRSSFUClient({});
		const params = {
			token: "tok",
			serverUrl: "/rtc/v1/whip/",
			identity: "alice",
			room: "room1",
			stream: "gs-alice",
			streamToken: "st",
		};
		await a.joinRoom(params);
		const p = b.joinRoom(params);
		// b 应请求 a leave，而不是立刻第二路 WHIP 并行
		await vi.waitFor(() => {
			expect(events.filter((e) => e === "DELETE").length).toBeGreaterThan(0);
		});
		await p;
		const whips = events.filter((e) => e === "WHIP").length;
		expect(whips).toBeGreaterThanOrEqual(2);
		// 第二次 WHIP 必须在 DELETE 之后
		expect(events.indexOf("DELETE")).toBeLessThan(events.lastIndexOf("WHIP"));
		await b.leaveRoom();
	});
});
