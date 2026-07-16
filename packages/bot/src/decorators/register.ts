import type { Plugin, PluginMetadata } from "../core/plugin";

export interface PluginMetaInput {
	name: string;
	author: string;
	desc: string;
	version: string;
	repo?: string;
}

/**
 * 仅挂载插件元数据到类上，真正 registerPlugin 推迟到 loadPlugin。
 * 这样动态 import 时可以用正确的 modulePath 注册。
 */
export function RegisterPlugin(meta: PluginMetaInput) {
	return <T extends new (...args: any[]) => Plugin>(target: T): T => {
		(target as unknown as { __pluginMeta?: PluginMetadata }).__pluginMeta = {
			name: meta.name,
			author: meta.author,
			desc: meta.desc,
			version: meta.version,
			repo: meta.repo,
			activated: true,
			handlerNames: [],
		};
		return target;
	};
}

export function setPluginModulePath(
	target: new (...args: any[]) => Plugin,
	modulePath: string,
): void {
	(target as unknown as { __modulePath?: string }).__modulePath = modulePath;
}

export function getPluginModulePath(instance: Plugin | object): string {
	const ctor =
		typeof instance === "function"
			? (instance as unknown as { __modulePath?: string })
			: (instance.constructor as unknown as { __modulePath?: string });
	return (
		ctor.__modulePath ??
		(typeof instance === "function"
			? (instance as { name?: string }).name
			: instance.constructor.name) ??
		"unknown"
	);
}
