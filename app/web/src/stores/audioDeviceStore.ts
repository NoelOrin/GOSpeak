import to from "await-to-js";
import { get, set } from "idb-keyval";
import { debounce } from "lodash-es";
import { create } from "zustand";
import { toast } from "@gospeak/components";
import { getAudioDevices } from "../hooks/media";
import { localizedApiMessage } from "@/utils/errorMessages";

interface AudioDeviceInfo {
	deviceId: string;
	label: string;
}

export type AudioBitrate = 32000 | 48000 | 64000 | 96000 | 128000;
export type AudioSampleRate = 44100 | 48000;
export type AudioSampleSize = 8 | 16 | 24 | 32;
export type AudioInputMode = "voice" | "music";

interface AudioDeviceData {
	selectedAudioInput: string;
	selectedAudioOutput: string;
	audioinputs: AudioDeviceInfo[];
	audiooutputs: AudioDeviceInfo[];
	loaded: boolean;
	/** 设备枚举失败的原因：以前直接抛成未处理拒绝，界面上列表毫无变化。 */
	deviceLoadError: string | null;
	// 发布层
	audioBitrate: AudioBitrate;
	stereo: boolean;
	dtx: boolean;
	red: boolean;
	// 采集层
	sampleRate: AudioSampleRate;
	sampleSize: AudioSampleSize;
	echoCancellation: boolean;
	noiseSuppression: boolean;
	autoGainControl: boolean;
	voiceIsolation: boolean;
	// 语音行为
	muteOnJoin: boolean;
	inputMode: AudioInputMode;
	// 音效 / 输出
	notificationSounds: boolean;
	// 设备变化时是否弹窗询问是否切换（“以后不再显示”写这里，设置页可改回）
	deviceChangeNotice: boolean;
}

const initialState: AudioDeviceData = {
	selectedAudioInput: "",
	selectedAudioOutput: "",
	audioinputs: [],
	audiooutputs: [],
	loaded: false,
	deviceLoadError: null,
	audioBitrate: 64000,
	stereo: false,
	dtx: true,
	red: true,
	sampleRate: 48000,
	sampleSize: 16,
	echoCancellation: true,
	noiseSuppression: true,
	autoGainControl: true,
	voiceIsolation: false,
	muteOnJoin: false,
	inputMode: "voice",
	notificationSounds: true,
	deviceChangeNotice: true,
};

const loadPersistedState = async (): Promise<AudioDeviceData> => {
	const [err, savedState] = await to(get<AudioDeviceData>("audioDeviceStore"));
	if (!err) {
		return savedState ? { ...initialState, ...savedState } : initialState;
	}
	throw err;
};

export interface AudioDeviceStoreState extends AudioDeviceData {
	fetchAudioDevices(options?: { notify?: boolean }): Promise<void>;
	/** 只同步设备列表，不改动用户已选的设备（设备变化检测的"纯刷新"入口） */
	syncDeviceList(devices: {
		audioinputs: { deviceId: string; label: string }[];
		audiooutputs: { deviceId: string; label: string }[];
	}): void;
	setAudioBitrate(audioBitrate: AudioBitrate): void;
	setSelectedAudioInput(selectedAudioInput: string): void;
	/** 只在用户还没选过设备时补默认选择；空值代表「跟随系统默认」 */
	ensureSelectedAudioInput(deviceId: string): void;
	setSelectedAudioOutput(selectedAudioOutput: string): void;
	setStereo(stereo: boolean): void;
	setDtx(dtx: boolean): void;
	setRed(red: boolean): void;
	setSampleRate(sampleRate: AudioSampleRate): void;
	setSampleSize(sampleSize: AudioSampleSize): void;
	setEchoCancellation(echoCancellation: boolean): void;
	setNoiseSuppression(noiseSuppression: boolean): void;
	setAutoGainControl(autoGainControl: boolean): void;
	setVoiceIsolation(voiceIsolation: boolean): void;
	setMuteOnJoin(muteOnJoin: boolean): void;
	setDeviceChangeNotice(deviceChangeNotice: boolean): void;
	setInputMode(inputMode: AudioInputMode): void;
	setNotificationSounds(notificationSounds: boolean): void;
	resetAudioDefaults(): void;
}

async function fetchAudioDevices(options?: { notify?: boolean }) {
	let devices: Awaited<ReturnType<typeof getAudioDevices>>;
	try {
		devices = await getAudioDevices();
	} catch (error) {
		// 权限问题与设备故障要分开说：前者用户去系统设置放行即可，
		// 后者才是"设备读取失败"。混成一句话会把人引到错误的方向。
		const message =
			(error as { name?: string } | null)?.name === "NotAllowedError"
				? "麦克风权限被拒绝，请在系统设置中允许 GOSpeak 使用麦克风后重试"
				: localizedApiMessage(error, "读取音频设备失败，请检查麦克风权限");
		useAudioDeviceStore.setState({ deviceLoadError: message });
		// 挂载探测只在页面内联展示；用户主动点「刷新设备」再弹一条 toast。
		// apiClient 拦截器弹过的错误带 __toastShown，避免重复。
		if (
			options?.notify &&
			!(error as { __toastShown?: boolean } | null)?.__toastShown
		) {
			toast.error(message);
		}
		return;
	}
	useAudioDeviceStore.setState({ deviceLoadError: null });
	const state = useAudioDeviceStore.getState();
	const inputExists = devices.audioinputs.some(
		(d) => d.deviceId === state.selectedAudioInput,
	);
	const outputExists = devices.audiooutputs.some(
		(d) => d.deviceId === state.selectedAudioOutput,
	);
	useAudioDeviceStore.setState({
		selectedAudioInput: inputExists
			? state.selectedAudioInput
			: devices.audioinputs[0]?.deviceId || "",
		selectedAudioOutput: outputExists
			? state.selectedAudioOutput
			: devices.audiooutputs[0]?.deviceId || "",
		audioinputs: devices.audioinputs.map((d) => ({
			deviceId: d.deviceId,
			label: d.label,
		})),
		audiooutputs: devices.audiooutputs.map((d) => ({
			deviceId: d.deviceId,
			label: d.label,
		})),
		loaded: true,
	});
}

