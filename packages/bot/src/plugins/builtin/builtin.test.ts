import { describe, it, expect, beforeEach, vi } from "vitest";
import { EventType, EventBus } from "../../index";
import { clearRegistry, bindHandlerInstances } from "../../core/registry";
import type { MessageEvent, RoomEvent } from "../../core/types";

function makeCtx() {
	const sent: string[] = [];
	return {
		logger: { debug() {}, info() {}, warn() {}, error() {} },
		config: {},
		pluginName: "test",
		chat: {
			send: async (_r: string, c: string) => { sent.push(c); },
			reply: async (_e: any, c: string) => { sent.push(c); },
		},
		rooms: {
			listRooms: async () => [{ id: "r1", name: "Room1" }],
			getMembers: async () => [{ identity: "u1", name: "User1", role: "member" as const }],
			createRoom: async (name: string) => ({ id: name, name }),
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
		_sent: sent,
	};
}

function msg(content: string, role: "member" | "admin" = "member"): MessageEvent {
	return {
		eventType: EventType.AdapterMessage,
		messageId: "m1",
		room: { id: "r1", name: "Room1" },
		sender: { identity: "u1", name: "User1", role },
		content,
		isCommand: false,
		timestamp: Date.now(),
	};
}

function roomEvent(type: EventType, identity?: string): RoomEvent {
	return {
		eventType: type,
		room: { id: "r1", name: "Room1" },
		actor: identity ? { identity, name: identity, role: "member" } : undefined,
		timestamp: Date.now(),
	};
}

import { Plugin } from "../../core/plugin";

async function bootPlugin(moduleSpec: string, ctx: any): Promise<Plugin> {
	const mod: any = await import(moduleSpec);
	const PluginCls = Object.values(mod).find(
		(v: any) => typeof v === "function" && v.prototype instanceof Plugin,
	) as (new () => Plugin) | undefined;
	if (!PluginCls) throw new Error("no Plugin class found in " + moduleSpec);
	const instance = new PluginCls();
	instance.init(ctx);
	bindHandlerInstances(instance);
	return instance;
}

describe("builtin plugins", () => {

	it("room-manager lists rooms via command", async () => {
		const ctx: any = makeCtx();
		await bootPlugin("./room-manager", ctx);
		const bus = new EventBus({
			buildContext: () => ctx,
			getPluginConfig: () => ({}),
		});
		const res = await bus.dispatch(msg("/room list"));
		expect(res.executed).toBe(1);
		expect(ctx._sent.some((s: string) => s.includes("Room1"))).toBe(true);
	});

	it("keyword-reply matches and replies", async () => {
		const kvStore = new Map<string, any>([["keyword-reply:map", { hello: "你好！" }]]);
		const ctx: any = {
			...makeCtx(),
			kv: {
				get: async <T>(key: string): Promise<T | undefined> => kvStore.get(key) as T,
				set: async (key: string, val: any) => { kvStore.set(key, val); },
				delete: async (key: string) => { kvStore.delete(key); },
			},
		};
		await bootPlugin("./keyword-reply", ctx);
		const bus = new EventBus({
			buildContext: () => ctx,
			getPluginConfig: () => ({}),
		});
		const res = await bus.dispatch({
			eventType: EventType.OnMessageReceived,
			messageId: "m2",
			room: { id: "r1", name: "Room1" },
			sender: { identity: "u1", name: "User1", role: "member" },
			content: "hello everyone",
			isCommand: false,
			timestamp: Date.now(),
		} as any);
		expect(res.executed).toBe(1);
		expect(ctx._sent).toContain("你好！");
	});

	it("moderation blocks non-admin kick", async () => {
		const ctx: any = makeCtx();
		await bootPlugin("./moderation", ctx);
		const bus = new EventBus({
			buildContext: () => ctx,
			getPluginConfig: () => ({}),
		});
		const res = await bus.dispatch(msg("/kick someone", "member"));
		expect(res.executed).toBe(0);
	});

	it("moderation allows admin kick", async () => {
		const removeFn = vi.fn(async () => {});
		const ctx: any = {
			...makeCtx(),
			voice: {
				muteMember: async () => {},
				removeMember: removeFn,
				setMemberVolume: async () => {},
			},
		};
		await bootPlugin("./moderation", ctx);
		const bus = new EventBus({
			buildContext: () => ctx,
			getPluginConfig: () => ({}),
		});
		const res = await bus.dispatch(msg("/kick baduser", "admin"));
		expect(res.executed).toBe(1);
		expect(res.errors).toHaveLength(0);
		expect(ctx._sent.some((s: string) => s.includes("baduser"))).toBe(true);
	});

	it("welcome greets new members", async () => {
		const ctx: any = makeCtx();
		await bootPlugin("./welcome", ctx);
		const bus = new EventBus({
			buildContext: () => ctx,
			getPluginConfig: () => ({}),
		});
		const res = await bus.dispatch(roomEvent(EventType.OnRoomJoined, "NewUser"));
		expect(res.executed).toBeGreaterThanOrEqual(1);
		expect(ctx._sent.some((s: string) => s.includes("NewUser"))).toBe(true);
	});
});
