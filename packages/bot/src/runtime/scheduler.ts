export type SchedulerTask = () => void | Promise<void>;

/**
 * Lightweight timer registry for proactive plugin tasks.
 * IDs are namespaced by the caller (pluginName:taskId).
 */
export class Scheduler {
	private timers = new Map<
		string,
		{ kind: "interval" | "timeout"; handle: ReturnType<typeof setInterval> }
	>();

	every(id: string, ms: number, fn: SchedulerTask): void {
		this.clear(id);
		const handle = setInterval(() => {
			void Promise.resolve(fn()).catch(() => {
				// swallow task errors to keep the schedule alive
			});
		}, ms);
		// Avoid keeping the process alive solely for background tasks
		if (typeof handle.unref === "function") handle.unref();
		this.timers.set(id, { kind: "interval", handle });
	}

	once(id: string, ms: number, fn: SchedulerTask): void {
		this.clear(id);
		const handle = setTimeout(() => {
			this.timers.delete(id);
			void Promise.resolve(fn()).catch(() => {
				// swallow task errors
			});
		}, ms);
		if (typeof handle.unref === "function") handle.unref();
		this.timers.set(id, { kind: "timeout", handle });
	}

	clear(id: string): void {
		const entry = this.timers.get(id);
		if (!entry) return;
		if (entry.kind === "interval") clearInterval(entry.handle);
		else clearTimeout(entry.handle);
		this.timers.delete(id);
	}

	clearAll(): void {
		for (const id of [...this.timers.keys()]) this.clear(id);
	}

	has(id: string): boolean {
		return this.timers.has(id);
	}

	ids(): string[] {
		return [...this.timers.keys()];
	}

	clearByPrefix(prefix: string): void {
		for (const id of this.ids()) {
			if (id.startsWith(prefix)) this.clear(id);
		}
	}

	get size(): number {
		return this.timers.size;
	}
}
