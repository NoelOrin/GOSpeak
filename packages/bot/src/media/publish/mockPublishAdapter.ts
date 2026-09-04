import type { SFUPublishAdapter } from "./types";

/** Test/dev publish adapter that records publish calls. */
export class MockPublishAdapter implements SFUPublishAdapter {
	public published: Array<{ room: string; samples: number }> = [];
	private rooms = new Set<string>();

	async join(params: {
		room: string;
		identity: string;
		token: string;
		serverUrl: string;
	}): Promise<void> {
		this.rooms.add(params.room);
	}

	async publishPcm(
		room: string,
		pcm16: Int16Array,
		_sampleRate = 16000,
	): Promise<void> {
		if (!this.rooms.has(room)) throw new Error(`not joined: ${room}`);
		this.published.push({ room, samples: pcm16.length });
	}

	async unpublish(room: string): Promise<void> {
		// no-op
		void room;
	}

	async leave(room: string): Promise<void> {
		this.rooms.delete(room);
	}

	async dispose(): Promise<void> {
		this.rooms.clear();
	}
}
