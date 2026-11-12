import to from "await-to-js";
import { get, set } from "idb-keyval";
import { debounce } from "lodash-es";
import { create } from "zustand";

type MemberVoiceChatState = {
	outputVolume: number;
	isMute: boolean;
};

type OtherMemberStateType = Record<string, MemberVoiceChatState>;

type SelfVoiceChatState = {
	isInputMute: boolean; // 音频输入是否静音
	isOutMute: boolean; // 音频输出是否静音
	isVideoMute: boolean; // 视频输出是否静音
	inputVolume: number; // 音频输入音量 (0-100)
	outputVolume: number; // 音频输出音量 (0-100)
	videoVolume: number; // 视频输出音量 (0-100)
	otherMemberState: OtherMemberStateType;
};

// Load persisted state from IndexedDB or use defaults
const initialState: SelfVoiceChatState = {
	isInputMute: false,
	isOutMute: false,
	isVideoMute: false,
	inputVolume: 100,
	outputVolume: 100,
	videoVolume: 100,
	otherMemberState: {},
};

function normalizeOtherMemberState(
	value: SelfVoiceChatState["otherMemberState"],
): OtherMemberStateType {
	if (Array.isArray(value)) {
		return value.reduce<OtherMemberStateType>((acc, entry) => {
			if (!entry || typeof entry !== "object") return acc;
			for (const [identity, rawState] of Object.entries(entry)) {
				const state = rawState as Partial<MemberVoiceChatState> | undefined;
				acc[identity] = {
					outputVolume: state?.outputVolume ?? 100,
					isMute: state?.isMute ?? false,
				};
			}
			return acc;
		}, {});
	}
	return (value || {}) as OtherMemberStateType;
}

const loadPersistedState = async (): Promise<SelfVoiceChatState> => {
	const [err, savedState] = await to(get<SelfVoiceChatState>("voiceChatStore"));
	if (!err) {
		return savedState ? { ...initialState, ...savedState } : initialState;
	}
	throw err;
};

export const useVoiceChatStore = create<SelfVoiceChatState>(() => initialState);

// 用户是否已主动修改过状态；若持久化加载完成前用户已操作，不再用旧值覆盖。
let userModified = false;

function markModified<T extends (...args: any[]) => unknown>(fn: T): T {
	return ((...args: any[]) => {
		userModified = true;
		return fn(...args);
	}) as T;
}

void loadPersistedState()
	.then((state) => {
		if (userModified) return;
		useVoiceChatStore.setState({
			...state,
			otherMemberState: normalizeOtherMemberState(state.otherMemberState),
		});
	})
	.catch((error) => {
		console.error("Failed to load persisted voice chat state:", error);
	});

const voiceChatActions = {
	// 设置音频输入是否静音
	setIsInputMute: markModified((isInputMute: boolean) => {
		useVoiceChatStore.setState({ isInputMute });
	}),
	// 设置音频输出是否静音
	setIsOutMute: markModified((isOutMute: boolean) => {
		useVoiceChatStore.setState({ isOutMute });
	}),
	// 设置视频输出是否静音
	setIsVideoMute: markModified((isVideoMute: boolean) => {
		useVoiceChatStore.setState({ isVideoMute });
	}),

	// 设置音频输入音量
	setInputVolume: markModified((inputVolume: number) => {
		useVoiceChatStore.setState({ inputVolume });
	}),
	// 设置音频输出音量
	setOutputVolume: markModified((outputVolume: number) => {
		useVoiceChatStore.setState({ outputVolume });
	}),
	// 设置视频输出音量
	setVideoVolume: markModified((videoVolume: number) => {
		useVoiceChatStore.setState({ videoVolume });
	}),
	setMemberOutputVolume: markModified(
		(identity: string, outputVolume: number) => {
			useVoiceChatStore.setState((prev) => ({
				otherMemberState: {
					...prev.otherMemberState,
					[identity]: {
						outputVolume,
						isMute: prev.otherMemberState[identity]?.isMute ?? false,
					},
				},
			}));
		},
	),
	setMemberMute: markModified((identity: string, isMute: boolean) => {
		useVoiceChatStore.setState((prev) => ({
			otherMemberState: {
				...prev.otherMemberState,
				[identity]: {
					outputVolume: prev.otherMemberState[identity]?.outputVolume ?? 100,
					isMute,
				},
			},
		}));
	}),
	memberState(identity: string): MemberVoiceChatState {
		return (
			useVoiceChatStore.getState().otherMemberState[identity] || {
				outputVolume: 100,
				isMute: false,
			}
		);
	},
};

const debouncedPersist = debounce((state: SelfVoiceChatState) => {
	try {
		const cleanState = JSON.parse(JSON.stringify(state));
		set("voiceChatStore", cleanState);
	} catch (error) {
		console.error("Failed to serialize voice chat state:", error);
	}
}, 200);

// 监听&防抖持久化
useVoiceChatStore.subscribe((state) => {
	debouncedPersist(state);
});

export const useVoiceChatActions = () => voiceChatActions;

const VoiceChatStore = {
	get data() {
		return useVoiceChatStore.getState();
	},
	...voiceChatActions,
};
export default VoiceChatStore;
