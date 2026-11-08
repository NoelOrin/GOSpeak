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

interface AudioDeviceState {
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

const initialState: AudioDeviceState = {
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

const loadPersistedState = async (): Promise<AudioDeviceState> => {
	const [err, savedState] = await to(get<AudioDeviceState>("audioDeviceStore"));
	if (!err) {
		return savedState ? { ...initialState, ...savedState } : initialState;
	}
	throw err;
};

export const useAudioDeviceStore = create<AudioDeviceState>(() => initialState);

// 顶层 await：与 solid 版语义一致，保证消费方读到的是已恢复的持久化状态
const persisted = await loadPersistedState();
useAudioDeviceStore.setState(persisted);

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

/**
 * 只同步设备列表，不改动用户已选的设备。
 * 设备变化检测需要一个“纯刷新”入口：是否切换由用户确认后决定。
 */
function syncDeviceList(devices: {
	audioinputs: { deviceId: string; label: string }[];
	audiooutputs: { deviceId: string; label: string }[];
}) {
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
}

const debouncedPersist = debounce((state: AudioDeviceState) => {
	try {
		const cleanState = JSON.parse(JSON.stringify(state));
		// 设备列表不持久化（会过期），只存选择与偏好
		delete (cleanState as Partial<AudioDeviceState>).audioinputs;
		delete (cleanState as Partial<AudioDeviceState>).audiooutputs;
		delete (cleanState as Partial<AudioDeviceState>).loaded;
		set("audioDeviceStore", cleanState);
	} catch (error) {
		console.error("Failed to serialize audio device state:", error);
	}
}, 200);

useAudioDeviceStore.subscribe((state) => {
	debouncedPersist(state);
});

const AudioDeviceStore = {
	get state() {
		return useAudioDeviceStore.getState();
	},
	fetchAudioDevices,
	syncDeviceList,
	setAudioBitrate: (audioBitrate: AudioBitrate) =>
		useAudioDeviceStore.setState({ audioBitrate }),
	setSelectedAudioInput: (selectedAudioInput: string) =>
		useAudioDeviceStore.setState({ selectedAudioInput }),
	/**
	 * 只在用户还没选过设备时，用启动授权解析出的真实设备补上默认选择。
	 * 空值代表「跟随系统默认」，但那样无法跟踪具体设备，设备变化就提示不出来。
	 */
	ensureSelectedAudioInput: (deviceId: string) => {
		if (!deviceId) return;
		const { selectedAudioInput } = useAudioDeviceStore.getState();
		if (selectedAudioInput) return;
		useAudioDeviceStore.setState({ selectedAudioInput: deviceId });
	},
	setSelectedAudioOutput: (selectedAudioOutput: string) =>
		useAudioDeviceStore.setState({ selectedAudioOutput }),
	setStereo: (stereo: boolean) => useAudioDeviceStore.setState({ stereo }),
	setDtx: (dtx: boolean) => useAudioDeviceStore.setState({ dtx }),
	setRed: (red: boolean) => useAudioDeviceStore.setState({ red }),
	setSampleRate: (sampleRate: AudioSampleRate) =>
		useAudioDeviceStore.setState({ sampleRate }),
	setSampleSize: (sampleSize: AudioSampleSize) =>
		useAudioDeviceStore.setState({ sampleSize }),
	setEchoCancellation: (echoCancellation: boolean) =>
		useAudioDeviceStore.setState({ echoCancellation }),
	setNoiseSuppression: (noiseSuppression: boolean) =>
		useAudioDeviceStore.setState({ noiseSuppression }),
	setAutoGainControl: (autoGainControl: boolean) =>
		useAudioDeviceStore.setState({ autoGainControl }),
	setVoiceIsolation: (voiceIsolation: boolean) =>
		useAudioDeviceStore.setState({ voiceIsolation }),
	setMuteOnJoin: (muteOnJoin: boolean) =>
		useAudioDeviceStore.setState({ muteOnJoin }),
	setDeviceChangeNotice: (deviceChangeNotice: boolean) =>
		useAudioDeviceStore.setState({ deviceChangeNotice }),
	setInputMode: (inputMode: AudioInputMode) => {
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
	setNotificationSounds: (notificationSounds: boolean) =>
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
};

export default AudioDeviceStore;
