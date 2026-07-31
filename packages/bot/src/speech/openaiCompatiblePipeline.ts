import { concatPcm16, pcm16ToWav, pcmRms } from "../media/audio";
import type { AudioFrameEvent } from "../media/types";
import type { Logger } from "../core/context";
import type { SpeechPipeline, SpeechResult } from "./types";

type TranscribeFn = (
	wav: Buffer,
	options: { model: string; language?: string },
) => Promise<{ text: string; confidence?: number }>;

export interface OpenAICompatibleSpeechPipelineOptions {
	/** Full URL, e.g. https://api.openai.com/v1/audio/transcriptions */
	apiUrl: string;
	apiKey?: string;
	model?: string;
	language?: string;
	minSilenceMs?: number;
	maxChunkMs?: number;
	minChunkMs?: number;
	vadThreshold?: number;
	logger?: Logger;
	/** Test override for the HTTP transcription call. */
	transcribe?: TranscribeFn;
}

interface SpeakerBuffer {
	room: string;
	identity: string;
	frames: Int16Array[];
	sampleRate: number;
	firstAt: number;
	lastAt: number;
	lastSpeechAt: number;
	active: boolean;
	sending: Promise<void> | null;
}

/**
 * Real speech pipeline for OpenAI-compatible `/audio/transcriptions` APIs.
 *
 * PCM is segmented with a lightweight energy VAD, encoded to WAV, sent to the
 * remote endpoint and emitted as final SpeechResult events. It never invents
 * transcript text.
 */
export class OpenAICompatibleSpeechPipeline implements SpeechPipeline {
	private options: Required<
		Pick<
			OpenAICompatibleSpeechPipelineOptions,
			"model" | "minSilenceMs" | "maxChunkMs" | "minChunkMs" | "vadThreshold"
		>
	> &
		OpenAICompatibleSpeechPipelineOptions;
	private listeners = new Set<(r: SpeechResult) => void>();
	private buffers = new Map<string, SpeakerBuffer>();
	private transcribe: TranscribeFn;

	constructor(options: OpenAICompatibleSpeechPipelineOptions) {
		if (!options.apiUrl && !options.transcribe) {
			throw new Error("OpenAI-compatible ASR requires apiUrl");
		}
		this.options = {
			model: options.model ?? "whisper-1",
			minSilenceMs: options.minSilenceMs ?? 700,
			maxChunkMs: options.maxChunkMs ?? 15_000,
			minChunkMs: options.minChunkMs ?? 300,
			vadThreshold: options.vadThreshold ?? 350,
			...options,
		};
		this.transcribe = options.transcribe ?? this.transcribeViaHttp.bind(this);
	}

	write(frame: AudioFrameEvent): void {
		const key = `${frame.room}:${frame.identity}`;
		const now = frame.timestamp || Date.now();
		let state = this.buffers.get(key);
		if (!state) {
			state = {
				room: frame.room,
				identity: frame.identity,
				frames: [],
				sampleRate: frame.sampleRate,
				firstAt: now,
				lastAt: now,
				lastSpeechAt: now,
				active: false,
				sending: null,
			};
			this.buffers.set(key, state);
		}
		if (state.sampleRate !== frame.sampleRate) return;

		state.frames.push(frame.pcm16.slice());
		state.lastAt = now;
		if (pcmRms(frame.pcm16) >= this.options.vadThreshold) {
			state.active = true;
			state.lastSpeechAt = now;
		}

		if (
			state.active &&
			(state.lastAt - state.lastSpeechAt >= this.options.minSilenceMs ||
				state.lastAt - state.firstAt >= this.options.maxChunkMs)
		) {
			void this.flush(key, state, false);
		}
	}

	endTrack(room: string, identity: string): void {
		const key = `${room}:${identity}`;
		const state = this.buffers.get(key);
		if (!state) return;
		void this.flush(key, state, true);
	}

	onResult(cb: (result: SpeechResult) => void): () => void {
		this.listeners.add(cb);
		return () => this.listeners.delete(cb);
	}

	dispose(): void {
		for (const [key, state] of [...this.buffers]) {
			void this.flush(key, state, true);
		}
		this.buffers.clear();
		this.listeners.clear();
	}

	private async flush(
		key: string,
		state: SpeakerBuffer,
		force: boolean,
	): Promise<void> {
		if (state.sending) return;
		const pcm = concatPcm16(state.frames);
		const durationMs = (pcm.length / state.sampleRate) * 1000;
		if (pcm.length === 0) {
			this.buffers.delete(key);
			return;
		}
		if (!force && durationMs < this.options.minChunkMs) return;

		state.frames = [];
		const wav = pcm16ToWav(pcm, state.sampleRate, 1);
		state.sending = this.transcribe(wav, {
			model: this.options.model,
			language: this.options.language,
		})
			.then((result) => {
				const text = (result.text || "").trim();
				if (!text) return;
				this.emit({
					room: state.room,
					speaker: state.identity,
					text,
					isFinal: true,
					confidence: result.confidence,
					language: this.options.language,
					provider: "openai-compatible",
					timestamp: Date.now(),
				});
			})
			.catch((err) => {
				this.options.logger?.error(
					`[speech] transcription failed ${state.room}:${state.identity}:`,
					err,
				);
			})
			.finally(() => {
				state.sending = null;
			});
		await state.sending;
	}

	private async transcribeViaHttp(
		wav: Buffer,
		options: { model: string; language?: string },
	): Promise<{ text: string; confidence?: number }> {
		const form = new FormData();
		form.append(
			"file",
			new Blob([new Uint8Array(wav)], { type: "audio/wav" }),
			"speech.wav",
		);
		form.append("model", options.model);
		if (options.language) form.append("language", options.language);

		const headers: Record<string, string> = {};
		if (this.options.apiKey) {
			headers.Authorization = `Bearer ${this.options.apiKey}`;
		}
		const res = await fetch(this.options.apiUrl, {
			method: "POST",
			headers,
			body: form,
		});
		if (!res.ok) {
			throw new Error(`transcription API ${res.status}: ${await res.text()}`);
		}
		const json = (await res.json()) as {
			text?: string;
			confidence?: number;
		};
		return { text: json.text ?? "", confidence: json.confidence };
	}

	private emit(result: SpeechResult): void {
		for (const cb of this.listeners) {
			try {
				cb(result);
			} catch {
				// ignore
			}
		}
	}
}
