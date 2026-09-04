import type { BotEvent, MessageEvent, PermissionLevel } from "../core/types";
import type { FilterContext, HandlerFilter } from "./handlerFilter";

const RANK: Record<PermissionLevel, number> = {
	guest: 0,
	member: 1,
	moderator: 2,
	admin: 3,
	owner: 4,
};

export class PermissionFilter implements HandlerFilter {
	private readonly required: PermissionLevel;

	constructor(required: PermissionLevel) {
		this.required = required;
	}

	filter(event: BotEvent, _ctx: FilterContext): boolean {
		if (event.eventType !== ("AdapterMessage" as BotEvent["eventType"])) {
			return false;
		}
		const sender = (event as MessageEvent).sender;
		return RANK[sender.role] >= RANK[this.required];
	}
}
