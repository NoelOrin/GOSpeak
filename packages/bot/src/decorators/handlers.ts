import { registerHandler } from "../core/registry";
import type { EventType } from "../core/types";
import { CommandFilter } from "../filters/commandFilter";
import type { HandlerFilter } from "../filters/handlerFilter";
import { getPluginModulePath } from "./register";

type HandlerFn = (event: any, ctx: any) => unknown | Promise<unknown>;

function decorate(
	target: any,
	propertyKey: string,
	eventType: EventType,
	filters: HandlerFilter[],
	priority: number,
	desc?: string,
): void {
	const modulePath = getPluginModulePath(target);
	const handler = (target as unknown as Record<string, HandlerFn>)[propertyKey];
	registerHandler({
		eventType,
		modulePath,
		handlerName: propertyKey,
		fullName: `${modulePath}_${propertyKey}`,
		handler,
		filters,
		desc: desc ?? "",
		priority,
		enabled: true,
	});
}

export function On(
	eventType: EventType,
	options: { priority?: number; desc?: string; filters?: HandlerFilter[] } = {},
) {
	return (target: any, propertyKey: string): void => {
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
	return (target: any, propertyKey: string): void => {
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
