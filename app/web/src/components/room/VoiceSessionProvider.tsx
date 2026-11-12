import {
	createContext,
	useContext,
	useEffect,
	useRef,
	type ReactNode,
} from "react";
import { useChatStore } from "@/stores/chatStore";
import { useScreenShareStore } from "@/stores/screenShareStore";
import { useSocketStore } from "@/stores/socketStore";
import { useWorkspaceUiStore } from "@/stores/workspaceUiStore";
import { useRoomAudioBridge } from "./hooks/useRoomAudioBridge";
import { useRoomSounds } from "./hooks/useRoomSounds";
import { useVoiceSession } from "./hooks/useVoiceSession";

type VoiceSessionValue = ReturnType<typeof useVoiceSession>;

const VoiceSessionContext = createContext<VoiceSessionValue | null>(null);

/**
 * 语音会话的常驻宿主。
 *
 * Discord 里语音连接独立于频道视图：切到文字频道后仍在通话。因此 SFU 会话、
 * 音频桥与提示音必须挂在不会被主舞台切换卸载的层级，视图组件只消费 context。
 */
export function VoiceSessionProvider({ children }: { children: ReactNode }) {
	const session = useVoiceSession();
	useRoomAudioBridge(session.sfuClient, session.isJoined);
	useRoomSounds(session.phase);

	// 远端屏幕共享 / 摄像头画面：client 层只抛 track，渲染交给视图组件
	const sfuClient = session.sfuClient;
	useEffect(() => {
		if (!sfuClient) return;
		const store = useScreenShareStore.getState();
		for (const info of sfuClient.getExistingRemoteVideoTracks?.() ?? []) {
			store.upsertRemote(info);
		}
		sfuClient.onRemoteVideoTrack?.((info) => {
			useScreenShareStore.getState().upsertRemote(info);
		});
		sfuClient.onRemoteVideoTrackRemoved?.((identity) => {
			useScreenShareStore.getState().removeRemote(identity);
		});
		sfuClient.onLocalScreenShareChanged?.((sharing) => {
			const store = useScreenShareStore.getState();
			store.setLocalSharing(sharing);
			store.setLocalTrack(
				sharing ? (sfuClient.getLocalScreenShareTrack?.() ?? null) : null,
			);
		});
		return () => {
			useScreenShareStore.getState().reset();
		};
	}, [sfuClient]);

	const selectedRoom = useSocketStore((s) => s.selectedRoomInfo);
	const textRoom = useChatStore((s) => s.textRoom);

	// 只在「值真正变化」时切换舞台，避免初次挂载或无关渲染覆盖用户选择
	const prevSelected = useRef(selectedRoom);
	const prevText = useRef(textRoom);

	// 首次进入时若只选了文字频道（刷新后常见），直接落在文字舞台而不是空态
	useEffect(() => {
		const ui = useWorkspaceUiStore.getState();
		if (
			ui.stage === "voice" &&
			!useSocketStore.getState().selectedRoomInfo &&
			useChatStore.getState().textRoom
		) {
			ui.showText();
		}
	}, []);

	useEffect(() => {
		if (selectedRoom === prevSelected.current) return;
		prevSelected.current = selectedRoom;
		const ui = useWorkspaceUiStore.getState();
		if (selectedRoom) {
			// 加入/切换语音频道 → 展示语音舞台，文字聊天仍可通过侧栏打开
			ui.showVoice();
		} else if (ui.stage === "voice" && useChatStore.getState().textRoom) {
			// 断开语音且当前停在语音舞台 → 回落到已选中的文字频道
			ui.showText();
		}
	}, [selectedRoom]);

	useEffect(() => {
		if (textRoom === prevText.current) return;
		prevText.current = textRoom;
		const ui = useWorkspaceUiStore.getState();
		if (textRoom) {
			// 进入文字频道 → 自动展开聊天区，语音连接保持不变
			ui.showText();
			return;
		}
		// 退出文字频道 → 收起聊天区，避免它常驻占位
		ui.setChatOpen(false);
		if (ui.stage === "text" && useSocketStore.getState().selectedRoomInfo) {
			ui.showVoice();
		}
	}, [textRoom]);

	return (
		<VoiceSessionContext.Provider value={session}>
			{children}
		</VoiceSessionContext.Provider>
	);
}

export function useVoiceSessionContext(): VoiceSessionValue {
	const value = useContext(VoiceSessionContext);
	if (!value) {
		throw new Error(
			"useVoiceSessionContext must be used within VoiceSessionProvider",
		);
	}
	return value;
}
