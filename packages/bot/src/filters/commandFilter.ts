import type { BotEvent, MessageEvent, ParsedCommand } from "../core/types";
import type { FilterContext, HandlerFilter } from "./handlerFilter";

export interface CommandFilterOptions {
	prefix?: string;
	alias?: string[];
	ignorePrefixCase?: boolean;
}

function parseCommand(content: string, prefix: string): ParsedCommand | undefined {
	if (!content.startsWith(prefix)) return undefined;
	const raw = content.slice(prefix.length).trim();
	if (!raw) return undefined;
	const parts = raw.split(/\s+/);
	return {
		name: parts[0],
		args: parts.slice(1),
		raw,
	};
}

export class CommandFilter implements HandlerFilter {
	private readonly command: string;
	private readonly prefix: string;
	private readonly alias: string[];
	private readonly ignoreCase: boolean;

	constructor(command: string, options: CommandFilterOptions = {}) {
		this.command = options.ignorePrefixCase ? command.toLowerCase() : command;
		this.prefix = options.prefix ?? "/";
		this.alias = options.alias ?? [];
		this.ignoreCase = options.ignorePrefixCase ?? false;
	}

	filter(event: BotEvent, _ctx: FilterContext): boolean {
		if (event.eventType !== ("AdapterMessage" as BotEvent["eventType"])) {
			return false;
		}
		const msg = event as MessageEvent;
		const parsed = parseCommand(msg.content, this.prefix);
		if (!parsed) return false;
		const name = this.ignoreCase ? parsed.name.toLowerCase() : parsed.name;
		if (name === this.command || this.alias.includes(name)) {
			msg.isCommand = true;
			msg.rawCommand = { ...parsed, alias: name === this.command ? undefined : name };
			return true;
		}
		return false;
	}
}
