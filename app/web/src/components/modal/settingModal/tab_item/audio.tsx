import {
	Button,
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
	toast,
} from "@gospeak/components";
import {
	normalizeScreenShareQuality,
	SCREEN_SHARE_QUALITY_ORDER,
	SCREEN_SHARE_QUALITY_PRESETS,
} from "@gospeak/sfu-client";
import { useEffect, useRef, useState } from "react";
import AudioDeviceStore, {
	useAudioDeviceStore,
	type AudioBitrate,
	type AudioInputMode,
	type AudioSampleRate,
	type AudioSampleSize,
} from "@/stores/audioDeviceStore";
import ScreenShareSettingsStore, {
	useScreenShareSettingsStore,
} from "@/stores/screenShareSettingsStore";
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

// Radix Select 不接受空字符串 value，空设备 id 用哨兵值映射
const DEFAULT_DEVICE = "__default__";

function AudioForm() {
	const [testingMic, setTestingMic] = useState(false);
	const [micLevel, setMicLevel] = useState(0);
	const testStreamRef = useRef<MediaStream | null>(null);
	const analyserRef = useRef<AnalyserNode | null>(null);
	const audioCtxRef = useRef<AudioContext | null>(null);
	const rafRef = useRef(0);

	const store = useAudioDeviceStore();
	const screenShareQuality = useScreenShareSettingsStore((s) => s.quality);

	function stopMicTest() {
		if (rafRef.current) cancelAnimationFrame(rafRef.current);
		rafRef.current = 0;
		testStreamRef.current?.getTracks().forEach((t) => {
			t.stop();
		});
		testStreamRef.current = null;
		void audioCtxRef.current?.close();
		audioCtxRef.current = null;
		analyserRef.current = null;
		setMicLevel(0);
		setTestingMic(false);
	}

	async function startMicTest() {
		if (testingMic) {
			stopMicTest();
			return;
		}
		try {
			const constraints: MediaStreamConstraints = {
				audio: {
					deviceId: store.selectedAudioInput
						? { exact: store.selectedAudioInput }
						: undefined,
					echoCancellation: store.echoCancellation,
					noiseSuppression: store.noiseSuppression,
					autoGainControl: store.autoGainControl,
				},
				video: false,
			};
			testStreamRef.current =
				await navigator.mediaDevices.getUserMedia(constraints);
			audioCtxRef.current = new AudioContext();
			const source = audioCtxRef.current.createMediaStreamSource(
				testStreamRef.current,
			);
			analyserRef.current = audioCtxRef.current.createAnalyser();
			analyserRef.current.fftSize = 256;
			source.connect(analyserRef.current);
			const data = new Uint8Array(analyserRef.current.frequencyBinCount);
			const tick = () => {
				const analyser = analyserRef.current;
				if (!analyser) return;
				analyser.getByteTimeDomainData(data);
				let sum = 0;
				for (let i = 0; i < data.length; i++) {
					const v = (data[i] - 128) / 128;
					sum += v * v;
				}
				const rms = Math.sqrt(sum / data.length);
				setMicLevel(Math.min(100, Math.round(rms * 220)));
				rafRef.current = requestAnimationFrame(tick);
			};
			setTestingMic(true);
			tick();
		} catch (error) {
			toast.error(
				error instanceof Error && error.name === "NotAllowedError"
					? "麦克风权限被拒绝，请在浏览器设置中允许后重试"
					: "麦克风测试失败，请检查设备是否可用",
			);
			stopMicTest();
		}
	}

	useEffect(() => {
		void AudioDeviceStore.fetchAudioDevices();
		return () => {
			stopMicTest();
		};
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, []);

	return (
		<Page
			title="音频"
			desc="设备、采集质量与传输参数。设备可在房间内热切换，采集/传输参数需重新进房生效。"
		>
			<Section
				title="设备"
				action={
					<button
						type="button"
						className="hover:bg-muted inline-flex h-7 cursor-pointer items-center rounded-sm px-2 text-xs transition-colors"
						onClick={() =>
							void AudioDeviceStore.fetchAudioDevices({ notify: true })
						}
					>
						刷新设备
					</button>
				}
			>
				{store.deviceLoadError && (
					<div
						role="alert"
						className="border-destructive/40 bg-destructive/10 text-destructive flex flex-wrap items-center gap-2 rounded-sm border px-3 py-2 text-xs"
					>
						<span>{store.deviceLoadError}</span>
						<button
							type="button"
							className="underline underline-offset-2"
							onClick={() =>
								void AudioDeviceStore.fetchAudioDevices({ notify: true })
							}
						>
							重试
						</button>
					</div>
				)}
				<Field label="输入设备（麦克风）">
					<Select
						value={store.selectedAudioInput || DEFAULT_DEVICE}
						onValueChange={(value) =>
							AudioDeviceStore.setSelectedAudioInput(
								value === DEFAULT_DEVICE ? "" : value,
							)
						}
					>
						<SelectTrigger>
							<SelectValue placeholder="选择输入设备" />
						</SelectTrigger>
						<SelectContent>
							{store.audioinputs.map((d) => (
								<SelectItem
									key={d.deviceId || DEFAULT_DEVICE}
									value={d.deviceId || DEFAULT_DEVICE}
								>
									{d.label || `麦克风 (${d.deviceId.slice(0, 8)})`}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				</Field>

				<Field
					label="输出设备（扬声器）"
					hint="部分浏览器仅支持 setSinkId；不支持时将使用系统默认输出。"
				>
					<Select
						value={store.selectedAudioOutput || DEFAULT_DEVICE}
						onValueChange={(value) =>
							AudioDeviceStore.setSelectedAudioOutput(
								value === DEFAULT_DEVICE ? "" : value,
							)
						}
					>
						<SelectTrigger>
							<SelectValue placeholder="选择输出设备" />
						</SelectTrigger>
						<SelectContent>
							{store.audiooutputs.map((d) => (
								<SelectItem
									key={d.deviceId || DEFAULT_DEVICE}
									value={d.deviceId || DEFAULT_DEVICE}
								>
									{d.label || `扬声器 (${d.deviceId.slice(0, 8)})`}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				</Field>

				<Toggle
					label="设备变化时提醒"
					desc="插入或拔出麦克风/扬声器时弹窗询问是否切换；关闭后静默沿用当前设备"
					checked={store.deviceChangeNotice}
					onChange={AudioDeviceStore.setDeviceChangeNotice}
				/>

				<div className="border-border bg-muted/40 rounded-lg border p-3">
					<div className="mb-2 flex items-center justify-between gap-3">
						<div>
							<div className="text-sm font-medium">麦克风测试</div>
							<div className="text-faint-foreground text-xs">
								本地电平检测，不会推流
							</div>
						</div>
						<button
							type="button"
							className={`inline-flex h-9 cursor-pointer items-center rounded-sm border px-3 text-sm transition-colors ${testingMic ? "border-destructive/50 text-destructive hover:bg-destructive/10" : "border-border hover:bg-muted"}`}
							onClick={() => void startMicTest()}
						>
							{testingMic ? "停止" : "开始测试"}
						</button>
					</div>
					<div className="bg-border h-2 w-full overflow-hidden rounded-full">
						<div
							className="bg-success h-full transition-[width] duration-75"
							style={{ width: `${micLevel}%` }}
						/>
					</div>
				</div>
			</Section>

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
							<div className="text-muted-foreground mt-1 text-xs">
								{opt.desc}
							</div>
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

			<Section title="网络传输优化">
				<Toggle
					label="立体声"
					desc="以双声道发布音频"
					checked={store.stereo}
					onChange={AudioDeviceStore.setStereo}
				/>
				<Toggle
					label="DTX 不连续传输"
					desc="静音时停止发包，节省带宽"
					checked={store.dtx}
					onChange={AudioDeviceStore.setDtx}
				/>
				<Toggle
					label="RED 冗余音频"
					desc="发送冗余数据抗丢包，轻微增加带宽"
					checked={store.red}
					onChange={AudioDeviceStore.setRed}
				/>
			</Section>

			<Section
				title="屏幕共享"
				desc="改档即时生效，影响他人看到的清晰度与上传带宽。"
			>
				<Field
					label="共享画质"
					hint="共享过程中改档不会中断共享，也不会重新弹出窗口选择。"
				>
					<Select
						value={screenShareQuality}
						onValueChange={(value) =>
							ScreenShareSettingsStore.setQuality(
								normalizeScreenShareQuality(value),
							)
						}
					>
						<SelectTrigger>
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							{SCREEN_SHARE_QUALITY_ORDER.map((item) => (
								<SelectItem key={item} value={item}>
									{SCREEN_SHARE_QUALITY_PRESETS[item].label}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				</Field>
			</Section>

			<div className="pt-2">
				<Button
					type="button"
					variant="outline"
					onClick={() => AudioDeviceStore.resetAudioDefaults()}
				>
					恢复音频默认值
				</Button>
			</div>

			{store.inputMode === "music" && (
				<div className="border-info/30 bg-info/10 text-info flex items-center rounded-sm px-3 py-2 text-sm">
					音乐模式已关闭常见语音处理；若出现回声，请改回「语音通话」。
				</div>
			)}
		</Page>
	);
}

const audio: SettingTabConfig = {
	id: "audio",
	label: "音频",
	icon: "mic",
	component: AudioForm,
};

export default audio;
