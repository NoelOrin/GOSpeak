import type { Plugin, PluginMetadata } from "../core/plugin";
import { registerPlugin } from "../core/registry";

export interface PluginMetaInput {
	name: string;
	author: string;
	desc: string;
	version: string;
	repo?: string;
}

export function RegisterPlugin(meta: PluginMetaInput) {
	return <T extends new (...args: any[]) => Plugin>(target: T): T => {
		const modulePath =
			(target as unknown as { __modulePath?: string }).__modulePath ??
			target.name;
		registerPlugin(modulePath, {
			name: meta.name,
			author: meta.author,
			desc: meta.desc,
			version: meta.version,
			repo: meta.repo,
			activated: true,
			handlerNames: [],
		});
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

export function getPluginModulePath(instance: Plugin): string {
	const ctor = instance.constructor as unknown as { __modulePath?: string };
	return ctor.__modulePath ?? instance.constructor.name;
}
