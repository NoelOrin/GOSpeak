import type { BotEvent } from "./types";
import type { BotContext } from "./context";
import { getHandlersByEventType } from "./registry";
import type { FilterContext } from "../filters/handlerFilter";

export interface DispatchOptions {
	stopOnFirstResult?: boolean;
}

export interface DispatchResult {
	matched: number;
	executed: number;
	errors: { handler: string; error: Error }[];
	lastResult: unknown;
}

export class EventBus {
	private readonly buildContext: (pluginName: string) => BotContext;
	private readonly getPluginConfig: (pluginName: string) => Record<string, unknown>;

	constructor(opts: {
		buildContext: (pluginName: string) => BotContext;
		getPluginConfig: (pluginName: string) => Record<string, unknown>;
	}) {
		this.buildContext = opts.buildContext;
		this.getPluginConfig = opts.getPluginConfig;
	}

	async dispatch(
		event: BotEvent,
		options: DispatchOptions = {},
	): Promise<DispatchResult> {
		const handlers = getHandlersByEventType(event.eventType);
		const result: DispatchResult = {
			matched: handlers.length,
			executed: 0,
			errors: [],
			lastResult: undefined,
		};

		for (const md of handlers) {
			const filterCtx: FilterContext = {
				config: this.getPluginConfig(md.modulePath),
			};
			let pass = true;
			for (const filter of md.filters) {
				const ok = await filter.filter(event, filterCtx);
				if (!ok) {
					pass = false;
					break;
				}
			}
			if (!pass) continue;

			const ctx = this.buildContext(md.modulePath);
			try {
				result.lastResult = await md.handler(event, ctx);
				result.executed++;
				if (options.stopOnFirstResult && result.lastResult !== undefined) {
					break;
				}
			} catch (err) {
				result.errors.push({
					handler: md.fullName,
					error: err instanceof Error ? err : new Error(String(err)),
				});
			}
		}

		return result;
	}
}
