import type { SFUClient } from "@gospeak/sfu-client/types";
import { toast } from "@gospeak/components";
import { useEffect, useRef } from "react";
import {
	setAudioOutputDevice,
	setMasterMuted,
	setMasterVolume,
} from "@/handler_audio";
import { useAudioDeviceStore } from "@/stores/audioDeviceStore";
import { useSocketStore } from "@/stores/socketStore";
import {
	useVoiceChatActions,
	useVoiceChatStore,
} from "@/stores/voiceChatStore";

// mic 发布条件：未本地静音 且 未被服务端禁言。任一不满足即停止发布。
function micShouldPublish() {
	return (
		!useVoiceChatStore.getState().isInputMute &&
		!useSocketStore.getState().speechRestricted
	);
}

export function useRoomAudioBridge(client: SFUClient | null, joined: boolean) {
	// 每个房间会话只应用一次 muteOnJoin
	const appliedMuteOnJoinFor = useRef<SFUClient | null>(null);

	// 音量/静音持久化同步；setupAudioHandler 由 join pipeline 唯一负责
	const outputVolume = useVoiceChatStore((s) => s.outputVolume);
	const isOutMute = useVoiceChatStore((s) => s.isOutMute);
	const muteOnJoin = useAudioDeviceStore((s) => s.muteOnJoin);
	const { setIsInputMute } = useVoiceChatActions();

	useEffect(() => {
		if (!client || !joined) return;
		setMasterVolume(outputVolume / 100);
		setMasterMuted(isOutMute);

		if (appliedMuteOnJoinFor.current !== client && muteOnJoin) {
			appliedMuteOnJoinFor.current = client;
			setIsInputMute(true);
		}
	}, [client, joined, outputVolume, isOutMute, muteOnJoin, setIsInputMute]);

	const isInputMute = useVoiceChatStore((s) => s.isInputMute);
	const speechRestricted = useSocketStore((s) => s.speechRestricted);

	// biome-ignore lint/correctness/useExhaustiveDependencies: isInputMute/speechRestricted 为有意触发器（静音态变化即同步推流开关）
	useEffect(() => {
		if (!client || !joined) return;
		// 开麦可能因为权限被拒而失败：必须让用户知道"我没在说话"，而不是静默无声
		void client.setMicEnabled(micShouldPublish()).catch((err: unknown) => {
			console.warn("[voice] setMicEnabled failed:", err);
			if (micShouldPublish()) {
				toast.error("无法开启麦克风，请在浏览器地址栏允许麦克风权限后重试");
			}
		});
	}, [client, joined, isInputMute, speechRestricted]);

	// 输出设备切换（支持 setSinkId 的浏览器）
	const selectedAudioOutput = useAudioDeviceStore((s) => s.selectedAudioOutput);
	useEffect(() => {
		setAudioOutputDevice(selectedAudioOutput || "");
	}, [selectedAudioOutput]);

	// 输入设备热切换：进房时已按当前选择采集，之后只有「变化」才需要切
	const selectedAudioInput = useAudioDeviceStore((s) => s.selectedAudioInput);
	const appliedInputRef = useRef<{
		client: SFUClient | null;
		deviceId: string;
	}>({ client: null, deviceId: "" });
	useEffect(() => {
		if (!client || !joined) return;
		const applied = appliedInputRef.current;
		const isSameClient = applied.client === client;
		appliedInputRef.current = { client, deviceId: selectedAudioInput };
		// 新 client 的首个 effect 只是记录基线（join 已按该设备采集）
		if (!isSameClient || applied.deviceId === selectedAudioInput) return;
		if (!client.setAudioInputDevice) return;
		void client.setAudioInputDevice(selectedAudioInput).catch((error) => {
			console.warn("[audio] 切换麦克风失败", error);
			toast.error("切换麦克风失败，请重新进入房间后再试");
		});
	}, [client, joined, selectedAudioInput]);
}
