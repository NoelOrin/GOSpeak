import type { BotEvent, MessageEvent } from "../core/types";
import type { FilterContext, HandlerFilter } from "./handlerFilter";

export class RegexFilter implements HandlerFilter {
	private readonly pattern: RegExp;

	constructor(pattern: RegExp | string) {
		this.pattern = typeof pattern === "string" ? new RegExp(pattern) : pattern;
	}

	filter(event: BotEvent, _ctx: FilterContext): boolean {
		if (event.eventType !== ("AdapterMessage" as BotEvent["eventType"])) {
			return false;
		}
		return this.pattern.test((event as MessageEvent).content);
	}
}
