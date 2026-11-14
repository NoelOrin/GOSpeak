import { create } from "zustand";

interface SpeakingState {
	/** 当前正在发言的成员 identity 列表 */
	speakingIdentities: string[];
	setSpeakingIdentities: (identities: string[]) => void;
}

export const useSpeakingStore = create<SpeakingState>((set) => ({
	speakingIdentities: [],
	setSpeakingIdentities: (speakingIdentities) => set({ speakingIdentities }),
}));

export const speakingStore = {
	get: () => useSpeakingStore.getState().speakingIdentities,
	set: (identities: string[]) =>
		useSpeakingStore.getState().setSpeakingIdentities(identities),
};
