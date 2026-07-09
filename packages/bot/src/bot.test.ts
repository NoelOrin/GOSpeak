import { describe, it, expect } from "vitest";
import { EventType, EventBus } from "./core/index";
import { CommandFilter, PermissionFilter } from "./filters/index";
import { Plugin } from "./core/plugin";
import { RegisterPlugin } from "./decorators/register";
import { Command } from "./decorators/handlers";
import { getHandlersByEventType, listPlugins, clearRegistry } from "./core/registry";
import { loadPlugin, initPlugin } from "./core/loader";
import type { BotContext, MessageEvent } from "./core/index";

function makeCtx(): BotContext {
  return {
    logger: { debug() {}, info() {}, warn() {}, error() {} },
    config: {},
    pluginName: "test",
    chat: { send: async () => {}, reply: async (_e, _c) => {} },
    rooms: { listRooms: async () => [], getMembers: async () => [], createRoom: async () => ({ id: "r", name: "r" }), join: async () => {}, leave: () => {}, joined: () => [] },
    voice: { muteMember: async () => {}, removeMember: async () => {}, setMemberVolume: async () => {} },
    kv: { get: async () => undefined, set: async () => {}, delete: async () => {} },
    hasPermission: () => true,
  };
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

function boot(instance: Plugin): Plugin {
    console.log("boot ctx check", makeCtx().chat);
    instance.metadata;
  initPlugin({ metadata: instance.metadata, instance }, () => makeCtx());
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

describe("bot plugin runtime", () => {
  it("registers command handler and dispatches it", async () => {
    boot(new DemoPlugin());
    const bus = new EventBus({
      buildContext: () => makeCtx(),
      getPluginConfig: () => ({}),
    });
    const res = await bus.dispatch(msg("/ping hello world"));
    expect(res.executed).toBe(1);
  });

  it("skips handlers when filter does not match", async () => {
    boot(new DemoPlugin());
    const bus = new EventBus({
      buildContext: () => makeCtx(),
      getPluginConfig: () => ({}),
    });
    const res = await bus.dispatch(msg("no command here"));
    expect(res.executed).toBe(0);
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
    clearRegistry();
    const loaded = await loadPlugin(
      new URL("./plugins/example/echoPlugin.ts", import.meta.url).pathname,
      "example/echo",
    );
    expect(loaded.metadata.name).toBe("echo");
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
