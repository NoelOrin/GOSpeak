import type { SFUClient } from "@gospeak/sfu-client/types";
import { createEffect } from "solid-js";
import {
	cleanupAudioHandler,
	setAudioOutputDevice,
	setMasterMuted,
	setMasterVolume,
} from "@/handler_audio";
import AudioDeviceStore from "@/stores/audioDeviceStore";
import { socketStore } from "@/stores/socketStore";
import VoiceChatStore from "@/stores/voiceChatStore";

// mic 发布条件：未本地静音 且 未被服务端禁言。任一不满足即停止发布。
function micShouldPublish() {
	return !VoiceChatStore.data.isInputMute && !socketStore.speechRestricted();
}

export function useRoomAudioBridge(
	client: () => SFUClient | null,
	joined: () => boolean,
) {
	// 每个房间会话只应用一次 muteOnJoin
	let appliedMuteOnJoinFor: SFUClient | null = null;

	// 音量/静音持久化同步；setupAudioHandler 由 join pipeline 唯一负责
	createEffect(() => {
		const currentClient = client();
		if (!currentClient || !joined()) return;
		setMasterVolume(VoiceChatStore.data.outputVolume / 100);
		setMasterMuted(VoiceChatStore.data.isOutMute);

		if (
			appliedMuteOnJoinFor !== currentClient &&
			AudioDeviceStore.state.muteOnJoin
		) {
			appliedMuteOnJoinFor = currentClient;
			VoiceChatStore.setIsInputMute(true);
		}
	});

	createEffect(() => {
		const currentClient = client();
		if (!currentClient || !joined()) return;
		void currentClient.setMicEnabled(micShouldPublish());
	});

	// 输出设备切换（支持 setSinkId 的浏览器）
	createEffect(() => {
		setAudioOutputDevice(AudioDeviceStore.state.selectedAudioOutput || "");
	});
}

export function teardownRoomAudioBridge() {
	cleanupAudioHandler();
}
