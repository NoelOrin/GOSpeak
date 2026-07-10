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

const VoiceChat = ({ ref }: { ref?: HTMLDivElement }) => {
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
		<div class="relative w-full h-full overflow-y-auto">
			<div
				class="box-border absolute inset-0 justify-center items-center place-content-center gap-2 grid p-4 w-full select-none"
				style={{
					"grid-template-columns": `repeat(auto-fit, minmax(190px, 1fr))`,
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
		<div class="box-border relative flex flex-col flex-1 rounded-xl w-full aspect-5/4 select-none max-h-[80vh]">
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
				<Show when={!isMe() && hasPermission("signal:kick")}>
					<div class="dropdown dropdown-end">
						<button class="dark:text-white btn-square btn btn-xs" tabIndex={0}>
							<SvgIcon name="more" />
						</button>
						<div tabIndex={-1} class="z-1 px-0 py-1 w-24 dropdown-content menu">
							<div class="flex flex-col bg-base-100 shadow-sm rounded-lg overflow-hidden join">
								<button type="button" class="list-item" onClick={handleKick}>
									踢出
								</button>
								<button
									type="button"
									class="list-item"
									onClick={handleToggleMute}
								>
									{isMuted() ? "取消静音" : "静音"}
								</button>
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

export default VoiceChat;
