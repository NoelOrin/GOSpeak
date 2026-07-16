import type {
	SFUListenAdapter,
	SFUListenJoinParams,
	SFUProviderName,
} from "../listenTypes";
import type { AudioFrameEvent } from "../types";

export class UnsupportedListenAdapter implements SFUListenAdapter {
	constructor(public readonly provider: SFUProviderName) {}

	async join(_params: SFUListenJoinParams): Promise<void> {
		throw new Error(
			`SFU listen adapter not implemented for provider: ${this.provider}`,
		);
	}

	async leave(_room: string): Promise<void> {
		// no-op
	}

	onAudioFrame(_cb: (frame: AudioFrameEvent) => void): void {
		// no-op
	}

	onTrackEnded(_cb: (info: { room: string; identity: string }) => void): void {
		// no-op
	}

	listActiveIdentities(_room: string): string[] {
		return [];
	}

	async dispose(): Promise<void> {
		// no-op
	}
}
