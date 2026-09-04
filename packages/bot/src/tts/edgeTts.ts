import { MPEGDecoder } from "mpg123-decoder";
import { EdgeTTS } from "@seepine/edge-tts";
import { float32ToPcm16, resampleMonoPcm16 } from "../media/audio";
import type { TTSProvider } from "./types";

export interface EdgeTTSOptions {
	voice?: string;
	lang?: string;
	rate?: string;
	pitch?: string;
	volume?: string;
	timeout?: number;
	/** Target output sample rate of synthesize(). Defaults to 16000. */
	sampleRate?: number;
}

interface EdgeTTSClientLike {
	call(text: string): Promise<{
		data: Uint8Array;
		subtitles?: Array<{ part: string; start: number; end: number }>;
	}>;
}

interface DecoderLike {
	ready: Promise<void>;
	decode(input: Uint8Array): {
		channelData: Float32Array[];
		sampleRate: number;
	};
	free(): void;
}

export interface EdgeTTSProviderDeps {
	client?: EdgeTTSClientLike;
	decoder?: DecoderLike;
}

/**
 * Real TTS provider backed by Microsoft Edge's online neural voices.
 *
 * Edge returns MP3, so the provider decodes it with mpg123 (WASM), resamples
 * to the bot's target rate and returns signed 16-bit PCM.
 */
export class EdgeTTSProvider implements TTSProvider {
	readonly name = "edge";
	private options: Required<
		Pick<EdgeTTSOptions, "voice" | "lang" | "sampleRate">
	> &
		EdgeTTSOptions;
	private client: EdgeTTSClientLike;
	private decoder?: DecoderLike;

	constructor(options: EdgeTTSOptions = {}, deps: EdgeTTSProviderDeps = {}) {
		this.options = {
			voice: options.voice ?? "zh-CN-XiaoxiaoNeural",
			lang: options.lang ?? "zh-CN",
			sampleRate: options.sampleRate ?? 16000,
			...options,
		};
		this.client =
			deps.client ??
			new EdgeTTS({
				voice: this.options.voice,
				lang: this.options.lang,
				rate: this.options.rate,
				pitch: this.options.pitch,
				volume: this.options.volume,
				timeout: this.options.timeout,
			});
		this.decoder = deps.decoder;
	}

	async synthesize(text: string): Promise<Int16Array> {
		const trimmed = text.trim();
		if (!trimmed) return new Int16Array(0);

		const res = await this.client.call(trimmed);
		const data = new Uint8Array(
			res.data.buffer,
			res.data.byteOffset,
			res.data.byteLength,
		);
		if (data.byteLength === 0) {
			throw new Error("edge tts returned empty audio");
		}

		const decoder = this.createDecoder();
		try {
			await decoder.ready;
			const decoded = decoder.decode(data);
			const channel = decoded.channelData[0];
			if (!channel || channel.length === 0) {
				throw new Error("edge tts returned no decoded audio");
			}
			const pcm = float32ToPcm16(channel);
			return resampleMonoPcm16(
				pcm,
				decoded.sampleRate,
				this.options.sampleRate,
			);
		} finally {
			decoder.free();
		}
	}

	private createDecoder(): DecoderLike {
		return this.decoder ?? new MPEGDecoder();
	}
}
