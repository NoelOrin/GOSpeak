import { onCleanup, onMount } from "solid-js";
import {
	playJoinSound,
	playLeaveSound,
} from "@/handler_audio/notificationSounds";
import { socketStore } from "@/stores/socketStore";
import userStore from "@/stores/userStore";

export function useRoomPresenceSounds() {
	onMount(() => {
		const dispose = socketStore.onPresence((event) => {
			// 仅当前选中房间触发音效，避免切房后旧房间事件误响
			const current = socketStore.selectedRoomInfo()?.name;
			if (event.room !== current) return;
			// self-join sound played by media lifecycle
			if (event.identity === userStore.user()?.name) return;
			if (event.type === "member_joined") {
				playJoinSound();
				return;
			}
			playLeaveSound();
		});

		onCleanup(dispose);
	});
}
