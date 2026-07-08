import { Plugin, type PluginMetadata } from "./plugin";
import { bindPluginInstance, registerPlugin } from "./registry";
import type { BotContext } from "./context";

export interface LoadedPlugin {
	metadata: PluginMetadata;
	instance: Plugin;
}

async function importModule(spec: string): Promise<Record<string, unknown>> {
	const resolved =
		spec.startsWith(".") || spec.startsWith("/")
			? new URL(spec, import.meta.url).pathname
			: spec;
	const mod = (await import(resolved)) as Record<string, unknown>;
	return mod;
}

function collectPluginClasses(mod: Record<string, unknown>): new (...args: any[]) => Plugin {
	const candidates = Object.values(mod).filter(
		(v): v is new (...args: any[]) => Plugin =>
			typeof v === "function" && (v as any).prototype instanceof Plugin,
	);
	if (candidates.length === 0) {
		throw new Error(`no Plugin subclass exported in module ${mod}`);
	}
	return candidates[0];
}

export async function loadPlugin(spec: string, modulePath: string): Promise<LoadedPlugin> {
	const mod = await importModule(spec);
	const Cls = collectPluginClasses(mod);
	(Cls as unknown as { __modulePath?: string }).__modulePath = modulePath;
	const instance = new Cls();
	bindPluginInstance(modulePath, instance);
	const metadata = instance.metadata;
	return { metadata, instance };
}

export function initPlugin(
	loaded: LoadedPlugin,
	buildContext: (pluginName: string) => BotContext,
): void {
	loaded.instance.init(buildContext(loaded.metadata.name));
}
