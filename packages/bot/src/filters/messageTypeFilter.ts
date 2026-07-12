import type { BotEvent, EventType } from "../core/types";
import type { FilterContext, HandlerFilter } from "./handlerFilter";

export class MessageTypeFilter implements HandlerFilter {
	private readonly allowed: EventType[];

	constructor(...allowed: EventType[]) {
		this.allowed = allowed;
	}

	filter(event: BotEvent, _ctx: FilterContext): boolean {
		return this.allowed.includes(event.eventType);
	}
}
