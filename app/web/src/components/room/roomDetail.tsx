import {
	createEffect,
	createMemo,
	createSignal,
	For,
	onCleanup,
	onMount,
	Show,
} from "solid-js";
import Avatar from "@/components/common/avatar";
import SvgIcon from "@/components/svgIcon";
import { setMutedByIdentity, setVolumeByIdentity } from "@/handler_audio";
import { speakingIdentities } from "@/handler_audio/speakingStore";
import { type MemberInfo, socketStore } from "@/stores/socketStore";
import userStore from "@/stores/userStore";
import VoiceChatStore from "@/stores/voiceChatStore";
import { hasPermission } from "@/utils/permissions";
import MemberSidebar from "./components/memberSidebar";
import { useRoomAudioBridge } from "./hooks/useRoomAudioBridge";
import { useRoomSounds } from "./hooks/useRoomSounds";
import { useVoiceSession } from "./hooks/useVoiceSession";

const RoomDetail = ({
	ref,
	mobileHideMembers = false,
}: {
	ref?: HTMLDivElement;
	/** 移动端舞台自管成员 tab，隐藏侧栏 */
	mobileHideMembers?: boolean;
}) => {
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

	const sortedMembers = createMemo(() => {
		const members = socketStore.members();
		const myName = userStore.user()?.name;
		return [...members].sort((a, b) => {
			if (a.identity === myName) return -1;
			if (b.identity === myName) return 1;
			return 0;
		});
	});
	const [_columnCount, setColumnCount] = createSignal(4);
	let containerRef: HTMLDivElement | undefined;

	const updateColumnCount = () => {
		if (!containerRef) return;
		const containerWidth = containerRef.clientWidth;
		const minItemWidth = 220;
		const gap = 16;
		const maxColumns = Math.max(
			1,
			Math.floor((containerWidth + gap) / (minItemWidth + gap)),
		);
		setColumnCount(maxColumns);
	};

	onMount(() => {
		if (!containerRef) return;
		const resizeObserver = new ResizeObserver(updateColumnCount);
		resizeObserver.observe(containerRef);
		updateColumnCount();
		onCleanup(() => resizeObserver.disconnect());
	});

	return (
		<div
			class="relative flex flex-1 flex-col justify-center items-center w-full h-full select-none"
			ref={ref}
		>
			<Show
				when={selectedRoom()}
				fallback={
					<div class="text-base-content/40 text-sm px-4 text-center">
						请从列表选择一个房间
					</div>
				}
			>
				<div class="relative flex flex-row w-full h-full min-w-0">
					<div class="flex flex-col flex-1 min-w-0">
						<Show when={isJoined()} fallback={null}>
							<div class="flex justify-between items-center px-3 sm:px-4 h-12 border-b border-base-300 gap-2">
								<div class="min-w-0">
									<div class="font-bold truncate">{currentRoom()}</div>
								</div>
								<div class="flex items-center gap-1 sm:gap-2 shrink-0">
									<span class="text-xs sm:text-sm text-base-content/60 hidden xs:inline sm:inline">
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
						</Show>
						<div class="relative flex-1 min-h-0 w-full overflow-y-auto">
							<div
								class="box-border absolute inset-0 justify-center items-center place-content-center gap-2 grid p-2 sm:p-4 w-full select-none"
								style={{
									"grid-template-columns": `repeat(auto-fit, minmax(min(100%, 140px), 1fr))`,
								}}
								ref={(el) => {
									containerRef = el;
								}}
							>
								<Show
									when={socketStore.members().length > 0}
									fallback={
										<div class="flex justify-center items-center col-span-full h-32 text-base-content/40">
											已连接，等待成员加入
										</div>
									}
								>
									<For each={sortedMembers()}>
										{(member) => <MemberCard member={member} />}
									</For>
								</Show>
							</div>
						</div>
					</div>
					<Show when={!mobileHideMembers}>
						<MemberSidebar />
					</Show>
					<Show when={!isJoined()}>
						<div class="absolute inset-0 z-10 flex flex-col items-center justify-center gap-4 bg-base-100/90 px-4">
							<div class="text-lg font-bold text-center">
								{selectedRoom()?.name}
							</div>
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
									<div class="text-sm text-error/70 text-center">
										{error() || phaseLabel()}
									</div>
									<button class="btn btn-sm btn-primary" onClick={retry}>
										重试
									</button>
								</div>
							</Show>
						</div>
					</Show>
				</div>
			</Show>
		</div>
	);
};

