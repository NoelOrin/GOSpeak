import { afterEach, describe, expect, it } from "vitest";
import type { BotContext, MessageEvent } from "./core/index";
import { EventBus, EventType } from "./core/index";
import { initPlugin, loadPlugin, registerPendingHandlers } from "./core/loader";
import { Plugin } from "./core/plugin";
import { clearRegistry, registerPlugin, removePlugin } from "./core/registry";
import { Command } from "./decorators/handlers";
import { RegisterPlugin } from "./decorators/register";
import { CommandFilter, PermissionFilter } from "./filters/index";

function makeCtx(): BotContext {
	const joinedRooms: string[] = [];
	return {
		logger: { debug() {}, info() {}, warn() {}, error() {} },
		config: {},
		pluginName: "test",
		chat: { send: async () => {}, reply: async (_e, _c) => {} },
		rooms: {
			listRooms: async () => [],
			getMembers: async () => [],
			createRoom: async () => ({ id: "r", name: "r" }),
			join: async () => {
				joinedRooms.push("dummy");
			},
			leave: () => {
				joinedRooms.pop();
			},
			joined: () => joinedRooms.slice(),
		},
		voice: {
			muteMember: async () => {},
			removeMember: async () => {},
			setMemberVolume: async () => {},
		},
		users: {
			getByIdentity: async (identity: string) => ({
				id: 1,
				name: identity,
				role: "user",
				uuid: "u1",
			}),
		},
		mutes: {
			list: async () => [],
			status: async () => null,
		},
		scheduler: {
			every: () => {},
			once: () => {},
			clear: () => {},
			clearAll: () => {},
		},
		kv: {
			get: async () => undefined,
			set: async () => {},
			delete: async () => {},
		},
		sharedKv: {
			get: async () => undefined,
			set: async () => {},
			delete: async () => {},
		},
		bus: {
			publish: async () => 0,
			subscribe: () => () => {},
			once: () => () => {},
		},
		hasPermission: () => true,
	};
}

function msg(
	content: string,
	role: "member" | "admin" = "member",
): MessageEvent {
	return {
		eventType: EventType.AdapterMessage,
		messageId: Math.random().toString(36).slice(2),
		room: { id: "room1", name: "Room" },
		sender: { identity: "u1", name: "U", role },
		content,
		isCommand: false,
		timestamp: Date.now(),
	};
}

function boot(Cls: new () => Plugin, modulePath = "test/demo"): Plugin {
	clearRegistry();
	(Cls as unknown as { __modulePath?: string }).__modulePath = modulePath;
	const meta = (Cls as unknown as { __pluginMeta?: any }).__pluginMeta;
	registerPlugin(modulePath, {
		name: meta?.name ?? "demo",
		author: meta?.author ?? "t",
		desc: meta?.desc ?? "d",
		version: meta?.version ?? "0.0.1",
		activated: true,
		handlerNames: [],
	});
	registerPendingHandlers(Cls, modulePath);
	const instance = new Cls();
	initPlugin(
		{
			metadata: instance.metadata,
			instance,
			modulePath,
			absPath: modulePath,
			importUrl: modulePath,
		},
		() => makeCtx(),
	);
	return instance;
}

@RegisterPlugin({ name: "demo", author: "t", desc: "d", version: "0.0.1" })
class DemoPlugin extends Plugin {
	last: string | undefined;

	@Command("ping")
	async ping(event: MessageEvent): Promise<void> {
		this.last = event.rawCommand?.args.join(" ");
		await this.ctx.chat.reply(event, "pong");
	}

	@Command("admin", { filters: [new PermissionFilter("admin")] })
	async adminOnly(): Promise<void> {
		this.last = "admin-ran";
	}
}

afterEach(() => {
	clearRegistry();
});

describe("bot plugin runtime", () => {
	it("registers command handler and dispatches it", async () => {
		boot(DemoPlugin);
		const bus = new EventBus({
			buildContext: () => makeCtx(),
			getPluginConfig: () => ({}),
		});
		const res = await bus.dispatch(msg("/ping hello world"));
		expect(res.executed).toBe(1);
	});

	it("skips handlers when filter does not match", async () => {
		boot(DemoPlugin);
		const bus = new EventBus({
			buildContext: () => makeCtx(),
			getPluginConfig: () => ({}),
		});
		const res = await bus.dispatch(msg("no command here"));
		expect(res.executed).toBe(0);
	});

	it("blocks non-admin from permission-gated commands", async () => {
		boot(DemoPlugin);
		const bus = new EventBus({
			buildContext: () => makeCtx(),
			getPluginConfig: () => ({}),
		});
		const res = await bus.dispatch(msg("/admin", "member"));
		expect(res.executed).toBe(0);
	});

	it("loads a plugin module by specifier", async () => {
		clearRegistry();
		const loaded = await loadPlugin(
			new URL("./plugins/example/echoPlugin.ts", import.meta.url).pathname,
			"example/echo",
		);
		expect(loaded.metadata.name).toBe("echo");
		removePlugin(loaded.modulePath);
	});

	it("CommandFilter parses command name, alias and args", () => {
		const f = new CommandFilter("echo", { alias: ["say"] });
		const event = msg("/say one two");
		const ok = f.filter(event, { config: {} });
		expect(ok).toBe(true);
		expect(event.rawCommand?.args).toEqual(["one", "two"]);
		expect(event.rawCommand?.alias).toBe("say");
	});
});
