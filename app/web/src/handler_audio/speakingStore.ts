import { useSocketStore } from "@/stores/socketStore";
import type { SocketState } from "@/stores/socketStore";

/**
 * 发言高亮状态的事实源收编在 socketStore（signal 聚合 + SFU 原生事件双源写入）。
 * 这里只保留对外读取 API：组件 hook（selector / getState / subscribe）与命令式门面。
 * 注意：socketStore → socketEvents → handler_audio → 本模块存在模块环引用，
 * 因此所有 useSocketStore 的解引用都放在调用期闭包里，不在模块求值期发生。
 */
export function selectSpeakingIdentities(state: SocketState): string[] {
	return state.speakingIdentities;
}

type UseSpeakingStore = {
	(selector?: (state: SocketState) => string[]): string[];
	getState(): string[];
	subscribe(
		listener: (state: SocketState, prev: SocketState) => void,
	): () => void;
};

export const useSpeakingStore: UseSpeakingStore = Object.assign(
	(selector = selectSpeakingIdentities) => useSocketStore(selector),
	{
		getState: () => useSocketStore.getState().speakingIdentities,
		subscribe: (listener: (state: SocketState, prev: SocketState) => void) =>
			useSocketStore.subscribe(listener),
	},
);

export const speakingStore = {
	get: (): string[] => useSocketStore.getState().speakingIdentities,
	set: (identities: string[]) =>
		useSocketStore.setState({ speakingIdentities: identities }),
};
