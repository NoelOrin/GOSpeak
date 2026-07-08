import { describe, it, expect, beforeEach } from "vitest";
import { EventType, EventBus } from "./core/index";
import { CommandFilter, PermissionFilter, RegexFilter } from "./filters/index";
import { Plugin } from "./core/plugin";
import { RegisterPlugin } from "./decorators/register";
import { On, Command } from "./decorators/handlers";
import { clearRegistry } from "./core/registry";
import { initPlugin } from "./core/loader";
import { loadPlugin } from "./core/loader";
import type { BotContext, MessageEvent } from "./core/index";

function makeCtx(): BotContext {
	const sent: string[] = [];
	return {
		logger: { debug() {}, info() {}, warn() {}, error() {} },
		config: {},
		pluginName: "test",
		chat: {
			send: async () => {},
			reply: async (_e, c) => {
				sent.push(c);
			},
		},
		rooms: {
			listRooms: async () => [],
			getMembers: async () => [],
			createRoom: async () => ({ id: "r", name: "r" }),
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

function boot(instance: DemoPlugin): DemoPlugin {
	const ctx = makeCtx();
	initPlugin({ metadata: instance.metadata, instance }, () => ctx);
	return instance;
}

function msg(content: string, role: "member" | "admin" = "member"): MessageEvent {
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

@RegisterPlugin({ name: "demo", author: "t", desc: "d", version: "0.0.1" })
class DemoPlugin extends Plugin {
	last: string | undefined;

	@Command("ping")
	async ping(event: MessageEvent): Promise<void> {
		this.last = event.rawCommand?.args.join(" ");
		await this.ctx.chat.reply(event, "pong");
	}

	@On(EventType.OnMessageReceived, { filters: [new RegexFilter(/bot/i)] })
	async greet(event: MessageEvent): Promise<void> {
		await this.ctx.chat.reply(event, "greeting");
	}

	@Command("admin", { filters: [new PermissionFilter("admin")] })
	async adminOnly(): Promise<void> {
		this.last = "admin-ran";
	}
}

describe("bot plugin runtime", () => {
	it("registers handlers and dispatches a command", async () => {
		boot(new DemoPlugin());
		process.stdout.write("H:" + JSON.stringify(getHandlersByEventType(EventType.AdapterMessage,false).map(h=>({f:h.fullName,m:h.modulePath}))) + "\n");
		process.stdout.write("P:" + JSON.stringify(listPlugins().map(p=>p.name)) + "\n");
		const bus = new EventBus({
			buildContext: () => makeCtx(),
			getPluginConfig: () => ({}),
		});
		const res = await bus.dispatch(msg("/ping hello world"));
		expect(res.executed).toBe(1);
	});

	it("skips handlers whose filters do not match", async () => {
		boot(new DemoPlugin());
		const bus = new EventBus({
			buildContext: () => makeCtx(),
			getPluginConfig: () => ({}),
		});
		const res = await bus.dispatch(msg("no command here"));
		expect(res.executed).toBe(0);
	});

	it("matches regex filter on On handlers", async () => {
		boot(new DemoPlugin());
		const bus = new EventBus({
			buildContext: () => makeCtx(),
			getPluginConfig: () => ({}),
		});
		const res = await bus.dispatch({
			eventType: EventType.OnMessageReceived,
			room: { id: "room1", name: "Room" },
			actor: { identity: "u1", name: "U", role: "member" },
			timestamp: Date.now(),
		} as any);
		expect(res.executed).toBe(1);
	});

	it("blocks non-admin from permission-gated commands", async () => {
		boot(new DemoPlugin());
		const bus = new EventBus({
			buildContext: () => makeCtx(),
			getPluginConfig: () => ({}),
		});
		const res = await bus.dispatch(msg("/admin", "member"));
		expect(res.executed).toBe(0);
	});

	it("loads a plugin module by specifier", async () => {
		const loaded = await loadPlugin(
			new URL("./plugins/example/echoPlugin.ts", import.meta.url).pathname,
			"example/echo",
		);
		expect(loaded.metadata.name).toBe("echo");
	});

	it("CommandFilter parses alias and args", () => {
		const f = new CommandFilter("echo", { alias: ["say"] });
		const event = msg("/say one two");
		const ok = f.filter(event, { config: {} });
		expect(ok).toBe(true);
		expect(event.rawCommand?.args).toEqual(["one", "two"]);
		expect(event.rawCommand?.alias).toBe("say");
	});
});