const MemberCard = ({ member }: { member: MemberInfo }) => {
	const isMe = () => member.identity === userStore.user()?.name;
	const displayName = () =>
		isMe()
			? userStore.user()?.display_name ||
				member.displayName ||
				member.name ||
				member.identity
			: member.displayName || member.name || member.identity;
	const memberState = () => VoiceChatStore.memberState(member.identity);
	const volume = () => memberState().outputVolume;
	const isSpeaking = () => speakingIdentities().includes(member.identity);
	const isMuted = () => memberState().isMute;

	createEffect(() => {
		setVolumeByIdentity(member.identity, volume() / 100);
		setMutedByIdentity(member.identity, isMuted());
	});

	const handleVolume = (e: Event) => {
		const val = Number((e.target as HTMLInputElement).value);
		VoiceChatStore.setMemberOutputVolume(member.identity, val);
		setVolumeByIdentity(member.identity, val / 100);
	};

	const handleKick = () => {
		const room = socketStore.currentRoom();
		if (!room) return;
		socketStore.kickMember(room, member.identity);
	};

	const handleToggleMute = () => {
		const nextMuted = !isMuted();
		VoiceChatStore.setMemberMute(member.identity, nextMuted);
		setMutedByIdentity(member.identity, nextMuted);
	};

	return (
		<div class="box-border relative flex flex-col flex-1 rounded-xl w-full aspect-5/4 select-none max-h-[80vh] min-w-0">
			<div
				class="flex justify-center items-center rounded-xl h-full overflow-hidden bg-gradient-to-br from-primary/20 to-secondary/20 transition-all duration-200"
				classList={{
					"ring-2 ring-success ring-offset-2 ring-offset-base-100":
						isSpeaking(),
				}}
			>
				<Avatar
					src={member.avatar}
					name={displayName()}
					alt={member.name}
					class="size-20"
					textClass="text-2xl"
				/>
			</div>
			<div class="flex justify-between items-center px-2 py-1">
				<span class="text-sm font-medium truncate">{displayName()}</span>
				<Show when={!isMe()}>
					<div class="dropdown dropdown-end">
						<button class="dark:text-white btn-square btn btn-xs" tabIndex={0}>
							<SvgIcon name="more" />
						</button>
						<div tabIndex={-1} class="z-1 px-0 py-1 w-24 dropdown-content menu">
							<div class="flex flex-col bg-base-100 shadow-sm rounded-lg overflow-hidden join">
								{/* 本地听感静音：仅影响当前客户端，不需要管理权限 */}
								<button
									type="button"
									class="list-item"
									onClick={handleToggleMute}
								>
									{isMuted() ? "取消静音" : "静音"}
								</button>
								{/* 踢人：服务端动作，需 signal:kick */}
								<Show when={hasPermission("signal:kick")}>
									<button type="button" class="list-item" onClick={handleKick}>
										踢出
									</button>
								</Show>
							</div>
						</div>
					</div>
				</Show>
			</div>
			<Show when={!isMe() && !isMuted()}>
				<div class="flex items-center gap-1 px-2 pb-1">
					<span class="text-[10px] text-base-content/40 whitespace-nowrap">
						音量
					</span>
					<input
						type="range"
						min="0"
						max="100"
						value={volume()}
						onInput={handleVolume}
						class="range range-xs range-primary w-full"
					/>
				</div>
			</Show>
			<Show when={isMuted()}>
				<div class="flex items-center justify-center gap-1 px-2 pb-1">
					<span class="text-[10px] text-error/80">已静音</span>
				</div>
			</Show>
		</div>
	);
};

export default RoomDetail;
