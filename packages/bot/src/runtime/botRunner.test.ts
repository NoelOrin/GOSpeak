import { describe, it, expect, beforeEach, vi } from "vitest";
import { BotRunner } from "./botRunner";
import { EventType, Plugin, RegisterPlugin, Command } from "../index";
import type { MessageEvent } from "../core/types";
import * as fs from "node:fs";
import * as path from "node:path";
import * as os from "node:os";

const mockLogger = {
	debug: vi.fn(),
	info: vi.fn(),
	warn: vi.fn(),
	error: vi.fn(),
};

@RegisterPlugin({ name: "runner-test", author: "t", desc: "d", version: "1.0.0" })
class TestPlugin extends Plugin {
	received: string[] = [];

	@Command("test")
	async onTest(event: MessageEvent): Promise<void> {
		this.received.push(event.rawCommand?.args.join(" ") ?? "");
		await this.ctx.chat.reply(event, "ok");
	}
}

describe("BotRunner", () => {
	let tmpDir: string;

	beforeEach(() => {
		tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "gospeak-bot-test-"));
	});

	it("starts, loads plugins, and dispatches events", async () => {
		const runner = new BotRunner({
			serverUrl: "http://localhost:8998",
			socketUrl: "http://localhost:8998",
			accessToken: "fake-token",
			identity: "bot",
			displayName: "Bot",
			pluginDir: tmpDir,
		}, mockLogger as any);

		await runner.start();

		expect(runner.status.pluginCount).toBe(0);
		expect(runner.status.startedAt).toBeGreaterThan(0);

		await runner.stop();
	});

	it("exposes status with correct shape", () => {
		const runner = new BotRunner({
			serverUrl: "http://localhost:8998",
			socketUrl: "http://localhost:8998",
			accessToken: "fake-token",
			identity: "bot",
			displayName: "Bot",
		}, mockLogger as any);

		const status = runner.status;
		expect(status).toHaveProperty("connected");
		expect(status).toHaveProperty("pluginCount");
		expect(status).toHaveProperty("handlerCount");
		expect(status).toHaveProperty("startedAt");
		expect(status.connected).toBe(false);
	});

	it("starts without pluginDir gracefully", async () => {
		const runner = new BotRunner({
			serverUrl: "http://localhost:8998",
			socketUrl: "http://localhost:8998",
			accessToken: "fake-token",
			identity: "bot",
			displayName: "Bot",
		}, mockLogger as any);

		await runner.start();
		expect(runner.status.pluginCount).toBe(0);
		await runner.stop();
	});
});
