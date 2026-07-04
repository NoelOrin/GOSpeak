import { Show } from "solid-js";
import { socketStore } from "@/stores/socketStore";
import MemberSidebar from "./components/memberSidebar";
import VoiceChat from "./components/voiceChat";
import { useRoomAudioBridge } from "./hooks/useRoomAudioBridge";
import { useRoomJoinSession } from "./hooks/useRoomJoinSession";
import { useRoomPresenceSounds } from "./hooks/useRoomPresenceSounds";

const RoomDetail = ({ ref }: { ref?: HTMLDivElement }) => {
	const {
		selectedRoom,
		joinState,
		sfuClient,
		isJoined,
		isReconnecting,
		currentRoom,
		handleManualLeave,
	} = useRoomJoinSession();
	useRoomAudioBridge(sfuClient, isJoined);
	useRoomPresenceSounds();

	return (
		<div
			class="flex flex-1 flex-col justify-center items-center w-full h-full select-none"
			ref={ref}
		>
			<Show
				when={selectedRoom()}
				fallback={
					<div class="text-base-content/40 text-sm">
						请从左侧列表选择一个房间
					</div>
				}
			>
				<Show
					when={isJoined()}
					fallback={
						<div class="flex flex-col items-center gap-4">
							<div class="text-lg font-bold">{selectedRoom()?.name}</div>
							<Show
								when={joinState() !== "failed"}
								fallback={
									<div class="text-sm text-error/70">加入失败，请重试</div>
								}
							>
								<div class="loading loading-spinner loading-sm" />
								<div class="text-sm text-base-content/40">正在加入...</div>
							</Show>
						</div>
					}
				>
					<div class="flex flex-row w-full h-full">
						<div class="flex flex-col flex-1">
							<Show when={isReconnecting()}>
								<div class="flex items-center gap-2 px-4 h-9 bg-warning/10 border-b border-warning/30 text-xs text-warning">
									<div class="loading loading-spinner loading-xs" />
									正在重连...
								</div>
							</Show>
							<div class="flex justify-between items-center px-4 h-12 border-b border-base-300">
								<div class="min-w-0">
									<div class="font-bold truncate">{currentRoom()}</div>
									<div class="flex items-center gap-1 mt-1 text-[11px] text-base-content/55">
										{/* <Show when={selectedRoom()?.audioOnly}>
                      <span class="badge badge-ghost badge-xs">语音</span>
                    </Show>
                    <Show when={selectedRoom()?.allowAudience === false}>
                      <span class="badge badge-ghost badge-xs">仅成员</span>
                    </Show> */}
										{/* <Show when={selectedRoom()?.description}>
                      <span class="truncate">
                        {selectedRoom()?.description}
                      </span>
                    </Show> */}
									</div>
								</div>
								<div class="flex items-center gap-2">
									<span class="text-sm text-base-content/60">
										{socketStore.members().length} 人在线
									</span>
									<button
										class="btn btn-sm btn-ghost"
										onClick={handleManualLeave}
									>
										离开
									</button>
								</div>
							</div>
							<VoiceChat />
						</div>
						<MemberSidebar />
					</div>
				</Show>
			</Show>
		</div>
	);
};

export default RoomDetail;
