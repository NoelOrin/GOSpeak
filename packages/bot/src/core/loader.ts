import { pathToFileURL } from "node:url";
import { getPendingHandlers } from "../decorators/handlers";
import type { BotContext } from "./context";
import { Plugin, type PluginMetadata } from "./plugin";
import {
	bindHandlerInstances,
	bindPluginInstance,
	registerHandler,
	registerPlugin,
	removePlugin,
} from "./registry";

export interface LoadedPlugin {
	metadata: PluginMetadata;
	instance: Plugin;
	modulePath: string;
	absPath: string;
	importUrl: string;
}

async function importModule(spec: string): Promise<Record<string, unknown>> {
	// 绝对路径 / 相对路径统一走 file URL；query 用于 bust ESM 缓存
	const resolved =
		spec.startsWith("file:") ||
		spec.startsWith("data:") ||
		(!spec.startsWith(".") &&
			!spec.startsWith("/") &&
			!spec.match(/^[A-Za-z]:/))
			? spec
			: pathToFileURL(spec).href;
	const mod = (await import(resolved)) as Record<string, unknown>;
	return mod;
}

function collectPluginClasses(
	mod: Record<string, unknown>,
): new (
	...args: any[]
) => Plugin {
	const candidates = Object.values(mod).filter(
		(v): v is new (...args: any[]) => Plugin =>
			typeof v === "function" && (v as any).prototype instanceof Plugin,
	);
	if (candidates.length === 0) {
		throw new Error("no Plugin subclass exported in module");
	}
	return candidates[0];
}

/** 把装饰器挂在类上的 pending handlers 注册进全局 registry */
export function registerPendingHandlers(
	Cls: new (...args: any[]) => Plugin,
	modulePath: string,
): void {
	const pending = getPendingHandlers(Cls);
	for (const h of pending) {
		registerHandler({
			eventType: h.eventType,
			modulePath,
			handlerName: h.handlerName,
			fullName: `${modulePath}_${h.handlerName}`,
			handler: h.handler,
			filters: h.filters,
			desc: h.desc,
			priority: h.priority,
			enabled: true,
		});
	}
}

/**
 * 动态加载插件模块。
 * @param absPath 插件文件绝对路径
 * @param modulePath 逻辑模块路径（如 user_plugins/echo）
 * @param cacheBust 是否附加时间戳绕过 ESM 缓存（热重载时 true）
 */
export async function loadPlugin(
	absPath: string,
	modulePath: string,
	cacheBust = false,
): Promise<LoadedPlugin> {
	const baseUrl = pathToFileURL(absPath).href;
	const importUrl = cacheBust ? `${baseUrl}?t=${Date.now()}` : baseUrl;
	const mod = await importModule(importUrl);
	const Cls = collectPluginClasses(mod);
	(Cls as unknown as { __modulePath?: string }).__modulePath = modulePath;

	const metaFromDecorator = (
		Cls as unknown as { __pluginMeta?: PluginMetadata }
	).__pluginMeta;
	if (!metaFromDecorator) {
		throw new Error(
			`plugin class ${Cls.name} missing @RegisterPlugin metadata`,
		);
	}

	// 先清理同 modulePath 的旧注册，再注册新 meta + handlers
	removePlugin(modulePath);
	registerPlugin(modulePath, {
		...metaFromDecorator,
		handlerNames: [],
	});
	registerPendingHandlers(Cls, modulePath);

	const instance = new Cls();
	// 确保实例 metadata 与 registry 一致
	if (!instance.metadata) instance.metadata = { ...metaFromDecorator };
	instance.metadata = {
		...instance.metadata,
		handlerNames: [...(instance.metadata.handlerNames ?? [])],
	};
	bindPluginInstance(modulePath, instance);

	return {
		metadata: instance.metadata,
		instance,
		modulePath,
		absPath,
		importUrl,
	};
}

export function initPlugin(
	loaded: LoadedPlugin,
	buildContext: (pluginName: string) => BotContext,
): void {
	loaded.instance.init(buildContext(loaded.metadata.name));
	bindHandlerInstances(loaded.instance);
}

/** 卸载插件：调用 onUnload + 清理 registry */
export async function unloadPlugin(loaded: {
	instance: Plugin;
	modulePath: string;
}): Promise<void> {
	try {
		await loaded.instance.onUnload?.();
	} finally {
		removePlugin(loaded.modulePath);
	}
}
