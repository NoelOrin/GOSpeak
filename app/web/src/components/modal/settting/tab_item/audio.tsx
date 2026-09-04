import { createSignal, onCleanup, onMount, Show } from "solid-js";
import AudioDeviceStore, {
	type AudioBitrate,
	type AudioInputMode,
	type AudioSampleRate,
	type AudioSampleSize,
} from "@/stores/audioDeviceStore";
import { Field, Page, Section, Toggle } from "./shared";
import type { SettingTabConfig } from "./types";

const BITRATE_OPTIONS: { value: AudioBitrate; label: string }[] = [
	{ value: 32000, label: "32 kbps · 低带宽" },
	{ value: 48000, label: "48 kbps · 省流" },
	{ value: 64000, label: "64 kbps · 推荐" },
	{ value: 96000, label: "96 kbps · 高音质" },
	{ value: 128000, label: "128 kbps · 音乐/广播" },
];

const SAMPLE_RATE_OPTIONS: { value: AudioSampleRate; label: string }[] = [
	{ value: 44100, label: "44100 Hz（CD）" },
	{ value: 48000, label: "48000 Hz（推荐）" },
];

const SAMPLE_SIZE_OPTIONS: { value: AudioSampleSize; label: string }[] = [
	{ value: 8, label: "8 bit" },
	{ value: 16, label: "16 bit（推荐）" },
	{ value: 24, label: "24 bit" },
	{ value: 32, label: "32 bit" },
];

const INPUT_MODE_OPTIONS: {
	value: AudioInputMode;
	label: string;
	desc: string;
}[] = [
	{
		value: "voice",
		label: "语音通话",
		desc: "开启回声消除 / 噪声抑制，适合游戏麦",
	},
	{
		value: "music",
		label: "音乐 / 乐器",
		desc: "关闭处理链，保留更多原声与立体声",
	},
];

