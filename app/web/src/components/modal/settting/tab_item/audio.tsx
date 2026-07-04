import { onMount } from "solid-js";
import AudioDeviceStore, {
	type AudioBitrate,
	type AudioSampleRate,
	type AudioSampleSize,
} from "../../../../stores/audioDeviceStore";
import type { SettingTabConfig } from "./types";

const BITRATE_OPTIONS: { value: AudioBitrate; label: string }[] = [
	{ value: 32000, label: "32 kbps" },
	{ value: 48000, label: "48 kbps" },
	{ value: 64000, label: "64 kbps（推荐）" },
	{ value: 96000, label: "96 kbps" },
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

const Toggle = (props: {
	label: string;
	desc: string;
	checked: boolean;
	onChange: (v: boolean) => void;
}) => (
	<div class="flex items-center justify-between py-2">
		<div>
			<div class="text-sm font-medium">{props.label}</div>
			<div class="text-xs text-base-content/50">{props.desc}</div>
		</div>
		<input
			type="checkbox"
			class="toggle toggle-sm"
			checked={props.checked}
			onChange={(e) => props.onChange(e.target.checked)}
		/>
	</div>
);

const AudioForm = () => {
	onMount(() => {
		AudioDeviceStore.fetchAudioDevices();
	});

	return (
		<div class="p-4 flex flex-col gap-4">
			<h3 class="font-bold text-lg">音频</h3>
			<div class="divider my-0 text-xs text-base-content/40">设备设置</div>

			<fieldset class="fieldset">
				<legend class="fieldset-legend text-[14px]">输入设备</legend>
				<select class="select w-full">
					{AudioDeviceStore.state.audioinputs.map((d) => (
						<option value={d.deviceId}>
							{d.label || `麦克风 (${d.deviceId.slice(0, 8)})`}
						</option>
					))}
				</select>
			</fieldset>

			<fieldset class="fieldset">
				<legend class="fieldset-legend text-[14px]">输出设备</legend>
				<select class="select w-full">
					{AudioDeviceStore.state.audiooutputs.map((d) => (
						<option value={d.deviceId}>
							{d.label || `扬声器 (${d.deviceId.slice(0, 8)})`}
						</option>
					))}
				</select>
			</fieldset>

			<fieldset class="fieldset">
				<legend class="fieldset-legend text-[14px]">音质（比特率）</legend>
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
			</fieldset>

			<fieldset class="fieldset">
				<legend class="fieldset-legend text-[14px]">采样率</legend>
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
			</fieldset>

			<fieldset class="fieldset">
				<legend class="fieldset-legend text-[14px]">位深</legend>
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
			</fieldset>

			<div class="divider my-0 text-xs text-base-content/40">语音优化</div>

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

			<div class="divider my-0 text-xs text-base-content/40">网络传输优化</div>

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
		</div>
	);
};

const audio: SettingTabConfig = {
	label: "音频",
	component: AudioForm,
};

export default audio;
