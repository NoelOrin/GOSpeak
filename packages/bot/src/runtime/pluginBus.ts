/**
 * First-class in-process plugin message bus.
 * Plugins publish/subscribe topics without importing each other.
 */

export interface PluginBusMessage<T = unknown> {
	/** Free-form topic, e.g. "moderation:kicked" */
	topic: string;
	/** Arbitrary payload */
	payload: T;
	/** Publisher plugin name */
	from: string;
	timestamp: number;
}

export type PluginBusHandler<T = unknown> = (
	msg: PluginBusMessage<T>,
) => void | Promise<void>;

export interface PluginBus {
	/**
	 * Publish a message to a topic.
	 * @returns number of handlers invoked
	 */
	publish<T = unknown>(topic: string, payload?: T): Promise<number>;
	/**
	 * Subscribe to a topic. Returns unsubscribe function.
	 * Wildcard: trailing `*` matches prefix, e.g. `moderation:*`
	 */
	subscribe<T = unknown>(
		topic: string,
		handler: PluginBusHandler<T>,
	): () => void;
	/** Subscribe once then auto-unsubscribe after first message. */
	once<T = unknown>(topic: string, handler: PluginBusHandler<T>): () => void;
}

interface SubEntry {
	id: number;
	pluginName: string;
	topic: string;
	handler: PluginBusHandler;
	once: boolean;
}

function topicMatches(pattern: string, topic: string): boolean {
	if (pattern === topic) return true;
	if (pattern.endsWith("*")) {
		const prefix = pattern.slice(0, -1);
		return topic.startsWith(prefix);
	}
	return false;
}

/**
 * Host-owned bus. Each plugin gets a PluginBus facade via forPlugin().
 * Subscriptions are cleaned on plugin unload / bot stop.
 */
export class PluginBusHost {
	private subs = new Map<number, SubEntry>();
	private nextId = 1;
	private publishing = 0;
	private pendingRemovals: number[] = [];

	forPlugin(pluginName: string): PluginBus {
		return {
			publish: (topic, payload) => this.publish(pluginName, topic, payload),
			subscribe: (topic, handler) =>
				this.subscribe(pluginName, topic, handler as PluginBusHandler, false),
			once: (topic, handler) =>
				this.subscribe(pluginName, topic, handler as PluginBusHandler, true),
		};
	}

	async publish(
		from: string,
		topic: string,
		payload?: unknown,
	): Promise<number> {
		const msg: PluginBusMessage = {
			topic,
			payload,
			from,
			timestamp: Date.now(),
		};
		const targets = [...this.subs.values()].filter((s) =>
			topicMatches(s.topic, topic),
		);
		this.publishing++;
		try {
			for (const sub of targets) {
				try {
					await sub.handler(msg);
				} catch {
					// do not block other subscribers
				}
				if (sub.once) this.subs.delete(sub.id);
			}
		} finally {
			this.publishing--;
			if (this.publishing === 0 && this.pendingRemovals.length) {
				for (const id of this.pendingRemovals) this.subs.delete(id);
				this.pendingRemovals = [];
			}
		}
		return targets.length;
	}

	private subscribe(
		pluginName: string,
		topic: string,
		handler: PluginBusHandler,
		once: boolean,
	): () => void {
		const id = this.nextId++;
		this.subs.set(id, { id, pluginName, topic, handler, once });
		return () => {
			if (this.publishing > 0) this.pendingRemovals.push(id);
			else this.subs.delete(id);
		};
	}

	/** Drop all subscriptions owned by a plugin (unload). */
	clearPlugin(pluginName: string): void {
		for (const [id, sub] of this.subs) {
			if (sub.pluginName === pluginName) this.subs.delete(id);
		}
	}

	clearAll(): void {
		this.subs.clear();
		this.pendingRemovals = [];
	}

	get size(): number {
		return this.subs.size;
	}
}
