import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { beforeEach, describe, expect, it, vi } from "vitest";

const emitted: Array<{ event: string; payload: unknown }> = [];

vi.mock("socket.io-client", () => {
	return {
		io: () => {
			const handlers = new Map<string, Function>();
			const socket = {
				on(event: string, cb: Function) {
					handlers.set(event, cb);
					if (event === "connect") queueMicrotask(() => cb());
					return socket;
				},
				emit(event: string, payload?: unknown) {
					emitted.push({ event, payload });
				},
				disconnect() {},
			};
			return socket;
		},
	};
});

import { BotRunner } from "./botRunner";

const mockLogger = {
	debug: vi.fn(),
	info: vi.fn(),
	warn: vi.fn(),
	error: vi.fn(),
};

describe("BotRunner", () => {
	let tmpDir: string;

	beforeEach(() => {
		tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "gospeak-bot-test-"));
		emitted.length = 0;
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
