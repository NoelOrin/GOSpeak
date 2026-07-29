import clsx from "clsx";
import Send from "lucide-solid/icons/send";
import { createEffect, createSignal, For, Show } from "solid-js";
import Avatar from "@/components/common/avatar";
import { chatStore } from "@/stores/chatStore";
import userStore from "@/stores/userStore";

const ChatWindow = () => {
	const activeID = chatStore.activeConversationID;
	const messages = chatStore.messages;
	const hasMore = chatStore.hasMore;
	const loadingMessages = chatStore.loadingMessages;
	const [inputText, setInputText] = createSignal("");
	let messagesEndRef!: HTMLDivElement;
	let messagesContainerRef!: HTMLDivElement;

	const activeConv = () =>
		chatStore.conversations().find((c) => c.conversation_id === activeID());

	const convMessages = () => messages()[activeID() || ""] || [];

	const isSelf = (identity: string) => identity === userStore.user()?.name;

	const handleSend = () => {
		const text = inputText().trim();
		if (!text || !activeID()) return;
		chatStore.sendMessage(activeID()!, text);
		setInputText("");
	};

	const handleKeyDown = (e: KeyboardEvent) => {
		if (e.key === "Enter" && !e.shiftKey) {
			e.preventDefault();
			handleSend();
		}
	};

	const handleScroll = () => {
		const el = messagesContainerRef;
		if (!el || el.scrollTop > 50) return;
		if (
			activeID() &&
			hasMore()[activeID()!] &&
			!loadingMessages()[activeID()!]
		) {
			chatStore.loadMoreMessages(activeID()!);
		}
	};

	// Auto-scroll to bottom on new messages
	// Auto-scroll when messages change or conversation switches
	createEffect(() => {
		const msgs = convMessages();
		// trigger on length change (new messages or conversation switch)
		if (msgs.length > 0) {
			setTimeout(() => {
				messagesEndRef?.scrollIntoView({ behavior: "smooth" });
			}, 100);
		}
	});

	return (
		<div class="flex flex-col flex-1 h-full overflow-hidden">
			<Show
				when={activeID()}
				fallback={
					<div class="flex justify-center items-center h-full text-sm text-base-content/40 select-none">
						选择一个会话开始聊天
					</div>
				}
			>
				{/* Top bar */}
				<div class="flex items-center px-4 h-12 border-b border-base-300 shrink-0">
					<div class="font-bold truncate">
						{activeConv()?.other_display_name || activeConv()?.other_identity}
					</div>
				</div>

				{/* Messages area */}
				<div
					ref={messagesContainerRef!}
					class="flex-1 overflow-y-auto px-4 py-2 space-y-1"
					onScroll={handleScroll}
				>
					<Show when={loadingMessages()[activeID()!]}>
						<div class="flex justify-center py-2">
							<span class="loading loading-spinner loading-xs" />
						</div>
					</Show>
					<Show
						when={!loadingMessages()[activeID()!] && hasMore()[activeID()!]}
					>
						<div class="flex justify-center py-2">
							<button
								class="btn btn-xs btn-ghost"
								onClick={() => chatStore.loadMoreMessages(activeID()!)}
							>
								加载更多
							</button>
						</div>
					</Show>
					<For each={convMessages()}>
						{(msg) => {
							const mine = isSelf(msg.senderIdentity);
							return (
								<div
									class={clsx(
										"flex gap-2 max-w-[80%]",
										mine ? "ml-auto flex-row-reverse" : "",
									)}
								>
									<Show when={!mine}>
										<Avatar
											class="size-7 shrink-0 mt-1"
											textClass="text-[10px]"
											src=""
											name={msg.senderDisplay || msg.senderIdentity}
										/>
									</Show>
									<div
										class={clsx(
											"px-3 py-1.5 rounded-lg text-sm break-words",
											mine
												? "bg-primary text-primary-content rounded-br-sm"
												: "bg-base-300 rounded-bl-sm",
										)}
									>
										{msg.content}
										<div
											class={clsx(
												"text-[10px] mt-0.5",
												mine
													? "text-primary-content/60"
													: "text-base-content/40",
											)}
										>
											{new Date(msg.timestamp).toLocaleTimeString([], {
												hour: "2-digit",
												minute: "2-digit",
											})}
										</div>
									</div>
								</div>
							);
						}}
					</For>
					<div ref={messagesEndRef!} />
				</div>

				{/* Input area */}
				<div class="flex items-center gap-2 px-4 py-2 border-t border-base-300 shrink-0">
					<textarea
						class="textarea textarea-bordered textarea-sm flex-1 resize-none min-h-[36px] max-h-[120px]"
						placeholder="输入消息..."
						value={inputText()}
						onInput={(e) => setInputText(e.currentTarget.value)}
						onKeyDown={handleKeyDown}
						rows={1}
					/>
					<button
						class="btn btn-sm btn-primary btn-square"
						onClick={handleSend}
						disabled={!inputText().trim() || !activeID()}
					>
						<Send size={16} />
					</button>
				</div>
			</Show>
		</div>
	);
};

export default ChatWindow;
