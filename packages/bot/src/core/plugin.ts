import type { EventType } from "./types";
import type { HandlerFilter } from "../filters/handlerFilter";

export interface PluginMetadata {
	name: string;
	author: string;
	desc: string;
	version: string;
	repo?: string;
	activated: boolean;
	handlerNames: string[];
}

export interface HandlerMetadata {
	eventType: EventType;
	fullName: string;
	handlerName: string;
	modulePath: string;
	handler: (event: any, ctx: any) => unknown | Promise<unknown>;
	filters: HandlerFilter[];
	desc: string;
	priority: number;
	enabled: boolean;
}

export class Plugin {
	metadata!: PluginMetadata;
	protected ctx!: import("./context").BotContext;

	constructor() {
		const meta = (this.constructor as unknown as { __pluginMeta?: PluginMetadata })
			.__pluginMeta;
		if (meta) this.metadata = meta;
	}

	init(ctx: import("./context").BotContext): void {
		this.ctx = ctx;
		this.onLoad?.();
	}

	onLoad?(): void | Promise<void>;
	onUnload?(): void | Promise<void>;
}
