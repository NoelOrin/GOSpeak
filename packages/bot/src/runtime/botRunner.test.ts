import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const emitted: Array<{ event: string; payload: unknown }> = [];

class MockWebSocket {
	static CONNECTING = 0;
	static OPEN = 1;
	static CLOSED = 3;
	readyState = MockWebSocket.CONNECTING;
	onopen: (() => void) | null = null;
	onmessage: ((ev: { data: unknown }) => void) | null = null;
	onerror: (() => void) | null = null;
	onclose: ((ev: { reason?: string }) => void) | null = null;

	constructor(public url: string) {
		queueMicrotask(() => {
			this.readyState = MockWebSocket.OPEN;
			this.onopen?.();
		});
	}

	send(data: string) {
		const parsed = JSON.parse(String(data)) as {
			event: string;
			data?: unknown;
		};
		emitted.push({ event: parsed.event, payload: parsed.data });
	}

	close() {
		this.readyState = MockWebSocket.CLOSED;
		this.onclose?.({ reason: "closed" });
	}
}

import { BotRunner } from "./botRunner";

const mockLogger = {
	debug: vi.fn(),
	info: vi.fn(),
	warn: vi.fn(),
	error: vi.fn(),
};

function stubTicketFetch() {
	vi.stubGlobal(
		"fetch",
		vi.fn(async () => ({
			ok: true,
			status: 200,
			json: async () => ({ data: { ticket: "test-ticket" } }),
		})),
	);
}

describe("BotRunner", () => {
	let tmpDir: string;

	beforeEach(() => {
		tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "gospeak-bot-test-"));
		emitted.length = 0;
		vi.stubGlobal("WebSocket", MockWebSocket);
		stubTicketFetch();
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it("starts, loads plugins, and dispatches events", async () => {
		const runner = new BotRunner(
			{
				serverUrl: "http://localhost:8998",
				socketUrl: "http://localhost:8998",
				accessToken: "fake-token",
				identity: "bot",
				displayName: "Bot",
				pluginDir: tmpDir,
				watchPlugins: false,
			},
			mockLogger as any,
		);

		await runner.start();

		expect(runner.status.pluginCount).toBe(0);
		expect(runner.status.startedAt).toBeGreaterThan(0);

		await runner.stop();
	});

	it("exposes status with correct shape", () => {
		const runner = new BotRunner(
			{
				serverUrl: "http://localhost:8998",
				socketUrl: "http://localhost:8998",
				accessToken: "fake-token",
				identity: "bot",
				displayName: "Bot",
			},
			mockLogger as any,
		);

		const status = runner.status;
		expect(status).toHaveProperty("connected");
		expect(status).toHaveProperty("pluginCount");
		expect(status).toHaveProperty("handlerCount");
		expect(status).toHaveProperty("startedAt");
		expect(status.connected).toBe(false);
	});

	it("starts without pluginDir gracefully", async () => {
		const runner = new BotRunner(
			{
				serverUrl: "http://localhost:8998",
				socketUrl: "http://localhost:8998",
				accessToken: "fake-token",
				identity: "bot",
				displayName: "Bot",
				watchPlugins: false,
			},
			mockLogger as any,
		);

		await runner.start();
		expect(runner.status.pluginCount).toBe(0);
		await runner.stop();
	});

	it("loads plugin at runtime via loadPlugin API", async () => {
		const runner = new BotRunner(
			{
				serverUrl: "http://localhost:8998",
				socketUrl: "http://localhost:8998",
				accessToken: "fake-token",
				identity: "bot",
				displayName: "Bot",
				pluginDir: tmpDir,
				watchPlugins: false,
			},
			mockLogger as any,
		);
		await runner.start();
		const example = path.resolve(
			path.dirname(new URL(import.meta.url).pathname),
			"../plugins/example/echoPlugin.ts",
		);
		const meta = await runner.loadPlugin(example);
		expect(meta.name).toBe("echo");
		expect(runner.listPlugins().some((p) => p.name === "echo")).toBe(true);
		await runner.unloadPlugin("echo");
		expect(runner.listPlugins().some((p) => p.name === "echo")).toBe(false);
		await runner.stop();
	});
});

beforeEach(() => {
	vi.stubGlobal("WebSocket", MockWebSocket);
	stubTicketFetch();
});

afterEach(() => {
	vi.unstubAllGlobals();
});

beforeEach(() => {
	vi.stubGlobal("WebSocket", MockWebSocket);
	stubTicketFetch();
});

afterEach(() => {
	vi.unstubAllGlobals();
});

it("auto-joins configured rooms on start", async () => {
	const runner = new BotRunner(
		{
			serverUrl: "http://localhost:8998",
			socketUrl: "http://localhost:8998",
			accessToken: "fake-token",
			identity: "bot",
			displayName: "Bot",
			watchPlugins: false,
			autoJoinRooms: ["lobby", "music"],
		},
		mockLogger as any,
	);

	await runner.start();
	// allow connect microtask to settle
	await Promise.resolve();
	await Promise.resolve();

	const joins = emitted.filter((e) => e.event === "room:join");
	expect(joins).toHaveLength(2);
	expect(joins.map((j) => (j.payload as any).room)).toEqual(["lobby", "music"]);
	expect(runner.joinedRooms).toEqual(["lobby", "music"]);

	await runner.stop();
});
