import { createSignal, Show } from "solid-js";
import { chatStore } from "@/stores/chatStore";

interface MessageInputProps {
	replyTo?: string | null;
	onCancelReply?: () => void;
}

export default function MessageInput(props: MessageInputProps) {
	const [content, setContent] = createSignal("");
	let textareaRef: HTMLTextAreaElement | undefined;

	function resetTextareaHeight() {
		const el = textareaRef;
		if (!el) return;
		el.style.height = "auto";
		el.style.height = `${Math.min(el.scrollHeight, 72)}px`;
	}

	function handleSend() {
		const text = content().trim();
		if (!text) return;

		const opts: { reply_to?: string } = {};
		if (props.replyTo) {
			opts.reply_to = props.replyTo;
		}

		chatStore.send(text, opts);
		setContent("");
		props.onCancelReply?.();
		requestAnimationFrame(() => {
			if (textareaRef) {
				textareaRef.style.height = "auto";
			}
		});
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
		resetTextareaHeight();
	}

	return (
		<div class="border-t border-base-300 p-2 sm:p-3 safe-bottom">
			{/* Reply chip */}
			<Show when={props.replyTo}>
				<div class="flex items-center gap-2 mb-2 text-xs text-base-content/60 bg-base-200 rounded-lg px-3 py-1.5">
					<span class="i-lucide-corner-down-right shrink-0" />
					<span class="truncate">回复中...</span>
					<button
						type="button"
						class="ml-auto shrink-0 text-base-content/40 hover:text-base-content transition-colors"
						onClick={() => props.onCancelReply?.()}
					>
						✕
					</button>
				</div>
			</Show>

			<div class="flex items-end gap-2">
				<textarea
					ref={(el) => {
						textareaRef = el;
					}}
					value={content()}
					onInput={handleInput}
					onKeyDown={handleKeyDown}
					placeholder="输入消息..."
					class="textarea textarea-bordered flex-1 min-h-[40px] max-h-[96px] resize-none text-base sm:text-sm leading-relaxed py-[8px] sm:py-[6px]"
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
	);
}