const AudioForm = () => {
	const [testingMic, setTestingMic] = createSignal(false);
	const [micLevel, setMicLevel] = createSignal(0);
	let testStream: MediaStream | null = null;
	let analyser: AnalyserNode | null = null;
	let audioCtx: AudioContext | null = null;
	let raf = 0;

	const stopMicTest = () => {
		if (raf) cancelAnimationFrame(raf);
		raf = 0;
		testStream?.getTracks().forEach((t) => {
			t.stop();
		});
		testStream = null;
		void audioCtx?.close();
		audioCtx = null;
		analyser = null;
		setMicLevel(0);
		setTestingMic(false);
	};

	const startMicTest = async () => {
		if (testingMic()) {
			stopMicTest();
			return;
		}
		try {
			const constraints: MediaStreamConstraints = {
				audio: {
					deviceId: AudioDeviceStore.state.selectedAudioInput
						? { exact: AudioDeviceStore.state.selectedAudioInput }
						: undefined,
					echoCancellation: AudioDeviceStore.state.echoCancellation,
					noiseSuppression: AudioDeviceStore.state.noiseSuppression,
					autoGainControl: AudioDeviceStore.state.autoGainControl,
				},
				video: false,
			};
			testStream = await navigator.mediaDevices.getUserMedia(constraints);
			audioCtx = new AudioContext();
			const source = audioCtx.createMediaStreamSource(testStream);
			analyser = audioCtx.createAnalyser();
			analyser.fftSize = 256;
			source.connect(analyser);
			const data = new Uint8Array(analyser.frequencyBinCount);
			const tick = () => {
				if (!analyser) return;
				analyser.getByteTimeDomainData(data);
				let sum = 0;
				for (let i = 0; i < data.length; i++) {
					const v = (data[i] - 128) / 128;
					sum += v * v;
				}
				const rms = Math.sqrt(sum / data.length);
				setMicLevel(Math.min(100, Math.round(rms * 220)));
				raf = requestAnimationFrame(tick);
			};
			setTestingMic(true);
			tick();
		} catch (err) {
			console.error("mic test failed", err);
			stopMicTest();
		}
	};

	onMount(() => {
		void AudioDeviceStore.fetchAudioDevices();
		const onDeviceChange = () => {
			void AudioDeviceStore.fetchAudioDevices();
		};
		navigator.mediaDevices?.addEventListener?.("devicechange", onDeviceChange);
		onCleanup(() => {
			navigator.mediaDevices?.removeEventListener?.(
				"devicechange",
				onDeviceChange,
			);
			stopMicTest();
		});
	});

	return (
		<Page
			title="音频"
			desc="设备、采集质量与传输参数。更改后通常需重新进入房间生效。"
		>
			<Section
				title="设备"
				action={
					<button
						type="button"
						class="btn btn-ghost btn-xs"
						onClick={() => void AudioDeviceStore.fetchAudioDevices()}
					>
						刷新设备
					</button>
				}
			>
				<Field label="输入设备（麦克风）">
					<select
						class="select w-full"
						value={AudioDeviceStore.state.selectedAudioInput}
						onChange={(e) =>
							AudioDeviceStore.setSelectedAudioInput(e.currentTarget.value)
						}
					>
						{AudioDeviceStore.state.audioinputs.map((d) => (
							<option value={d.deviceId}>
								{d.label || `麦克风 (${d.deviceId.slice(0, 8)})`}
							</option>
						))}
					</select>
				</Field>

				<Field
					label="输出设备（扬声器）"
					hint="部分浏览器仅支持 setSinkId；不支持时将使用系统默认输出。"
				>
					<select
						class="select w-full"
						value={AudioDeviceStore.state.selectedAudioOutput}
						onChange={(e) =>
							AudioDeviceStore.setSelectedAudioOutput(e.currentTarget.value)
						}
					>
						{AudioDeviceStore.state.audiooutputs.map((d) => (
							<option value={d.deviceId}>
								{d.label || `扬声器 (${d.deviceId.slice(0, 8)})`}
							</option>
						))}
					</select>
				</Field>

				<div class="rounded-box border border-base-300 bg-base-200/40 p-3">
					<div class="mb-2 flex items-center justify-between gap-3">
						<div>
							<div class="text-sm font-medium">麦克风测试</div>
							<div class="text-xs text-base-content/50">
								本地电平检测，不会推流
							</div>
						</div>
						<button
							type="button"
							class="btn btn-sm btn-outline"
							classList={{ "btn-error": testingMic() }}
							onClick={() => void startMicTest()}
						>
							{testingMic() ? "停止" : "开始测试"}
						</button>
					</div>
					<div class="h-2 w-full overflow-hidden rounded-full bg-base-300">
						<div
							class="h-full bg-success transition-[width] duration-75"
							style={{ width: `${micLevel()}%` }}
						/>
					</div>
				</div>
			</Section>

			<Section title="输入模式">
				<div class="grid gap-2 sm:grid-cols-2">
					{INPUT_MODE_OPTIONS.map((opt) => (
						<button
							type="button"
							class="rounded-box border p-3 text-left transition"
							classList={{
								"border-base-content/40 bg-base-200":
									AudioDeviceStore.state.inputMode === opt.value,
								"border-base-300 hover:border-base-content/20":
									AudioDeviceStore.state.inputMode !== opt.value,
							}}
							onClick={() => AudioDeviceStore.setInputMode(opt.value)}
						>
							<div class="text-sm font-medium">{opt.label}</div>
							<div class="mt-1 text-xs text-base-content/55">{opt.desc}</div>
						</button>
					))}
				</div>
			</Section>

			<Section title="采集质量">
				<div class="grid gap-3 sm:grid-cols-2">
					<Field label="音质（比特率）">
						<select
							class="select w-full"
							value={String(AudioDeviceStore.state.audioBitrate)}
							onChange={(e) =>
								AudioDeviceStore.setAudioBitrate(
									Number(e.target.value) as AudioBitrate,
								)
							}
						>
							{BITRATE_OPTIONS.map((o) => (
								<option value={String(o.value)}>{o.label}</option>
							))}
						</select>
					</Field>
					<Field label="采样率">
						<select
							class="select w-full"
							value={String(AudioDeviceStore.state.sampleRate)}
							onChange={(e) =>
								AudioDeviceStore.setSampleRate(
									Number(e.target.value) as AudioSampleRate,
								)
							}
						>
							{SAMPLE_RATE_OPTIONS.map((o) => (
								<option value={String(o.value)}>{o.label}</option>
							))}
						</select>
					</Field>
					<Field label="位深">
						<select
							class="select w-full"
							value={String(AudioDeviceStore.state.sampleSize)}
							onChange={(e) =>
								AudioDeviceStore.setSampleSize(
									Number(e.target.value) as AudioSampleSize,
								)
							}
						>
							{SAMPLE_SIZE_OPTIONS.map((o) => (
								<option value={String(o.value)}>{o.label}</option>
							))}
						</select>
					</Field>
				</div>
			</Section>

			<Section title="语音优化">
				<Toggle
					label="回声消除"
					desc="消除扬声器声音反馈到麦克风"
					checked={AudioDeviceStore.state.echoCancellation}
					onChange={AudioDeviceStore.setEchoCancellation}
				/>
				<Toggle
					label="噪声抑制"
					desc="过滤背景噪音"
					checked={AudioDeviceStore.state.noiseSuppression}
					onChange={AudioDeviceStore.setNoiseSuppression}
				/>
				<Toggle
					label="自动增益"
					desc="自动调节麦克风音量"
					checked={AudioDeviceStore.state.autoGainControl}
					onChange={AudioDeviceStore.setAutoGainControl}
				/>
				<Toggle
					label="人声隔离（实验性）"
					desc="比噪声抑制更强，开启后覆盖噪声抑制，浏览器支持有限"
					checked={AudioDeviceStore.state.voiceIsolation}
					onChange={AudioDeviceStore.setVoiceIsolation}
				/>
			</Section>

			<Section title="网络传输优化">
				<Toggle
					label="立体声"
					desc="以双声道发布音频"
					checked={AudioDeviceStore.state.stereo}
					onChange={AudioDeviceStore.setStereo}
				/>
				<Toggle
					label="DTX 不连续传输"
					desc="静音时停止发包，节省带宽"
					checked={AudioDeviceStore.state.dtx}
					onChange={AudioDeviceStore.setDtx}
				/>
				<Toggle
					label="RED 冗余音频"
					desc="发送冗余数据抗丢包，轻微增加带宽"
					checked={AudioDeviceStore.state.red}
					onChange={AudioDeviceStore.setRed}
				/>
			</Section>

			<div class="pt-2">
				<button
					type="button"
					class="btn btn-outline btn-sm"
					onClick={() => AudioDeviceStore.resetAudioDefaults()}
				>
					恢复音频默认值
				</button>
			</div>

			<Show when={AudioDeviceStore.state.inputMode === "music"}>
				<div class="alert alert-info text-sm">
					音乐模式已关闭常见语音处理；若出现回声，请改回「语音通话」。
				</div>
			</Show>
		</Page>
	);
};

const audio: SettingTabConfig = {
	id: "audio",
	label: "音频",
	icon: "mic",
	component: AudioForm,
};

export default audio;
