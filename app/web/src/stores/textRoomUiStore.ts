import { create } from "zustand";
import { searchMessages } from "@/api/message";
import { useChatStore } from "@/stores/chatStore";
import type { MessageDTO } from "@/types/message";

/**
 * 文字频道的视图状态。
 *
 * Discord 把搜索框放在频道标题栏，而结果呈现在下方消息区，两者分属不同组件，
 * 因此搜索 / 线程 / 回复状态放在共享 store 里，避免 props 穿透。
 */
interface TextRoomUiState {
	query: string;
	searchActive: boolean;
	searchResults: MessageDTO[];
	searching: boolean;
	replyTo: string | null;
	threadParent: string | null;
	setQuery: (query: string) => void;
	runSearch: () => Promise<void>;
	clearSearch: () => void;
	setReplyTo: (uuid: string | null) => void;
	openThread: (uuid: string) => void;
	exitThread: () => void;
	resetForRoom: () => void;
}

const EMPTY = {
	query: "",
	searchActive: false,
	searchResults: [] as MessageDTO[],
	searching: false,
	replyTo: null,
	threadParent: null,
};

export const useTextRoomUiStore = create<TextRoomUiState>((set, get) => ({
	...EMPTY,
	setQuery: (query) => set({ query }),
	runSearch: async () => {
		const q = get().query.trim();
		const room = useChatStore.getState().textRoom;
		if (!room || !q) {
			set({ searchActive: false, searchResults: [] });
			return;
		}
		set({ searching: true });
		try {
			const results = await searchMessages(room, q);
			// 请求期间用户又改了关键词，丢弃过期结果
			if (get().query.trim() !== q) return;
			set({ searchResults: results, searchActive: true, searching: false });
		} catch {
			set({ searchResults: [], searchActive: true, searching: false });
		}
	},
	clearSearch: () => set({ query: "", searchActive: false, searchResults: [] }),
	setReplyTo: (uuid) => set({ replyTo: uuid }),
	openThread: (uuid) =>
		set({ threadParent: uuid, replyTo: null, searchActive: false }),
	exitThread: () => set({ threadParent: null, replyTo: null }),
	resetForRoom: () => set({ ...EMPTY }),
}));
