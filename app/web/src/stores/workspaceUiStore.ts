import { create } from "zustand";

/**
 * 域工作区的「视图」状态。
 *
 * 桌面端主区是上下分栏（参考 Oopz）：上方语音舞台，下方文字聊天。
 * 聊天区是主区的独立兄弟区块，不嵌进任何语音房间视图，切换语音频道
 * 只换上方内容，聊天区与其高度保持不变。
 * 语音连接由 VoiceSessionProvider 常驻持有，切频道不会断开媒体会话。
 */
export type WorkspaceStage = "voice" | "text";

const MEMBERS_PREF_KEY = "workspaceMembersOpen";
const CHAT_HEIGHT_KEY = "workspaceChatHeight";

/** 聊天区最小高度（px）：保证消息列表 + 输入框可用 */
export const MIN_CHAT_HEIGHT = 140;
/** 语音区最小高度（px）：保证成员卡片网格不被压扁 */
export const MIN_VOICE_HEIGHT = 200;
export const DEFAULT_CHAT_HEIGHT = 320;

/**
 * 把聊天区高度限制在「聊天够用、语音不被压死」的区间内。
 * containerHeight <= 0（尚未挂载/隐藏）时原样返回。
 */
export function clampChatHeight(
	height: number,
	containerHeight: number,
): number {
	const value = Number.isFinite(height) ? height : DEFAULT_CHAT_HEIGHT;
	if (containerHeight <= 0) return Math.max(MIN_CHAT_HEIGHT, value);
	// 容器本身太矮时，最多各占一半，避免上下限互相打架
	const max =
		containerHeight <= MIN_CHAT_HEIGHT + MIN_VOICE_HEIGHT
			? Math.max(MIN_CHAT_HEIGHT, containerHeight / 2)
			: containerHeight - MIN_VOICE_HEIGHT;
	return Math.min(Math.max(value, MIN_CHAT_HEIGHT), max);
}

function loadMembersOpen(): boolean {
	try {
		return localStorage.getItem(MEMBERS_PREF_KEY) !== "false";
	} catch {
		return true;
	}
}

function loadChatHeight(): number {
	try {
		const saved = Number.parseInt(
			localStorage.getItem(CHAT_HEIGHT_KEY) ?? "",
			10,
		);
		return Number.isFinite(saved) ? saved : DEFAULT_CHAT_HEIGHT;
	} catch {
		return DEFAULT_CHAT_HEIGHT;
	}
}

interface WorkspaceUiState {
	/** 当前聚焦的频道类型（移动端 tab / 语音状态条） */
	stage: WorkspaceStage;
	/** 下方文字聊天区是否展开；默认折叠，进入文字频道时才自动展开 */
	chatOpen: boolean;
	/** 文字聊天区高度（px），拖动分隔条时更新 */
	chatHeight: number;
	/** 成员侧栏是否可见 */
	membersOpen: boolean;
	/**
	 * 「请回到语音视图」的请求序号。
	 * 从其它页面点左下状态条时自增，移动端据此重新展开舞台
	 * （stage 本身可能已经是 voice，单靠它无法触发变化）。
	 */
	voiceFocusNonce: number;
	showVoice: () => void;
	/** 请求回到语音视图（带序号，供移动端重新展开舞台） */
	requestVoiceView: () => void;
	showText: () => void;
	toggleChat: () => void;
	setChatOpen: (open: boolean) => void;
	setChatHeight: (height: number) => void;
	toggleMembers: () => void;
}

export const useWorkspaceUiStore = create<WorkspaceUiState>((set) => ({
	stage: "voice",
	// 默认折叠：只有进入文字频道时才由 Provider 自动展开
	chatOpen: false,
	chatHeight: loadChatHeight(),
	membersOpen: loadMembersOpen(),
	voiceFocusNonce: 0,
	showVoice: () => set({ stage: "voice" }),
	requestVoiceView: () =>
		set((s) => ({ stage: "voice", voiceFocusNonce: s.voiceFocusNonce + 1 })),
	// 选中文字频道时确保聊天区是展开的，保持语音 + 聊天同屏
	showText: () => set({ stage: "text", chatOpen: true }),
	toggleChat: () => set((s) => ({ chatOpen: !s.chatOpen })),
	setChatOpen: (open) => set({ chatOpen: open }),
	setChatHeight: (height) => {
		set({ chatHeight: height });
		try {
			localStorage.setItem(CHAT_HEIGHT_KEY, String(Math.round(height)));
		} catch {
			// 隐私模式下 localStorage 不可用，仅保留内存态
		}
	},
	toggleMembers: () =>
		set((s) => {
			const membersOpen = !s.membersOpen;
			try {
				localStorage.setItem(MEMBERS_PREF_KEY, String(membersOpen));
			} catch {
				// 隐私模式下 localStorage 不可用，仅保留内存态
			}
			return { membersOpen };
		}),
}));
