import type { ListenRoomChange } from "./listenTypes";

type ChangeListener = (change: ListenRoomChange) => void;

/**
 * Desired listen-room set with priority:
 * runtime commands > BotConfig.listenRooms > GOSPEAK_LISTEN_ROOMS
 */
export class ListenRoomRegistry {
	private rooms = new Set<string>();
	private listeners = new Set<ChangeListener>();

	constructor(initial: string[] = []) {
		for (const room of initial) this.rooms.add(room);
	}

	list(): string[] {
		return [...this.rooms].sort();
	}

	has(room: string): boolean {
		return this.rooms.has(room);
	}

	add(room: string): boolean {
		const name = room.trim();
		if (!name || this.rooms.has(name)) return false;
		this.rooms.add(name);
		this.emit({ added: [name], removed: [], current: this.list() });
		return true;
	}

	remove(room: string): boolean {
		const name = room.trim();
		if (!this.rooms.delete(name)) return false;
		this.emit({ added: [], removed: [name], current: this.list() });
		return true;
	}

	clear(): string[] {
		const removed = this.list();
		if (removed.length === 0) return [];
		this.rooms.clear();
		this.emit({ added: [], removed, current: [] });
		return removed;
	}

	setAll(rooms: string[]): ListenRoomChange {
		const next = new Set(rooms.map((r) => r.trim()).filter(Boolean));
		const current = new Set(this.rooms);
		const added = [...next].filter((r) => !current.has(r));
		const removed = [...current].filter((r) => !next.has(r));
		this.rooms = next;
		const change = { added, removed, current: this.list() };
		if (added.length || removed.length) this.emit(change);
		return change;
	}

	onChange(cb: ChangeListener): () => void {
		this.listeners.add(cb);
		return () => this.listeners.delete(cb);
	}

	private emit(change: ListenRoomChange): void {
		for (const cb of this.listeners) {
			try {
				cb(change);
			} catch {
				// ignore listener errors
			}
		}
	}
}

export function parseRoomList(input?: string | string[] | null): string[] {
	if (!input) return [];
	if (Array.isArray(input)) {
		return [...new Set(input.map((s) => s.trim()).filter(Boolean))];
	}
	return [
		...new Set(
			input
				.split(",")
				.map((s) => s.trim())
				.filter(Boolean),
		),
	];
}
