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

	it("runs ordered phases and setupAudio before signal join", async () => {
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

		expect(deps.loadClient).toHaveBeenCalledWith("srs", expect.any(Object));
		expect(client.joinRoom).toHaveBeenCalledWith({
			token: "tok",
			serverUrl: "/rtc/v1/whip/",
			identity: "alice",
			room: "r1",
			stream: "gs-alice",
			streamToken: "st",
		});
		expect(deps.joinSignalSfu).toHaveBeenCalledWith("r1", "alice", "gs-alice");
		expect(client.subscribePeers).toHaveBeenCalledWith([
			{ identity: "bob", stream: "gs-bob" },
		]);
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
});
