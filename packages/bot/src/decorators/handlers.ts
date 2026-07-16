import type { EventType } from "../core/types";
import { CommandFilter } from "../filters/commandFilter";
import type { HandlerFilter } from "../filters/handlerFilter";

export type HandlerFn = (event: any, ctx: any) => unknown | Promise<unknown>;

/** 装饰器阶段收集的 handler 元数据，真正注册推迟到 load/init。 */
export interface PendingHandlerMeta {
	eventType: EventType;
	handlerName: string;
	handler: HandlerFn;
	filters: HandlerFilter[];
	desc: string;
	priority: number;
}

type PluginCtor = {
	__pendingHandlers?: PendingHandlerMeta[];
};

function getCtor(target: object): PluginCtor {
	// 实例方法装饰器的 target 是 prototype
	return (target as { constructor: PluginCtor }).constructor;
}

function decorate(
	target: object,
	propertyKey: string,
	eventType: EventType,
	filters: HandlerFilter[],
	priority: number,
	desc?: string,
): void {
	const ctor = getCtor(target);
	if (!ctor.__pendingHandlers) ctor.__pendingHandlers = [];
	const handler = (target as Record<string, HandlerFn>)[propertyKey];
	// 同名 handler 覆盖，支持热重载场景下重复装饰
	const existing = ctor.__pendingHandlers.findIndex(
		(h) => h.handlerName === propertyKey,
	);
	const meta: PendingHandlerMeta = {
		eventType,
		handlerName: propertyKey,
		handler,
		filters,
		desc: desc ?? "",
		priority,
	};
	if (existing >= 0) ctor.__pendingHandlers[existing] = meta;
	else ctor.__pendingHandlers.push(meta);
}

export function getPendingHandlers(ctor: unknown): PendingHandlerMeta[] {
	const list = (ctor as PluginCtor).__pendingHandlers;
	return list ? [...list] : [];
}

export function On(
	eventType: EventType,
	options: { priority?: number; desc?: string; filters?: HandlerFilter[] } = {},
) {
	return (target: object, propertyKey: string): void => {
		decorate(
			target,
			propertyKey,
			eventType,
			options.filters ?? [],
			options.priority ?? 0,
			options.desc,
		);
	};
}

export function Command(
	name: string,
	options: {
		alias?: string[];
		prefix?: string;
		priority?: number;
		desc?: string;
		filters?: HandlerFilter[];
	} = {},
) {
	return (target: object, propertyKey: string): void => {
		const filters = [
			new CommandFilter(name, options),
			...(options.filters ?? []),
		];
		decorate(
			target,
			propertyKey,
			"AdapterMessage" as EventType,
			filters,
			options.priority ?? 0,
			options.desc,
		);
	};
}
