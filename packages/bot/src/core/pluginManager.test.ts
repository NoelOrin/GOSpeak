import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import type { BotContext } from "./context";
import { EventBus, EventType } from "./index";
import { PluginManager } from "./pluginManager";
import { clearRegistry, getHandlersByEventType } from "./registry";

function makeCtx(): BotContext {
	return {
		logger: { debug() {}, info() {}, warn() {}, error() {} },
		config: {},
		pluginName: "test",
		chat: { send: async () => {}, reply: async () => {} },
		rooms: {
			listRooms: async () => [],
			getMembers: async () => [],
			createRoom: async () => ({ id: "r", name: "r" }),
			join: async () => {},
			leave: () => {},
			joined: () => [],
		},
		voice: {
			muteMember: async () => {},
			removeMember: async () => {},
			setMemberVolume: async () => {},
		},
		kv: {
			get: async () => undefined,
			set: async () => {},
			delete: async () => {},
		},
		hasPermission: () => true,
	};
}

function examplePluginPath(): string {
	return path.resolve(
		path.dirname(new URL(import.meta.url).pathname),
		"../plugins/example/echoPlugin.ts",
	);
}

/** 在包内写插件，保证 vitest/tsx 能处理装饰器 */
function writeInPackagePlugin(dir: string, name: string): string {
	fs.mkdirSync(dir, { recursive: true });
	const relToCore = path
		.relative(
			dir,
			path.resolve(path.dirname(new URL(import.meta.url).pathname)),
		)
		.split(path.sep)
		.join("/");
	const body = `import { Plugin } from "${relToCore}/plugin";
import type { MessageEvent } from "${relToCore}/types";
import { Command } from "${relToCore}/../decorators/handlers";
import { RegisterPlugin } from "${relToCore}/../decorators/register";

@RegisterPlugin({ name: "${name}", author: "t", desc: "runtime", version: "0.0.1" })
export class RuntimePlugin extends Plugin {
  @Command("${name}")
  async onCmd(event: MessageEvent): Promise<void> {
    await this.ctx.chat.reply(event, "ok");
  }
}
`;
	const abs = path.join(dir, `${name}.ts`);
	fs.writeFileSync(abs, body, "utf8");
	return abs;
}

describe("PluginManager runtime load", () => {
	let tmp: string;
	let manager: PluginManager | null = null;
	const packageTmpRoot = path.resolve(
		path.dirname(new URL(import.meta.url).pathname),
		"../plugins/.runtime-test",
	);

	afterEach(async () => {
		if (manager) {
			await manager.stop();
			manager = null;
		}
		clearRegistry();
		if (tmp && fs.existsSync(tmp)) {
			fs.rmSync(tmp, { recursive: true, force: true });
		}
		if (fs.existsSync(packageTmpRoot)) {
			fs.rmSync(packageTmpRoot, { recursive: true, force: true });
		}
	});

	it("loads a plugin from pluginDir at runtime", async () => {
		tmp = fs.mkdtempSync(path.join(os.tmpdir(), "gospeak-pm-"));
		manager = new PluginManager({
			pluginDir: tmp,
			watch: false,
			buildContext: () => makeCtx(),
		});
		await manager.start();
		expect(manager.pluginCount).toBe(0);

		const managed = await manager.loadFromPath(examplePluginPath(), true);
		expect(managed.name).toBe("echo");
		expect(manager.pluginCount).toBe(1);

		const handlers = getHandlersByEventType(EventType.AdapterMessage, false);
		expect(handlers.length).toBeGreaterThan(0);

		const bus = new EventBus({
			buildContext: () => makeCtx(),
			getPluginConfig: () => ({}),
		});
		const res = await bus.dispatch({
			eventType: EventType.AdapterMessage,
			messageId: "1",
			room: { id: "r", name: "r" },
			sender: { identity: "u", name: "U", role: "member" },
			content: "/echo hi",
			isCommand: false,
			timestamp: Date.now(),
		});
		expect(res.executed).toBe(1);

		await manager.unload("echo");
		expect(manager.pluginCount).toBe(0);
		expect(getHandlersByEventType(EventType.AdapterMessage, false).length).toBe(
			0,
		);
	});

	it("reloads plugin with cache bust", async () => {
		tmp = fs.mkdtempSync(path.join(os.tmpdir(), "gospeak-pm-"));
		manager = new PluginManager({
			pluginDir: tmp,
			watch: false,
			buildContext: () => makeCtx(),
		});
		await manager.start();
		await manager.loadFromPath(examplePluginPath(), true);
		await manager.reload("echo");
		expect(manager.pluginCount).toBe(1);
		expect(manager.get("echo")?.metadata.name).toBe("echo");
	});

	it("installs plugin into pluginDir", async () => {
		tmp = path.join(packageTmpRoot, "install-target");
		const srcDir = path.join(packageTmpRoot, "install-src");
		const src = writeInPackagePlugin(srcDir, "installed");
		manager = new PluginManager({
			pluginDir: tmp,
			watch: false,
			buildContext: () => makeCtx(),
		});
		await manager.start();
		const installed = await manager.installFromPath(src);
		expect(installed.name).toBe("installed");
		expect(fs.existsSync(path.join(tmp, "installed.ts"))).toBe(true);
		expect(manager.pluginCount).toBe(1);
	});

	it("discovers plugins under pluginDir on start", async () => {
		tmp = path.join(packageTmpRoot, "discover");
		writeInPackagePlugin(tmp, "autodiscover");
		manager = new PluginManager({
			pluginDir: tmp,
			watch: false,
			buildContext: () => makeCtx(),
		});
		await manager.start();
		expect(manager.pluginCount).toBe(1);
		expect(manager.get("autodiscover")).toBeTruthy();
	});
});
