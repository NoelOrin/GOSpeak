import type { TTSProvider } from "./types";

/** MVP TTS: generate a short sine beep proportional to text length (audible placeholder). */
export class SineTTSProvider implements TTSProvider {
	readonly name = "sine";
	constructor(private sampleRate = 16000) {}

	async synthesize(text: string): Promise<Int16Array> {
		const seconds = Math.min(2, Math.max(0.3, text.length * 0.05));
		const n = Math.floor(this.sampleRate * seconds);
		const out = new Int16Array(n);
		const freq = 440;
		for (let i = 0; i < n; i++) {
			const t = i / this.sampleRate;
			const env = Math.min(1, i / 200) * Math.min(1, (n - i) / 200);
			out[i] = (Math.sin(2 * Math.PI * freq * t) * 0.3 * env * 32767) | 0;
		}
		return out;
	}
}
