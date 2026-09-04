import type { HandlerMetadata, Plugin, PluginMetadata } from "./plugin";
import type { EventType } from "./types";

const handlerRegistry: HandlerMetadata[] = [];
const pluginMap = new Map<string, PluginMetadata>();
const pluginInstances = new Map<string, Plugin>();

export function registerHandler(md: HandlerMetadata): void {
	// Replace same module+handler if re-registering after reload.
	const idx = handlerRegistry.findIndex(
		(h) => h.modulePath === md.modulePath && h.handlerName === md.handlerName,
	);
	if (idx >= 0) handlerRegistry.splice(idx, 1);
	handlerRegistry.push(md);
	handlerRegistry.sort((a, b) => b.priority - a.priority);
	const plugin = pluginMap.get(md.modulePath);
	if (plugin && !plugin.handlerNames.includes(md.handlerName)) {
		plugin.handlerNames.push(md.handlerName);
	}
}

export function registerPlugin(modulePath: string, meta: PluginMetadata): void {
	pluginMap.set(modulePath, {
		...meta,
		handlerNames: meta.handlerNames ? [...meta.handlerNames] : [],
	});
}

export function bindPluginInstance(modulePath: string, instance: Plugin): void {
	pluginInstances.set(modulePath, instance);
}

export function getPluginInstance(modulePath: string): Plugin | undefined {
	return pluginInstances.get(modulePath);
}

export function getPluginMeta(modulePath: string): PluginMetadata | undefined {
	return pluginMap.get(modulePath);
}

export function findModulePathByName(name: string): string | undefined {
	for (const [modulePath, meta] of pluginMap) {
		if (meta.name === name) return modulePath;
	}
	return undefined;
}

export function getHandlersByEventType(
	eventType: EventType,
	onlyActivated = true,
): HandlerMetadata[] {
	return handlerRegistry.filter((h) => {
		if (h.eventType !== eventType) return false;
		if (!h.enabled) return false;
		if (onlyActivated) {
			const plugin = pluginMap.get(h.modulePath);
			if (plugin && !plugin.activated) return false;
		}
		return true;
	});
}

export function removeHandlersByModule(modulePath: string): void {
	for (let i = handlerRegistry.length - 1; i >= 0; i--) {
		if (handlerRegistry[i].modulePath === modulePath) {
			handlerRegistry.splice(i, 1);
		}
	}
	const meta = pluginMap.get(modulePath);
	if (meta) meta.handlerNames = [];
}

/** 完整卸载：handlers + plugin meta + instance */
export function removePlugin(modulePath: string): void {
	removeHandlersByModule(modulePath);
	pluginMap.delete(modulePath);
	pluginInstances.delete(modulePath);
}

export function listPlugins(): PluginMetadata[] {
	return [...pluginMap.values()];
}

export function setPluginActivated(name: string, activated: boolean): void {
	for (const meta of pluginMap.values()) {
		if (meta.name === name) meta.activated = activated;
	}
}

export function clearRegistry(): void {
	handlerRegistry.length = 0;
	pluginMap.clear();
	pluginInstances.clear();
}

export function bindHandlerInstances(instance: Plugin): void {
	const modulePath =
		(instance.constructor as any).__modulePath ?? instance.constructor.name;
	for (const h of handlerRegistry) {
		if (h.modulePath !== modulePath) continue;
		// Bind from the original unbound handler — re-binding an already-bound
		// function would not change `this`, so we cache the original first.
		const original = (h as any).__originalHandler ?? h.handler;
		(h as any).__originalHandler = original;
		h.handler = (original as any).bind(instance);
	}
}
