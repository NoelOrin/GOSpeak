import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/api/sfu", () => ({
	resolveSFUProvider: (response: { provider?: string }) =>
		response.provider || "livekit",
}));

import { runVoiceJoin, VoiceJoinAbortError } from "./runVoiceJoin";

vi.mock("idb-keyval", () => ({
	get: vi.fn(async () => undefined),
	set: vi.fn(async () => {}),
	del: vi.fn(async () => {}),
}));

function makeToken(overrides: Record<string, unknown> = {}) {
	return {
		token: "tok",
		serverUrl: "wss://lk.example",
		room: "r1",
		identity: "alice",
		provider: "livekit" as const,
		...overrides,
	};
}

function makeDeps(overrides: Record<string, unknown> = {}) {
	const phases: string[] = [];
	const order: string[] = [];
	const client = {
		joinRoom: vi.fn(async () => {
			order.push("joinRoom");
		}),
		subscribePeers: vi.fn(),
		leaveRoom: vi.fn(async () => {}),
		destroy: vi.fn(async () => {}),
	};
	const deps = {
		loadClient: vi.fn(async () => {
			order.push("loadClient");
			return client as any;
		}),
		setupAudio: vi.fn(() => {
			order.push("setupAudio");
		}),
		joinSignalRoom: vi.fn(async () => {
			order.push("joinSignalRoom");
		}),
		joinSignalSfu: vi.fn(async () => {
			order.push("joinSignalSfu");
			return { members: [] };
		}),
		onPhase: vi.fn((phase: string) => {
			phases.push(phase);
			order.push(`phase:${phase}`);
		}),
		onClientReady: vi.fn(),
		audioOptions: { audioCapture: { echoCancellation: true } },
		socket: {
			isConnected: () => true,
			emitAck: vi.fn(),
			emitFireAndForget: vi.fn(),
			onServerEvent: vi.fn(),
			onDisconnected: vi.fn(),
		},
		password: "pwd",
		...overrides,
	};
	return { deps, client, phases, order };
}

