import { useEffect } from "react";
import { socketStore, useSocketStore } from "@/stores/socketStore";
import { useUserStore } from "@/stores/userStore";
import {
	useVoiceChatActions,
	useVoiceChatStore,
} from "@/stores/voiceChatStore";

/** 焦点在可输入元素上时不劫持按键 */
function isTypingTarget(el: EventTarget | null): boolean {
	if (!(el instanceof HTMLElement)) return false;
	const tag = el.tagName;
	return (
		tag === "INPUT" ||
		tag === "TEXTAREA" ||
		tag === "SELECT" ||
		el.isContentEditable
	);
}

/**
 * 全局快捷键（语音场景的肌肉记忆）：
 *   M  静音 / 取消静音麦克风（仅在语音房间内生效）
 *
 * 不做 Ctrl/⌘ 组合键：避免与浏览器/输入法冲突，先把最刚需的 M 落地。
 * 输入框聚焦时不触发，防止聊天打字时误静音。
 */
export function useGlobalShortcuts() {
	const currentRoom = useSocketStore((s) => s.currentRoom);
	const speechRestricted = useSocketStore((s) => s.speechRestricted);
	const { setIsInputMute } = useVoiceChatActions();

	useEffect(() => {
		const onKeyDown = (e: KeyboardEvent) => {
			if (e.metaKey || e.ctrlKey || e.altKey) return;
			if (isTypingTarget(e.target)) return;
			if (e.key.toLowerCase() !== "m") return;
			// 只在语音房间里有意义：不在房间时静音/取消静音会造成困惑
			if (!currentRoom) return;
			// 被禁言时不允许用快捷键改麦
			if (speechRestricted) return;

			const next = !useVoiceChatStore.getState().isInputMute;
			setIsInputMute(next);
			const roomName = socketStore.currentRoom();
			const myName = useUserStore.getState().user?.name;
			if (roomName && myName) {
				socketStore.emitMicState(roomName, next);
			}
			e.preventDefault();
		};

		window.addEventListener("keydown", onKeyDown);
		return () => window.removeEventListener("keydown", onKeyDown);
	}, [currentRoom, speechRestricted, setIsInputMute]);
}

export default useGlobalShortcuts;
