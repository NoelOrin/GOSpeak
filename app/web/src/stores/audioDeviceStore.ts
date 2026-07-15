import to from "await-to-js";
import { get, set } from "idb-keyval";
import { debounce } from "lodash-es";
import { createEffect, createRoot, on } from "solid-js";
import { createStore } from "solid-js/store";
import { getAudioDevices } from "../hooks/media";

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
	// 测试用：麦克风输入增益提示（仅 UI 音量，真正输入增益仍走 AGC/系统）
}

const initialState: AudioDeviceState = {
	selectedAudioInput: "",
	selectedAudioOutput: "",
	audioinputs: [],
	audiooutputs: [],
	loaded: false,
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
};

const loadPersistedState = async (): Promise<AudioDeviceState> => {
	const [err, savedState] = await to(get<AudioDeviceState>("audioDeviceStore"));
	if (!err) {
		return savedState ? { ...initialState, ...savedState } : initialState;
	}
	throw err;
};

const [audioDeviceStore, setAudioDeviceStore] = createStore<AudioDeviceState>(
	await loadPersistedState(),
);

async function fetchAudioDevices() {
	const devices = await getAudioDevices();
	const inputExists = devices.audioinputs.some(
		(d) => d.deviceId === audioDeviceStore.selectedAudioInput,
	);
	const outputExists = devices.audiooutputs.some(
		(d) => d.deviceId === audioDeviceStore.selectedAudioOutput,
	);
	setAudioDeviceStore({
		selectedAudioInput: inputExists
			? audioDeviceStore.selectedAudioInput
			: devices.audioinputs[0]?.deviceId || "",
		selectedAudioOutput: outputExists
			? audioDeviceStore.selectedAudioOutput
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

function setAudioBitrate(bitrate: AudioBitrate) {
	setAudioDeviceStore("audioBitrate", bitrate);
}
function setSelectedAudioInput(deviceId: string) {
	setAudioDeviceStore("selectedAudioInput", deviceId);
}
function setSelectedAudioOutput(deviceId: string) {
	setAudioDeviceStore("selectedAudioOutput", deviceId);
}
function setStereo(v: boolean) {
	setAudioDeviceStore("stereo", v);
}
function setDtx(v: boolean) {
	setAudioDeviceStore("dtx", v);
}
function setRed(v: boolean) {
	setAudioDeviceStore("red", v);
}
function setSampleRate(rate: AudioSampleRate) {
	setAudioDeviceStore("sampleRate", rate);
}
function setSampleSize(size: AudioSampleSize) {
	setAudioDeviceStore("sampleSize", size);
}
function setEchoCancellation(v: boolean) {
	setAudioDeviceStore("echoCancellation", v);
}
function setNoiseSuppression(v: boolean) {
	setAudioDeviceStore("noiseSuppression", v);
}
function setAutoGainControl(v: boolean) {
	setAudioDeviceStore("autoGainControl", v);
}
function setVoiceIsolation(v: boolean) {
	setAudioDeviceStore("voiceIsolation", v);
}
function setMuteOnJoin(v: boolean) {
	setAudioDeviceStore("muteOnJoin", v);
}
function setInputMode(mode: AudioInputMode) {
	setAudioDeviceStore("inputMode", mode);
	// 音乐模式默认关闭 AEC/NS/AGC，语音模式打开
	if (mode === "music") {
		setAudioDeviceStore({
			echoCancellation: false,
			noiseSuppression: false,
			autoGainControl: false,
			voiceIsolation: false,
			stereo: true,
			dtx: false,
		});
	} else {
		setAudioDeviceStore({
			echoCancellation: true,
			noiseSuppression: true,
			autoGainControl: true,
			stereo: false,
			dtx: true,
		});
	}
}
function setNotificationSounds(v: boolean) {
	setAudioDeviceStore("notificationSounds", v);
}

function resetAudioDefaults() {
	setAudioDeviceStore({
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

createRoot(() => {
	createEffect(
		on(
			() => [
				audioDeviceStore.audioBitrate,
				audioDeviceStore.stereo,
				audioDeviceStore.dtx,
				audioDeviceStore.red,
				audioDeviceStore.sampleRate,
				audioDeviceStore.sampleSize,
				audioDeviceStore.echoCancellation,
				audioDeviceStore.noiseSuppression,
				audioDeviceStore.autoGainControl,
				audioDeviceStore.voiceIsolation,
				audioDeviceStore.selectedAudioInput,
				audioDeviceStore.selectedAudioOutput,
				audioDeviceStore.muteOnJoin,
				audioDeviceStore.inputMode,
				audioDeviceStore.notificationSounds,
			],
			() => {
				debouncedPersist(audioDeviceStore);
			},
		),
	);
});

const AudioDeviceStore = {
	state: audioDeviceStore,
	fetchAudioDevices,
	setAudioBitrate,
	setSelectedAudioInput,
	setSelectedAudioOutput,
	setStereo,
	setDtx,
	setRed,
	setSampleRate,
	setSampleSize,
	setEchoCancellation,
	setNoiseSuppression,
	setAutoGainControl,
	setVoiceIsolation,
	setMuteOnJoin,
	setInputMode,
	setNotificationSounds,
	resetAudioDefaults,
};

export default AudioDeviceStore;
