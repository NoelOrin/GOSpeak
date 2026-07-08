import type { BotEvent } from "../core/types";

export interface FilterContext {
	config: Record<string, unknown>;
}

export interface HandlerFilter {
	filter(event: BotEvent, ctx: FilterContext): boolean | Promise<boolean>;
}

export function isHandlerFilter(value: unknown): value is HandlerFilter {
	return (
		typeof value === "object" &&
		value !== null &&
		typeof (value as HandlerFilter).filter === "function"
	);
}
