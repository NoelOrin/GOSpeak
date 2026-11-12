import clsx from "clsx";
import { useEffect } from "react";
import { Badge } from "@gospeak/components";
import AsyncState from "@/components/common/AsyncState";
import Avatar from "@/components/common/avatar";
import { chatStore, useChatStore } from "@/stores/chatStore";

export default function ConversationList() {
	const conversations = useChatStore((s) => s.conversations);
	const activeID = useChatStore((s) => s.activeConversationID);
	const loadingList = useChatStore((s) => s.loadingList);

	useEffect(() => {
		void chatStore.loadConversations();
	}, []);

	return (
		<div className="flex h-full w-full flex-col overflow-hidden select-none">
			<div className="border-border flex h-10 items-center border-b px-3">
				<h3 className="text-sm font-bold">聊天</h3>
			</div>
			<div className="min-h-0 flex-1 overflow-y-auto px-1 py-1">
				<AsyncState
					loading={loadingList && conversations.length === 0}
					loadingText="正在加载会话…"
					empty={conversations.length === 0 && !loadingList}
					emptyText="暂无会话"
					compact
				>
					{conversations.map((conv) => (
						<button
							key={conv.conversation_id}
							type="button"
							className={clsx(
								"flex w-full cursor-pointer items-center gap-2 rounded-md px-2 py-2 text-left transition-colors",
								activeID === conv.conversation_id
									? "bg-primary/10"
									: "hover:bg-border/50",
							)}
							onClick={() => chatStore.selectConversation(conv.conversation_id)}
						>
							<Avatar
								className="size-8 shrink-0"
								textClass="text-xs"
								src=""
								name={conv.other_identity}
							/>
							<div className="flex min-w-0 flex-1 flex-col">
								<div className="flex items-center justify-between">
									<span className="truncate text-sm font-medium">
										{conv.other_display_name || conv.other_identity}
									</span>
									{conv.last_message_at > 0 && (
										<span className="text-faint-foreground ml-1 shrink-0 text-2xs">
											{new Date(conv.last_message_at).toLocaleTimeString([], {
												hour: "2-digit",
												minute: "2-digit",
											})}
										</span>
									)}
								</div>
								<div className="flex items-center justify-between">
									<span className="text-faint-foreground truncate text-xs">
										{conv.last_content || "\u200B"}
									</span>
									{conv.unread_count > 0 && (
										<Badge
											count={conv.unread_count}
											className="ml-1 shrink-0"
										/>
									)}
								</div>
							</div>
						</button>
					))}
				</AsyncState>
			</div>
		</div>
	);
}
