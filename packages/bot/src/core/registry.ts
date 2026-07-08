import type { EventType } from "./types";
import type { HandlerMetadata, Plugin, PluginMetadata } from "./plugin";

const handlerRegistry: HandlerMetadata[] = [];
const pluginMap = new Map<string, PluginMetadata>();
const pluginInstances = new Map<string, Plugin>();

export function registerHandler(md: HandlerMetadata): void {
	handlerRegistry.push(md);
	handlerRegistry.sort((a, b) => b.priority - a.priority);
	const plugin = pluginMap.get(md.modulePath);
	if (plugin) plugin.handlerNames.push(md.handlerName);
}

export function registerPlugin(modulePath: string, meta: PluginMetadata): void {
	pluginMap.set(modulePath, meta);
}

export function bindPluginInstance(modulePath: string, instance: Plugin): void {
	pluginInstances.set(modulePath, instance);
}

export function getPluginInstance(modulePath: string): Plugin | undefined {
	return pluginInstances.get(modulePath);
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
