import { For, onMount, Show } from "solid-js";
import Avatar from "@/components/common/avatar";
import { chatStore } from "@/stores/chatStore";
import guildStore from "@/stores/guildStore";
import userStore from "@/stores/userStore";

export default function MemberSidebar() {
	onMount(() => {
		const guildUUID = guildStore.state.currentGuildUUID;
		if (guildUUID) {
			void guildStore.loadMembers(guildUUID);
		}
	});

	const members = () => {
		const guildUUID = guildStore.state.currentGuildUUID;
		if (!guildUUID) return [];
		return guildStore.state.memberCache[guildUUID] || [];
	};

	const currentUserUUID = () => userStore.user()?.uuid || "";

	return (
		<div class="flex flex-col h-full overflow-y-auto">
			<div class="px-3 py-2 text-xs font-semibold text-base-content/50 uppercase tracking-wide shrink-0">
				成员
			</div>
			<Show
				when={members().length > 0}
				fallback={
					<div class="px-3 py-8 text-center text-sm text-base-content/30">
						暂无成员
					</div>
				}
			>
				<For each={members()}>
					{(member) => (
						<Show when={member.user_uuid !== currentUserUUID()}>
							<button
								type="button"
								class="flex items-center gap-2 px-3 py-2 hover:bg-base-200 transition-colors w-full text-left"
								onClick={() =>
									void chatStore.startConversation(
										member.nickname || member.user_uuid,
									)
								}
							>
								<Avatar
									name={member.nickname}
									alt={member.nickname}
									class="size-7"
								/>
								<span class="text-sm truncate">
									{member.nickname || member.user_uuid}
								</span>
							</button>
						</Show>
					)}
				</For>
			</Show>
		</div>
	);
}
