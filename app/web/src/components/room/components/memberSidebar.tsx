import MicOff from "lucide-solid/icons/mic-off";
import { createSignal, For, Show } from "solid-js";
import Avatar from "@/components/common/avatar";
import { socketStore } from "@/stores/socketStore";
import userStore from "@/stores/userStore";
import UserInfoPopover from "./userInfoPopover";

// ---------- MemberListItem ----------
interface MemberItemProps {
	identity: string;
	name: string;
	displayName: string;
	avatar: string;
	isMicMuted: boolean;
	isMuted: boolean;
	isMe: boolean;
	onSelect: (identity: string, x: number, y: number) => void;
	isSelected: boolean;
}

const MemberListItem = (props: MemberItemProps) => {
	const displayName = () => {
		if (props.isMe)
			return (
				userStore.user()?.display_name ||
				props.displayName ||
				props.name ||
				props.identity
			);
		return props.displayName || props.name || props.identity;
	};

	return (
		<button
			type="button"
			class="flex items-center gap-2 px-2 py-1.5 rounded-md transition-colors w-full text-left"
			classList={{
				"bg-base-300/50": props.isSelected,
				"hover:bg-base-300/50": !props.isSelected,
			}}
			onClick={(e) => props.onSelect(props.identity, e.clientX, e.clientY)}
		>
			<Avatar
				class="size-6"
				textClass="text-[10px]"
				src={props.avatar}
				name={props.displayName || props.name || props.identity}
			/>
			<span class="text-xs font-medium truncate flex-1">{displayName()}</span>
			<Show when={props.isMicMuted || props.isMuted}>
				<MicOff size={12} class="text-base-content/40 shrink-0" />
			</Show>
			<Show when={props.isMe}>
				<span class="text-[10px] text-base-content/40">你</span>
			</Show>
		</button>
	);
};

// ---------- MemberSidebar ----------
const MemberSidebar = () => {
	const [selectedIdentity, setSelectedIdentity] = createSignal<{
		identity: string;
		x: number;
		y: number;
	} | null>(null);

	const closePopover = () => setSelectedIdentity(null);

	const handleSelect = (identity: string, x: number, y: number) => {
		setSelectedIdentity((prev) =>
			prev?.identity === identity ? null : { identity, x, y },
		);
	};

	const sortedMembers = () => {
		const members = socketStore.members();
		const myName = userStore.user()?.name;
		return [...members].sort((a, b) => {
			if (a.identity === myName) return -1;
			if (b.identity === myName) return 1;
			return 0;
		});
	};

	return (
		<div class="relative flex flex-col w-52 border-l border-base-300 h-full overflow-hidden shrink-0">
			<div class="flex items-center px-3 h-10 border-b border-base-300 text-xs font-bold text-base-content/70">
				成员 ({socketStore.members().length})
			</div>

			<div class="flex-1 overflow-y-auto px-1 py-1">
				<Show
					when={sortedMembers().length > 0}
					fallback={
						<div class="flex justify-center items-center h-20 text-xs text-base-content/40">
							暂无成员
						</div>
					}
				>
					<For each={sortedMembers()}>
						{(member) => (
							<MemberListItem
								identity={member.identity}
								name={member.name}
								displayName={member.displayName}
								avatar={member.avatar}
								isMicMuted={member.isMicMuted}
								isMuted={member.isMuted}
								isMe={member.identity === userStore.user()?.name}
								onSelect={handleSelect}
								isSelected={selectedIdentity()?.identity === member.identity}
							/>
						)}
					</For>
				</Show>
			</div>
			<Show when={selectedIdentity()}>
				{(sel) => (
					<UserInfoPopover
						identity={sel().identity}
						pos={{ x: sel().x, y: sel().y }}
						onClose={closePopover}
					/>
				)}
			</Show>
		</div>
	);
};

export default MemberSidebar;
