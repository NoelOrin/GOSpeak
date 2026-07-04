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

export type AudioBitrate = 32000 | 48000 | 64000 | 96000;
export type AudioSampleRate = 44100 | 48000;
export type AudioSampleSize = 8 | 16 | 24 | 32;

interface AudioDeviceState {
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
}

const initialState: AudioDeviceState = {
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
	setAudioDeviceStore({
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

const debouncedPersist = debounce((state: AudioDeviceState) => {
	try {
		const cleanState = JSON.parse(JSON.stringify(state));
		set("audioDeviceStore", cleanState);
	} catch (error) {
		console.error("Failed to serialize audio device state:", error);
	}
}, 200);

createRoot(() => {
	createEffect(
		on(
			() => [
				audioDeviceStore.audioinputs,
				audioDeviceStore.audiooutputs,
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
	setStereo,
	setDtx,
	setRed,
	setSampleRate,
	setSampleSize,
	setEchoCancellation,
	setNoiseSuppression,
	setAutoGainControl,
	setVoiceIsolation,
};

export default AudioDeviceStore;
