import type { RemoteAudioTrackLike } from "./types";

export class SRSRemoteAudioTrack implements RemoteAudioTrackLike {
	private elements: HTMLAudioElement[] = [];

	constructor(private readonly stream: MediaStream) {}

	attach(): HTMLMediaElement {
		const element = document.createElement("audio");
		element.autoplay = true;
		element.srcObject = this.stream;
		this.elements.push(element);
		return element;
	}

	detach(): HTMLMediaElement[] {
		const detached = [...this.elements];
		this.elements = [];
		for (const element of detached) {
			element.pause();
			element.srcObject = null;
			element.remove();
		}
		return detached;
	}

	setVolume(volume: number): void {
		for (const element of this.elements) {
			element.volume = Math.max(0, Math.min(1, volume));
		}
	}
}
