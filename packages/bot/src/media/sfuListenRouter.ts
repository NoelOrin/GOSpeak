import { LiveKitListenAdapter } from "./adapters/livekitListenAdapter";
import { UnsupportedListenAdapter } from "./adapters/unsupportedListenAdapter";
import type { SFUListenAdapter, SFUProviderName } from "./listenTypes";

export interface SFUListenRouterOptions {
	logger?: {
		info: (...a: unknown[]) => void;
		warn: (...a: unknown[]) => void;
		error: (...a: unknown[]) => void;
	};
	/** Optional override for tests */
	createAdapter?: (provider: SFUProviderName) => SFUListenAdapter;
}

export class SFUListenRouter {
	private adapters = new Map<SFUProviderName, SFUListenAdapter>();
	private opts: SFUListenRouterOptions;

	constructor(opts: SFUListenRouterOptions = {}) {
		this.opts = opts;
	}

	get(provider: SFUProviderName | string): SFUListenAdapter {
		const name = (provider || "livekit") as SFUProviderName;
		let adapter = this.adapters.get(name);
		if (adapter) return adapter;

		if (this.opts.createAdapter) {
			adapter = this.opts.createAdapter(name);
		} else if (name === "livekit") {
			adapter = new LiveKitListenAdapter(this.opts.logger);
		} else {
			adapter = new UnsupportedListenAdapter(name);
		}
		this.adapters.set(name, adapter);
		return adapter;
	}

	async disposeAll(): Promise<void> {
		for (const adapter of this.adapters.values()) {
			await adapter.dispose();
		}
		this.adapters.clear();
	}
}
