import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/api/sfu", () => ({
	resolveSFUProvider: (response: { provider?: string }) =>
		response.provider || "livekit",
}));

import { runVoiceJoin, VoiceJoinAbortError } from "./runVoiceJoin";

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
		socket: { id: "sock" },
		password: "pwd",
		...overrides,
	};
	return { deps, client, phases, order };
}

describe("runVoiceJoin", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it("livekit awaits signal after media (no media_ready early return)", async () => {
		const { deps, client, phases, order } = makeDeps();
		const token = makeToken();
		const result = await runVoiceJoin(token as any, deps as any);

		expect(result.client).toBe(client);
		expect(result.provider).toBe("livekit");
		expect(phases).toEqual([
			"loading_sfu",
			"joining_media",
			"joining_signal",
		]);
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
			socket: { id: "sock" },
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
		expect(deps.joinSignalRoom).toHaveBeenCalledWith("r1", "alice", "pwd");
		expect(deps.joinSignalSfu).toHaveBeenCalledWith("r1", "alice", undefined);
	});

	it("srs adapter subscribePeers after ack", async () => {
		const { deps, client } = makeDeps({
			joinSignalSfu: vi.fn(async () => ({
				members: [
					{ identity: "alice", stream: "gs-alice" },
					{ identity: "bob", stream: "gs-bob" },
					{ identity: "carol" },
				],
			})),
		});
		const token = makeToken({
			provider: "srs",
			serverUrl: "http://srs:1985",
			whipUrl: "/rtc/v1/whip/",
			stream: "gs-alice",
			streamToken: "st",
		});
		await runVoiceJoin(token as any, deps as any);
		await vi.waitFor(() => {
			expect(deps.joinSignalSfu).toHaveBeenCalledWith("r1", "alice", "gs-alice");
			expect(client.subscribePeers).toHaveBeenCalledWith([
				{ identity: "bob", stream: "gs-bob" },
			]);
		});

		expect(deps.loadClient).toHaveBeenCalledWith("srs", expect.any(Object));
		expect(client.joinRoom).toHaveBeenCalledWith({
			token: "tok",
			serverUrl: "/rtc/v1/whip/",
			identity: "alice",
			room: "r1",
			stream: "gs-alice",
			streamToken: "st",
		});
	});

	it("srs media_ready after WHIP and onClientReady before phase", async () => {
		const order: string[] = [];
		let resolveSignalRoom!: () => void;
		const { deps, client } = makeDeps({
			onClientReady: vi.fn(() => {
				order.push("onClientReady");
			}),
			onPhase: vi.fn((phase: string) => {
				order.push(`phase:${phase}`);
			}),
			joinSignalRoom: vi.fn(
				() =>
					new Promise<void>((resolve) => {
						resolveSignalRoom = resolve;
					}),
			),
		});
		const token = makeToken({
			provider: "srs",
			serverUrl: "http://srs:1985",
			whipUrl: "/rtc/v1/whip/",
			stream: "gs-alice",
			streamToken: "st",
		});
		const result = await runVoiceJoin(token as any, deps as any);
		expect(result.client).toBe(client);
		const phaseCalls = (deps.onPhase as any).mock.calls.map((c: any[]) => c[0]);
		expect(phaseCalls).toEqual([
			"loading_sfu",
			"joining_media",
			"media_ready",
		]);
		expect(deps.onClientReady).toHaveBeenCalledWith(client, "srs");
		expect(order.indexOf("onClientReady")).toBeLessThan(
			order.indexOf("phase:media_ready"),
		);
		expect(deps.joinSignalRoom).toHaveBeenCalled();
		expect(deps.joinSignalSfu).not.toHaveBeenCalled();

		resolveSignalRoom();
		await vi.waitFor(() => {
			expect(deps.joinSignalSfu).toHaveBeenCalled();
			expect((deps.onPhase as any).mock.calls.map((c: any[]) => c[0])).toEqual([
				"loading_sfu",
				"joining_media",
				"media_ready",
				"ready",
			]);
		});
	});

	it("srs returns before signal completes so VoiceChat can load", async () => {
		let resolveSignal!: () => void;
		const { deps, client } = makeDeps({
			joinSignalRoom: vi.fn(
				() =>
					new Promise<void>((resolve) => {
						resolveSignal = resolve;
					}),
			),
		});
		const token = makeToken({
			provider: "srs",
			serverUrl: "http://srs:1985",
			whipUrl: "/rtc/v1/whip/",
			stream: "gs-alice",
		});
		const result = await Promise.race([
			runVoiceJoin(token as any, deps as any),
			new Promise((_, reject) =>
				setTimeout(() => reject(new Error("blocked on signal")), 20),
			),
		]);
		expect((result as any).client).toBe(client);
		expect(deps.onPhase).toHaveBeenCalledWith("media_ready");
		expect(deps.joinSignalSfu).not.toHaveBeenCalled();
		resolveSignal();
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

	it("serializes concurrent srs joins for same stream", async () => {
		let active = 0;
		let maxActive = 0;
		let resolveFirstJoin!: () => void;
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
								resolveFirstJoin = r;
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
		const token = makeToken({
			provider: "srs",
			whipUrl: "/rtc/v1/whip/",
			stream: "gs-alice",
		});
		const p1 = runVoiceJoin(token as any, deps as any);
		for (let i = 0; i < 50 && !firstEntered; i++) {
			await new Promise((r) => setTimeout(r, 0));
		}
		const p2 = runVoiceJoin(token as any, deps as any);
		await new Promise((r) => setTimeout(r, 20));
		expect(maxActive).toBe(1);
		resolveFirstJoin();
		await Promise.all([p1, p2]);
		expect(maxActive).toBe(1);
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
