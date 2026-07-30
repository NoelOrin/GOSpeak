import { For, onMount, Show } from "solid-js";
import Avatar from "@/components/common/avatar";
import { chatStore } from "@/stores/chatStore";

export default function ConversationList() {
	onMount(() => {
		void chatStore.loadConversations();
	});

	const active = () => chatStore.activeConversationID();

	return (
		<div class="flex flex-col h-full overflow-y-auto">
			<div class="px-3 py-2 text-xs font-semibold text-base-content/50 uppercase tracking-wide">
				私聊
			</div>
			<Show
				when={chatStore.conversations().length > 0}
				fallback={
					<div class="px-3 py-8 text-center text-sm text-base-content/30">
						暂无私聊会话
					</div>
				}
			>
				<For each={chatStore.conversations()}>
					{(conv) => (
						<button
							type="button"
							class="flex items-center gap-3 px-3 py-2.5 hover:bg-base-200 transition-colors text-left w-full"
							classList={{
								"bg-base-200": active() === conv.conversation_id,
							}}
							onClick={() =>
								void chatStore.selectConversation(conv.conversation_id)
							}
						>
							<Avatar
								src={conv.other_avatar || undefined}
								name={conv.other_display_name}
								alt={conv.other_display_name}
								class="size-9"
							/>
							<div class="flex-1 min-w-0">
								<div class="flex items-center justify-between gap-2">
									<span class="text-sm font-medium truncate">
										{conv.other_display_name || conv.other_identity}
									</span>
									<Show when={conv.unread_count > 0}>
										<span class="badge badge-primary badge-xs shrink-0">
											{conv.unread_count}
										</span>
									</Show>
								</div>
								<div class="text-xs text-base-content/40 truncate">
									{conv.last_content || "开始对话"}
								</div>
							</div>
						</button>
					)}
				</For>
			</Show>
		</div>
	);
}
