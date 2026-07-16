import { describe, expect, it } from "vitest";
import { PluginBusHost } from "./pluginBus";

describe("PluginBusHost", () => {
	it("delivers publish to matching subscribers", async () => {
		const host = new PluginBusHost();
		const a = host.forPlugin("moderation");
		const b = host.forPlugin("welcome");
		const seen: string[] = [];

		b.subscribe("moderation:kicked", (msg) => {
			seen.push(`${msg.from}:${msg.payload as string}`);
		});

		const n = await a.publish("moderation:kicked", "alice");
		expect(n).toBe(1);
		expect(seen).toEqual(["moderation:alice"]);
	});

	it("supports wildcard topics", async () => {
		const host = new PluginBusHost();
		const a = host.forPlugin("a");
		const b = host.forPlugin("b");
		const topics: string[] = [];
		b.subscribe("moderation:*", (msg) => {
			topics.push(msg.topic);
		});
		await a.publish("moderation:kicked", 1);
		await a.publish("moderation:muted", 2);
		await a.publish("other:x", 3);
		expect(topics).toEqual(["moderation:kicked", "moderation:muted"]);
	});

	it("once auto-unsubscribes", async () => {
		const host = new PluginBusHost();
		const a = host.forPlugin("a");
		const b = host.forPlugin("b");
		let count = 0;
		b.once("t", () => {
			count++;
		});
		await a.publish("t");
		await a.publish("t");
		expect(count).toBe(1);
	});

	it("clearPlugin drops only that plugin's subs", async () => {
		const host = new PluginBusHost();
		const a = host.forPlugin("a");
		const b = host.forPlugin("b");
		const c = host.forPlugin("c");
		let bHits = 0;
		let cHits = 0;
		b.subscribe("t", () => {
			bHits++;
		});
		c.subscribe("t", () => {
			cHits++;
		});
		host.clearPlugin("b");
		await a.publish("t");
		expect(bHits).toBe(0);
		expect(cHits).toBe(1);
	});

	it("unsubscribe works", async () => {
		const host = new PluginBusHost();
		const a = host.forPlugin("a");
		const b = host.forPlugin("b");
		let hits = 0;
		const off = b.subscribe("t", () => {
			hits++;
		});
		await a.publish("t");
		off();
		await a.publish("t");
		expect(hits).toBe(1);
	});
});
