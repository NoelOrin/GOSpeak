import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@gospeak/components";
import AudioDeviceStore, {
	type AudioBitrate,
	type AudioInputMode,
	type AudioSampleRate,
	type AudioSampleSize,
	type useAudioDeviceStore,
} from "@/stores/audioDeviceStore";
import { Field, Section, Toggle } from "../shared";

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

export const QualitySection = ({
	store,
}: {
	store: ReturnType<typeof useAudioDeviceStore.getState>;
}) => (
	<>
		<Section title="输入模式">
			<div className="grid gap-2 sm:grid-cols-2">
				{INPUT_MODE_OPTIONS.map((opt) => (
					<button
						key={opt.value}
						type="button"
						className={`rounded-lg border p-3 text-left transition ${
							store.inputMode === opt.value
								? "border-foreground/40 bg-muted"
								: "border-border hover:border-foreground/20"
						}`}
						onClick={() => AudioDeviceStore.setInputMode(opt.value)}
					>
						<div className="text-sm font-medium">{opt.label}</div>
						<div className="text-muted-foreground mt-1 text-xs">{opt.desc}</div>
					</button>
				))}
			</div>
		</Section>

		<Section title="采集质量">
			<div className="grid gap-3 sm:grid-cols-2">
				<Field label="音质（比特率）">
					<Select
						value={String(store.audioBitrate)}
						onValueChange={(value) =>
							AudioDeviceStore.setAudioBitrate(Number(value) as AudioBitrate)
						}
					>
						<SelectTrigger>
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							{BITRATE_OPTIONS.map((o) => (
								<SelectItem key={o.value} value={String(o.value)}>
									{o.label}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				</Field>
				<Field label="采样率">
					<Select
						value={String(store.sampleRate)}
						onValueChange={(value) =>
							AudioDeviceStore.setSampleRate(Number(value) as AudioSampleRate)
						}
					>
						<SelectTrigger>
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							{SAMPLE_RATE_OPTIONS.map((o) => (
								<SelectItem key={o.value} value={String(o.value)}>
									{o.label}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				</Field>
				<Field label="位深">
					<Select
						value={String(store.sampleSize)}
						onValueChange={(value) =>
							AudioDeviceStore.setSampleSize(Number(value) as AudioSampleSize)
						}
					>
						<SelectTrigger>
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							{SAMPLE_SIZE_OPTIONS.map((o) => (
								<SelectItem key={o.value} value={String(o.value)}>
									{o.label}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				</Field>
			</div>
		</Section>

		<Section title="语音优化">
			<Toggle
				label="回声消除"
				desc="消除扬声器声音反馈到麦克风"
				checked={store.echoCancellation}
				onChange={AudioDeviceStore.setEchoCancellation}
			/>
			<Toggle
				label="噪声抑制"
				desc="过滤背景噪音"
				checked={store.noiseSuppression}
				onChange={AudioDeviceStore.setNoiseSuppression}
			/>
			<Toggle
				label="自动增益"
				desc="自动调节麦克风音量"
				checked={store.autoGainControl}
				onChange={AudioDeviceStore.setAutoGainControl}
			/>
			<Toggle
				label="人声隔离（实验性）"
				desc="比噪声抑制更强，开启后覆盖噪声抑制，浏览器支持有限"
				checked={store.voiceIsolation}
				onChange={AudioDeviceStore.setVoiceIsolation}
			/>
		</Section>
	</>
);
