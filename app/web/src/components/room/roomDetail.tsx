import { Show } from "solid-js";
import { socketStore } from "@/stores/socketStore";
import MemberSidebar from "./components/memberSidebar";
import VoiceChat from "./components/voiceChat";
import { useRoomAudioBridge } from "./hooks/useRoomAudioBridge";
import { useRoomSounds } from "./hooks/useRoomSounds";
import { useVoiceSession } from "./hooks/useVoiceSession";

const RoomDetail = ({ ref }: { ref?: HTMLDivElement }) => {
	const {
		selectedRoom,
		phase,
		phaseLabel,
		sfuClient,
		isJoined,
		isLoading,
		currentRoom,
		error,
		handleManualLeave,
		retry,
	} = useVoiceSession();
	useRoomAudioBridge(sfuClient, isJoined);
	useRoomSounds(phase);

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
								when={phase() === "failed"}
								fallback={
									<div class="flex flex-col items-center gap-4">
										<div class="loading loading-spinner loading-sm" />
										<div class="text-sm text-base-content/40">
											{isLoading() ? phaseLabel() : "准备加入..."}
										</div>
									</div>
								}
							>
								<div class="flex flex-col items-center gap-3">
									<div class="text-sm text-error/70">
										{error() || phaseLabel()}
									</div>
									<button class="btn btn-sm btn-primary" onClick={retry}>
										重试
									</button>
								</div>
							</Show>
						</div>
					}
				>
					<div class="flex flex-row w-full h-full">
						<div class="flex flex-col flex-1">
							<div class="flex justify-between items-center px-4 h-12 border-b border-base-300">
								<div class="min-w-0">
									<div class="font-bold truncate">{currentRoom()}</div>
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