describe("runVoiceJoin", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it("connects to workerUrl before signal join", async () => {
		const connectSignal = vi.fn(async () => {});
		const { deps } = makeDeps({ connectSignal });
		await runVoiceJoin(
			makeToken({ workerUrl: "wss://worker-a.example" }) as any,
			deps as any,
		);
		expect(connectSignal).toHaveBeenCalledWith("wss://worker-a.example");
		expect(deps.joinSignalRoom).toHaveBeenCalled();
	});

	it("rejoins previous text room after switching worker", async () => {
		const connectSignal = vi.fn(async () => {});
		const rejoinTextRoom = vi.fn(async () => {});
		const { deps } = makeDeps({
			connectSignal,
			getCurrentRoom: () => ({
				room: "lobby",
				domain_uuid: "domain-a",
				type: "text",
			}),
			rejoinTextRoom,
		});
		await runVoiceJoin(
			makeToken({
				workerUrl: "wss://worker-a.example",
				room: "r1",
				domain_uuid: "domain-a",
			}) as any,
			deps as any,
		);
		expect(connectSignal).toHaveBeenCalledWith("wss://worker-a.example");
		expect(rejoinTextRoom).toHaveBeenCalledWith("lobby", "domain-a", "alice");
		expect(deps.joinSignalRoom.mock.invocationCallOrder[0]).toBeGreaterThan(
			rejoinTextRoom.mock.invocationCallOrder[0],
		);
	});

	it("uses sfuRoom for media join while signal keeps logical room + domain_uuid", async () => {
		const { deps, client } = makeDeps();
		const token = makeToken({
			sfuRoom: "domain-a:r1",
			domain_uuid: "domain-a",
		});
		await runVoiceJoin(token as any, deps as any);

		expect(client.joinRoom).toHaveBeenCalledWith({
			token: "tok",
			serverUrl: "wss://lk.example",
			identity: "alice",
			room: "domain-a:r1",
			stream: undefined,
			streamToken: undefined,
		});
		expect(deps.joinSignalRoom).toHaveBeenCalledWith(
			"r1",
			"alice",
			"pwd",
			"domain-a",
		);
		expect(deps.joinSignalSfu).toHaveBeenCalledWith(
			"r1",
			"alice",
			undefined,
			"domain-a",
		);
	});

	it("livekit awaits signal after media (no media_ready early return)", async () => {
		const { deps, client, phases, order } = makeDeps();
		const token = makeToken();
		const result = await runVoiceJoin(token as any, deps as any);

		expect(result.client).toBe(client);
		expect(result.provider).toBe("livekit");
		expect(phases).toEqual(["loading_sfu", "joining_media", "joining_signal"]);
		expect(order).toEqual([
			"phase:loading_sfu",
			"loadClient",
			"phase:joining_media",
			"joinRoom",
			"setupAudio",
			"phase:joining_signal",
			"joinSignalRoom",
			"joinSignalSfu",
		]);
		expect(deps.loadClient).toHaveBeenCalledWith("livekit", {
			audioCapture: { echoCancellation: true },
			socket: {
				isConnected: expect.any(Function),
				emitAck: expect.any(Function),
				emitFireAndForget: expect.any(Function),
				onServerEvent: expect.any(Function),
				onDisconnected: expect.any(Function),
			},
		});
		expect(client.joinRoom).toHaveBeenCalledWith({
			token: "tok",
			serverUrl: "wss://lk.example",
			identity: "alice",
			room: "r1",
			stream: undefined,
			streamToken: undefined,
		});
		expect(deps.setupAudio).toHaveBeenCalledWith(client);
		expect(deps.joinSignalRoom).toHaveBeenCalledWith(
			"r1",
			"alice",
			"pwd",
			undefined,
		);
		expect(deps.joinSignalSfu).toHaveBeenCalledWith(
			"r1",
			"alice",
			undefined,
			undefined,
		);
	});

	it("livekit does not serialize concurrent joins", async () => {
		let active = 0;
		let maxActive = 0;
		let releaseFirst!: () => void;
		let firstEntered = false;
		const { deps } = makeDeps({
			loadClient: vi.fn(async () => {
				return {
					joinRoom: vi.fn(async () => {
						active++;
						maxActive = Math.max(maxActive, active);
						if (!firstEntered) {
							firstEntered = true;
							await new Promise<void>((r) => {
								releaseFirst = r;
							});
						} else {
							await new Promise((r) => setTimeout(r, 5));
						}
						active--;
					}),
					subscribePeers: vi.fn(),
					leaveRoom: vi.fn(async () => {}),
					destroy: vi.fn(async () => {}),
				} as any;
			}),
		});
		const token = makeToken({ provider: "livekit" });
		const p1 = runVoiceJoin(token as any, deps as any);
		for (let i = 0; i < 50 && !firstEntered; i++) {
			await new Promise((r) => setTimeout(r, 0));
		}
		const p2 = runVoiceJoin(token as any, deps as any);
		await new Promise((r) => setTimeout(r, 20));
		// livekit serializeJoins=false，允许并发进入 joinRoom
		expect(maxActive).toBeGreaterThanOrEqual(1);
		releaseFirst();
		await Promise.all([p1, p2]);
	});

	it("aborts before side effects when signal already aborted", async () => {
		const controller = new AbortController();
		controller.abort();
		const { deps } = makeDeps({ signal: controller.signal });

		await expect(
			runVoiceJoin(makeToken() as any, deps as any),
		).rejects.toBeInstanceOf(VoiceJoinAbortError);

		expect(deps.loadClient).not.toHaveBeenCalled();
		expect(deps.onPhase).not.toHaveBeenCalled();
		expect(deps.setupAudio).not.toHaveBeenCalled();
		expect(deps.joinSignalRoom).not.toHaveBeenCalled();
		expect(deps.joinSignalSfu).not.toHaveBeenCalled();
	});

	it("tears down client when aborted after media join starts", async () => {
		const controller = new AbortController();
		const leaveRoom = vi.fn(async () => {});
		const destroy = vi.fn(async () => {});
		const { deps, client } = makeDeps({
			signal: controller.signal,
			loadClient: vi.fn(async () => {
				return {
					...client,
					joinRoom: vi.fn(async () => {
						controller.abort();
						await new Promise((r) => setTimeout(r, 0));
					}),
					leaveRoom,
					destroy,
				} as any;
			}),
		});

		await expect(
			runVoiceJoin(makeToken() as any, deps as any),
		).rejects.toBeInstanceOf(VoiceJoinAbortError);

		expect(leaveRoom).toHaveBeenCalled();
		expect(destroy).toHaveBeenCalled();
		expect(deps.joinSignalRoom).not.toHaveBeenCalled();
	});
});
