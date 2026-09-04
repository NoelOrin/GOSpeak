import { createSignal, For, Show } from "solid-js";
import { socketStore } from "@/stores/socketStore";
import userStore from "@/stores/userStore";
import Avatar from "@/components/common/avatar";

interface MemberSidebarProps {
	onStartChat: (identity: string, displayName: string) => void;
}

const MemberSidebar = (props: MemberSidebarProps) => {
	const [_, setSelectedIdentity] = createSignal<string | null>(null);

	const members = () => socketStore.members();

	const sortedMembers = () => {
		const myName = userStore.user()?.name;
		return [...members()].sort((a, b) => {
			if (a.identity === myName) return -1;
			if (b.identity === myName) return 1;
			return 0;
		});
	};

	const handleClick = (identity: string, displayName: string) => {
		setSelectedIdentity(identity);
		props.onStartChat(identity, displayName);
	};

	return (
		<div class="flex flex-col w-52 border-l border-base-300 h-full overflow-hidden shrink-0 ml-auto">
			<div class="flex items-center px-3 h-10 border-b border-base-300 text-xs font-bold text-base-content/70">
				服务器成员 ({members().length})
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
							<button
								type="button"
								class="flex items-center gap-2 px-2 py-1.5 rounded-md transition-colors w-full text-left hover:bg-base-300/50"
								onClick={() =>
									handleClick(
										member.identity,
										member.displayName || member.name || member.identity,
									)
								}
							>
								<Avatar
									class="size-6 shrink-0"
									textClass="text-[10px]"
									src={member.avatar}
									name={member.displayName || member.name || member.identity}
								/>
								<span class="text-xs font-medium truncate flex-1">
									{member.displayName || member.name || member.identity}
								</span>
								<Show when={member.identity === userStore.user()?.name}>
									<span class="text-[10px] text-base-content/40">你</span>
								</Show>
							</button>
						)}
					</For>
				</Show>
			</div>
		</div>
	);
};

export default MemberSidebar;
