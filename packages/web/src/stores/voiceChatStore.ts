import to from "await-to-js";
import { get, set } from "idb-keyval";
import { debounce } from "lodash-es";
import { createEffect, on } from "solid-js";
import { createStore } from "solid-js/store";

type MemberVoiceChatState = {
	outputVolume: number;
	isMute: boolean;
};

type OtherMemberStateType = {
	[key: string]: MemberVoiceChatState;
};

type SelfVoiceChatState = {
	isInputMute: boolean; // 音频输入是否静音
	isOutMute: boolean; // 音频输出是否静音
	isVideoMute: boolean; // 视频输出是否静音
	inputVolume: number; // 音频输入音量 (0-100)
	outputVolume: number; // 音频输出音量 (0-100)
	videoVolume: number; // 视频输出音量 (0-100)

	otherMemberState: OtherMemberStateType[];
};

// Load persisted state from IndexedDB or use defaults
const initialState: SelfVoiceChatState = {
	isInputMute: false,
	isOutMute: false,
	isVideoMute: false,
	inputVolume: 100,
	outputVolume: 100,
	videoVolume: 100,
	otherMemberState: [],
};

const loadPersistedState = async (): Promise<SelfVoiceChatState> => {
	const [err, savedState] = await to(get<SelfVoiceChatState>("voiceChatStore"));
	if (!err) {
		return savedState ? { ...initialState, ...savedState } : initialState;
	}
	throw err;
};

const [store, setStore] = createStore<SelfVoiceChatState>(
	await loadPersistedState(),
);

const voiceChatActions = {
	// 设置音频输入是否静音
	setIsInputMute(isMute: boolean) {
		setStore("isInputMute", isMute);
	},
	// 设置音频输出是否静音
	setIsOutMute(isOutMute: boolean) {
		setStore("isOutMute", isOutMute);
	},
	// 设置视频输出是否静音
	setIsVideoMute(isVideoMute: boolean) {
		setStore("isVideoMute", isVideoMute);
	},

	// 设置音频输入音量
	setInputVolume(inputVolume: number) {
		setStore("inputVolume", inputVolume);
	},
	// 设置音频输出音量
	setOutputVolume(outputVolume: number) {
		setStore("outputVolume", outputVolume);
	},
	// 设置视频输出音量
	setVideoVolume(videoVolume: number) {
		setStore("videoVolume", videoVolume);
	},
};

const debouncedPersist = debounce((state) => {
	try {
		const cleanState = JSON.parse(JSON.stringify(state));
		// console.log("Persisting state to DB:", cleanState);
		set("voiceChatStore", cleanState);
	} catch (error) {
		console.error("Failed to serialize voice chat state:", error);
	}
}, 200);

// 监听&防抖持久化
createEffect(
	on(
		() => [
			store.isInputMute,
			store.isOutMute,
			store.isVideoMute,
			store.inputVolume,
			store.outputVolume,
			store.videoVolume,
		],
		() => {
			debouncedPersist(store);
		},
	),
);

const VoiceChatStore = {
	data: store,
	...voiceChatActions,
};
export default VoiceChatStore;
