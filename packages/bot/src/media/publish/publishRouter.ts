import type { SFUProviderName } from "../listenTypes";
import { LiveKitPublishAdapter } from "./livekitPublishAdapter";
import { MockPublishAdapter } from "./mockPublishAdapter";
import type { SFUPublishAdapter } from "./types";
import { UnsupportedPublishAdapter } from "./unsupportedPublishAdapter";

export interface SFUPublishRouterOptions {
	logger?: {
		info: (...a: unknown[]) => void;
		warn: (...a: unknown[]) => void;
	};
	createAdapter?: (provider: string) => SFUPublishAdapter;
}

/** Resolves publish adapters by provider so BotRunner can follow the active SFU backend. */
export class SFUPublishRouter {
	private adapters = new Map<string, SFUPublishAdapter>();
	private opts: SFUPublishRouterOptions;

	constructor(opts: SFUPublishRouterOptions = {}) {
		this.opts = opts;
	}

	get(provider?: string | null): SFUPublishAdapter {
		const name = (provider || "livekit") as SFUProviderName;
		const key = String(name);
		let adapter = this.adapters.get(key);
		if (adapter) return adapter;

		if (this.opts.createAdapter) {
			adapter = this.opts.createAdapter(key);
		} else if (key === "livekit") {
			adapter = new LiveKitPublishAdapter(this.opts.logger);
		} else if (key === "mock") {
			adapter = new MockPublishAdapter();
		} else {
			adapter = new UnsupportedPublishAdapter(key);
		}
		this.adapters.set(key, adapter);
		return adapter;
	}

	async disposeAll(): Promise<void> {
		for (const adapter of this.adapters.values()) await adapter.dispose();
		this.adapters.clear();
	}
}
