import { createEffect, onCleanup, onMount } from "solid-js";
import type { VoicePhase } from "@/components/room/session/voiceSessionTypes";
import {
	playJoinSound,
	playLeaveSound,
} from "@/handler_audio/notificationSounds";
import { socketStore } from "@/stores/socketStore";
import userStore from "@/stores/userStore";

/**
 * 统一房间进入/离开提示音，集中在房间编排层处理，不进入 SFU 抽象层：
 * - 他人进入/离开：由信令在线状态（presence）驱动，与媒体会话无关
 * - 自己进入：由媒体会话建连完成（media_ready/ready）驱动
 * 音效实现留在 handler_audio/notificationSounds，SFUClient 只负责媒体会话生命周期。
 */
export function useRoomSounds(phase: () => VoicePhase) {
	onMount(() => {
		const dispose = socketStore.onPresence((event) => {
			const current = socketStore.selectedRoomInfo()?.name;
			if (event.room !== current) return;
			// 自己的进入音由媒体生命周期播放，避免重复触发
			if (event.identity === userStore.user()?.name) return;
			if (event.type === "member_joined") {
				playJoinSound();
				return;
			}
			playLeaveSound();
		});
		onCleanup(dispose);
	});

	let playedJoinOnMedia = false;
	let prevPhase: VoicePhase = "idle";
	createEffect(() => {
		const p = phase();
		if (p === "media_ready" || p === "ready") {
			if (!playedJoinOnMedia) {
				playedJoinOnMedia = true;
				playJoinSound();
			}
		} else if (p === "idle" || p === "resolving") {
			playedJoinOnMedia = false;
		}

		// 自己离开房间：从已进房态（ready/media_ready/reconnecting）转入非进房态时
		// 播放下行音。覆盖主动离开与切房——切房时 session 直接被替换成新房的
		// resolving，不会经过 leaving，故用“joined -> 非 joined”跳变兜底。
		// 异常断连走 failed，不在此集合，不误播。
		const wasJoined =
			prevPhase === "ready" ||
			prevPhase === "media_ready" ||
			prevPhase === "reconnecting";
		const leftPhase =
			p === "resolving" || p === "idle" || p === "leaving";
		if (wasJoined && leftPhase) {
			playLeaveSound();
		}
		prevPhase = p;
	});
}
