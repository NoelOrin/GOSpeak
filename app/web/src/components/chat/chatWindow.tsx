import { createEffect, createSignal, For, Show } from "solid-js";
import type { PrivateMessageDTO } from "@/api/conversation";
import { chatStore } from "@/stores/chatStore";
import userStore from "@/stores/userStore";

export default function ChatWindow() {
	const active = () => chatStore.activeConversationID();
	const messages = () => {
		const id = active();
		return id ? chatStore.pmMessages()[id] || [] : [];
	};
	let scrollRef!: HTMLDivElement;
	const [content, setContent] = createSignal("");
	let textareaRef: HTMLTextAreaElement | undefined;

	createEffect(() => {
		const len = messages().length;
		if (len > 0 && scrollRef) {
			requestAnimationFrame(() => {
				scrollRef.scrollTop = scrollRef.scrollHeight;
			});
		}
	});

	function handleSend() {
		const text = content().trim();
		const id = active();
		if (!text || !id) return;
		chatStore.sendDirect(id, text);
		setContent("");
		if (textareaRef) textareaRef.style.height = "auto";
	}

	function handleKeyDown(e: KeyboardEvent) {
		if (e.key === "Enter" && !e.shiftKey) {
			e.preventDefault();
			handleSend();
		}
	}

	function handleInput() {
		if (!textareaRef) return;
		setContent(textareaRef.value);
		textareaRef.style.height = "auto";
		textareaRef.style.height = `${Math.min(textareaRef.scrollHeight, 96)}px`;
	}

	const isOwn = (msg: PrivateMessageDTO) => {
		const u = userStore.user();
		if (!u) return false;
		return msg.author_id === u.name || msg.author_id === u.display_name;
	};

	const formatTime = (iso: string) => {
		try {
			return new Date(iso).toLocaleTimeString([], {
				hour: "2-digit",
				minute: "2-digit",
			});
		} catch {
			return "";
		}
	};

	return (
		<div class="flex flex-col h-full">
			<div class="px-4 py-3 border-b border-base-300 shrink-0">
				<span class="text-sm font-semibold">
					{active()
						? chatStore
								.conversations()
								.find((c) => c.conversation_id === active())
								?.other_display_name || "私聊"
						: "选择一个会话"}
				</span>
			</div>

			<div ref={scrollRef} class="flex-1 overflow-y-auto px-4 py-2">
				<Show
					when={messages().length > 0}
					fallback={
						<div class="flex items-center justify-center h-full text-sm text-base-content/30">
							发送一条消息开始对话
						</div>
					}
				>
					<For each={messages()}>
						{(msg) => (
							<div
								class="flex flex-col mb-2"
								classList={{
									"items-end": isOwn(msg),
									"items-start": !isOwn(msg),
								}}
							>
								<div
									class="max-w-[70%] rounded-2xl px-3 py-1.5 text-sm break-words"
									classList={{
										"bg-primary text-primary-content": isOwn(msg),
										"bg-base-200": !isOwn(msg),
									}}
								>
									{msg.deleted ? "[消息已删除]" : msg.content}
								</div>
								<span class="text-[10px] text-base-content/30 mt-0.5">
									{formatTime(msg.created_at)}
								</span>
							</div>
						)}
					</For>
				</Show>
			</div>

			<Show when={active()}>
				<div class="border-t border-base-300 p-2 shrink-0">
					<div class="flex items-end gap-2">
						<textarea
							ref={(el) => (textareaRef = el)}
							value={content()}
							onInput={handleInput}
							onKeyDown={handleKeyDown}
							placeholder="输入消息..."
							class="textarea textarea-bordered flex-1 min-h-[40px] max-h-[96px] resize-none text-sm"
							rows={1}
						/>
						<button
							type="button"
							class="btn btn-primary h-10 min-h-10 shrink-0 px-4"
							disabled={!content().trim()}
							onClick={handleSend}
						>
							发送
						</button>
					</div>
				</div>
			</Show>
		</div>
	);
}
