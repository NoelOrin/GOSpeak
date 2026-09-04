import clsx from "clsx";
import { For, Show } from "solid-js";
// import type { ConversationDTO } from "@/api/conversation";
import { chatStore } from "@/stores/chatStore";
import Avatar from "@/components/common/avatar";

interface ConversationListProps {
	onSelect: (id: string) => void;
}

const ConversationList = (_props: ConversationListProps) => {
	const conversations = chatStore.conversations;
	const activeID = chatStore.activeConversationID;

	return (
		<div class="flex flex-col w-full h-full overflow-hidden select-none">
			<div class="flex items-center px-3 border-base-300 border-b h-10">
				<h3 class="font-bold text-sm">聊天</h3>
			</div>
			<div class="flex-1 px-1 py-1 overflow-y-auto">
				<Show
					when={conversations().length > 0}
					fallback={
						<div class="flex justify-center items-center h-20 text-xs text-base-content/40">
							暂无会话
						</div>
					}
				>
					<For each={conversations()}>
						{(conv) => (
							<button
								type="button"
								class={clsx(
									"flex items-center gap-2 px-2 py-2 rounded-md w-full text-left transition-colors",
									activeID() === conv.conversation_id
										? "bg-primary/10"
										: "hover:bg-base-300/50",
								)}
								onClick={() =>
									chatStore.selectConversation(conv.conversation_id)
								}
							>
								<Avatar
									class="size-8 shrink-0"
									textClass="text-xs"
									src=""
									name={conv.other_identity}
								/>
								<div class="flex flex-col flex-1 min-w-0">
									<div class="flex justify-between items-center">
										<span class="font-medium text-sm truncate">
											{conv.other_display_name || conv.other_identity}
										</span>
										<Show when={conv.last_message_at > 0}>
											<span class="ml-1 text-[10px] text-base-content/40 shrink-0">
												{new Date(conv.last_message_at).toLocaleTimeString([], {
													hour: "2-digit",
													minute: "2-digit",
												})}
											</span>
										</Show>
									</div>
									<div class="flex justify-between items-center">
										<span class="text-xs text-base-content/50 truncate">
											{conv.last_content || "\u200B"}
										</span>
										<Show when={conv.unread_count > 0}>
											<span class="ml-1 badge badge-xs badge-primary shrink-0">
												{conv.unread_count > 99 ? "99+" : conv.unread_count}
											</span>
										</Show>
									</div>
								</div>
							</button>
						)}
					</For>
				</Show>
			</div>
		</div>
	);
};

export default ConversationList;
