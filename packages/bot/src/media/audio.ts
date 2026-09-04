/**
 * Small audio helpers shared by TTS, listen and speech pipelines.
 */

/** Convert a Float32Array of samples in [-1, 1] to signed 16-bit PCM. */
export function float32ToPcm16(input: Float32Array, gain = 1): Int16Array {
	const out = new Int16Array(input.length);
	for (let i = 0; i < input.length; i++) {
		const v = Math.max(-1, Math.min(1, (input[i] ?? 0) * gain));
		out[i] = v < 0 ? Math.round(v * 32768) : Math.round(v * 32767);
	}
	return out;
}

/** Linear-resample a mono Int16Array from one sample rate to another. */
export function resampleMonoPcm16(
	input: Int16Array,
	fromRate: number,
	toRate: number,
): Int16Array {
	if (fromRate === toRate) return input.slice();
	if (fromRate <= 0 || toRate <= 0) return input.slice();
	const ratio = toRate / fromRate;
	const outLen = Math.max(1, Math.round(input.length * ratio));
	const out = new Int16Array(outLen);
	for (let i = 0; i < outLen; i++) {
		const pos = i / ratio;
		const i0 = Math.min(input.length - 1, Math.floor(pos));
		const i1 = Math.min(input.length - 1, i0 + 1);
		const frac = pos - i0;
		out[i] = Math.round(
			(input[i0] ?? 0) + ((input[i1] ?? 0) - (input[i0] ?? 0)) * frac,
		);
	}
	return out;
}

/** Encode signed 16-bit PCM as a WAV buffer for HTTP transcription endpoints. */
/** Concatenate PCM frames into one mono Int16Array. */
export function concatPcm16(frames: Int16Array[]): Int16Array {
	const length = frames.reduce((sum, frame) => sum + frame.length, 0);
	const out = new Int16Array(length);
	let offset = 0;
	for (const frame of frames) {
		out.set(frame, offset);
		offset += frame.length;
	}
	return out;
}

/** RMS level of a PCM buffer, normalized roughly to signed 16-bit amplitude. */
export function pcmRms(pcm16: Int16Array): number {
	if (pcm16.length === 0) return 0;
	let sum = 0;
	for (const sample of pcm16) sum += sample * sample;
	return Math.sqrt(sum / pcm16.length);
}

export function pcm16ToWav(
	pcm16: Int16Array,
	sampleRate = 16000,
	channels = 1,
): Buffer {
	const bytesPerSample = 2;
	const blockAlign = channels * bytesPerSample;
	const byteRate = sampleRate * blockAlign;
	const dataSize = pcm16.length * bytesPerSample;
	const buffer = Buffer.alloc(44 + dataSize);

	buffer.write("RIFF", 0, "ascii");
	buffer.writeUInt32LE(36 + dataSize, 4);
	buffer.write("WAVE", 8, "ascii");
	buffer.write("fmt ", 12, "ascii");
	buffer.writeUInt32LE(16, 16);
	buffer.writeUInt16LE(1, 20);
	buffer.writeUInt16LE(channels, 22);
	buffer.writeUInt32LE(sampleRate, 24);
	buffer.writeUInt32LE(byteRate, 28);
	buffer.writeUInt16LE(blockAlign, 32);
	buffer.writeUInt16LE(16, 34);
	buffer.write("data", 36, "ascii");
	buffer.writeUInt32LE(dataSize, 40);

	for (let i = 0; i < pcm16.length; i++) {
		buffer.writeInt16LE(pcm16[i] ?? 0, 44 + i * bytesPerSample);
	}
	return buffer;
}