const debouncedPersist = debounce((state: AudioDeviceData) => {
	try {
		const cleanState = JSON.parse(JSON.stringify(state));
		// 设备列表不持久化（会过期），只存选择与偏好
		delete (cleanState as Partial<AudioDeviceData>).audioinputs;
		delete (cleanState as Partial<AudioDeviceData>).audiooutputs;
		delete (cleanState as Partial<AudioDeviceData>).loaded;
		set("audioDeviceStore", cleanState);
	} catch (error) {
		console.error("Failed to serialize audio device state:", error);
	}
}, 200);

export const useAudioDeviceStore = create<AudioDeviceStoreState>(() => ({
	...initialState,
	fetchAudioDevices,
	syncDeviceList: (devices) => {
		useAudioDeviceStore.setState({
			audioinputs: devices.audioinputs.map((d) => ({
				deviceId: d.deviceId,
				label: d.label,
			})),
			audiooutputs: devices.audiooutputs.map((d) => ({
				deviceId: d.deviceId,
				label: d.label,
			})),
			loaded: true,
		});
	},
	setAudioBitrate: (audioBitrate) =>
		useAudioDeviceStore.setState({ audioBitrate }),
	setSelectedAudioInput: (selectedAudioInput) =>
		useAudioDeviceStore.setState({ selectedAudioInput }),
	ensureSelectedAudioInput: (deviceId) => {
		if (!deviceId) return;
		const { selectedAudioInput } = useAudioDeviceStore.getState();
		if (selectedAudioInput) return;
		useAudioDeviceStore.setState({ selectedAudioInput: deviceId });
	},
	setSelectedAudioOutput: (selectedAudioOutput) =>
		useAudioDeviceStore.setState({ selectedAudioOutput }),
	setStereo: (stereo) => useAudioDeviceStore.setState({ stereo }),
	setDtx: (dtx) => useAudioDeviceStore.setState({ dtx }),
	setRed: (red) => useAudioDeviceStore.setState({ red }),
	setSampleRate: (sampleRate) => useAudioDeviceStore.setState({ sampleRate }),
	setSampleSize: (sampleSize) => useAudioDeviceStore.setState({ sampleSize }),
	setEchoCancellation: (echoCancellation) =>
		useAudioDeviceStore.setState({ echoCancellation }),
	setNoiseSuppression: (noiseSuppression) =>
		useAudioDeviceStore.setState({ noiseSuppression }),
	setAutoGainControl: (autoGainControl) =>
		useAudioDeviceStore.setState({ autoGainControl }),
	setVoiceIsolation: (voiceIsolation) =>
		useAudioDeviceStore.setState({ voiceIsolation }),
	setMuteOnJoin: (muteOnJoin) => useAudioDeviceStore.setState({ muteOnJoin }),
	setDeviceChangeNotice: (deviceChangeNotice) =>
		useAudioDeviceStore.setState({ deviceChangeNotice }),
	setInputMode: (inputMode) => {
		useAudioDeviceStore.setState({ inputMode });
		// 音乐模式默认关闭 AEC/NS/AGC，语音模式打开
		if (inputMode === "music") {
			useAudioDeviceStore.setState({
				echoCancellation: false,
				noiseSuppression: false,
				autoGainControl: false,
				voiceIsolation: false,
				stereo: true,
				dtx: false,
			});
		} else {
			useAudioDeviceStore.setState({
				echoCancellation: true,
				noiseSuppression: true,
				autoGainControl: true,
				stereo: false,
				dtx: true,
			});
		}
	},
	setNotificationSounds: (notificationSounds) =>
		useAudioDeviceStore.setState({ notificationSounds }),
	resetAudioDefaults: () => {
		useAudioDeviceStore.setState({
			audioBitrate: initialState.audioBitrate,
			stereo: initialState.stereo,
			dtx: initialState.dtx,
			red: initialState.red,
			sampleRate: initialState.sampleRate,
			sampleSize: initialState.sampleSize,
			echoCancellation: initialState.echoCancellation,
			noiseSuppression: initialState.noiseSuppression,
			autoGainControl: initialState.autoGainControl,
			voiceIsolation: initialState.voiceIsolation,
			muteOnJoin: initialState.muteOnJoin,
			inputMode: initialState.inputMode,
			notificationSounds: initialState.notificationSounds,
			deviceChangeNotice: initialState.deviceChangeNotice,
		});
	},
}));

// 顶层 await：与 solid 版语义一致，保证消费方读到的是已恢复的持久化状态
const persisted = await loadPersistedState();
useAudioDeviceStore.setState(persisted);

useAudioDeviceStore.subscribe((state) => {
	debouncedPersist(state);
});
