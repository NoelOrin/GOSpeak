import { describe, it } from "vitest";
import { EventType } from "./core/types";
import { getHandlersByEventType, listPlugins } from "./core/registry";
import { Plugin } from "./core/plugin";
import { RegisterPlugin } from "./decorators/register";
import { Command } from "./decorators/handlers";
import type { MessageEvent } from "./core/types";

@RegisterPlugin({ name: "probe", author: "x", desc: "x", version: "1.0.0" })
class ProbePlugin extends Plugin {
	@Command("ping")
	async ping(_e: MessageEvent): Promise<void> {}
}

describe("registry probe", () => {
	it("shows all registered data", () => {
		const plugins = listPlugins();
		const handlers = getHandlersByEventType(EventType.AdapterMessage, false);
		process.stdout.write("P:" + JSON.stringify(plugins.map(p => ({ n: p.name, a: p.activated }))) + "\n");
		process.stdout.write("H:" + JSON.stringify(handlers.map(h => ({ f: h.fullName, m: h.modulePath, e: h.enabled, filt: h.filters.length }))) + "\n");
	});
});
