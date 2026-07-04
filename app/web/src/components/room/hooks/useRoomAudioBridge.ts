import type { SFUClient } from "@gospeak/sfu-client/types";
import { createEffect, onCleanup } from "solid-js";
import {
	cleanupAudioHandler,
	setMasterMuted,
	setMasterVolume,
	setupAudioHandler,
} from "@/handler_audio";
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
	createEffect(() => {
		const currentClient = client();
		if (!currentClient) return;
		setupAudioHandler(currentClient);
		// 把 store 持久化的全局输出音量/静音同步回 handler（cleanup 已重置为默认）。
		setMasterVolume(VoiceChatStore.data.outputVolume / 100);
		setMasterMuted(VoiceChatStore.data.isOutMute);
		onCleanup(cleanupAudioHandler);
	});

	createEffect(() => {
		const currentClient = client();
		if (!currentClient || !joined()) return;
		void currentClient.setMicEnabled(micShouldPublish());
	});
}

export function teardownRoomAudioBridge() {
	cleanupAudioHandler();
}
