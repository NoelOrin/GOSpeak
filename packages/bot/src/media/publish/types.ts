export interface SFUPublishAdapter {
	join(params: {
		room: string;
		identity: string;
		token: string;
		serverUrl: string;
	}): Promise<void>;
	publishPcm(
		room: string,
		pcm16: Int16Array,
		sampleRate?: number,
	): Promise<void>;
	unpublish(room: string): Promise<void>;
	leave(room: string): Promise<void>;
	dispose(): Promise<void>;
}
