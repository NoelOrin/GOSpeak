import Paperclip from "lucide-solid/icons/paperclip";
import { createSignal, For, Show } from "solid-js";
import { showToast } from "solid-notifications";
import { useUpload } from "@/hooks/useUpload";
import { chatStore } from "@/stores/chatStore";
import { socketStore } from "@/stores/socketStore";

interface MessageInputProps {
	replyTo?: string | null;
	onCancelReply?: () => void;
}

export default function MessageInput(props: MessageInputProps) {
	const [content, setContent] = createSignal("");
	const [mentions, setMentions] = createSignal<string[]>([]);
	const [mentionOpen, setMentionOpen] = createSignal(false);
	const [mentionQuery, setMentionQuery] = createSignal("");
	const [mentionIndex, setMentionIndex] = createSignal(0);
	const { upload, uploading } = useUpload("chat");
	let textareaRef: HTMLTextAreaElement | undefined;
	let fileInputRef: HTMLInputElement | undefined;

	const members = () => {
		const roomName = chatStore.textRoomName();
		const room = socketStore.rooms().find((r) => r.name === roomName);
		return room?.members ?? [];
	};

	const mentionCandidates = () => {
		const q = mentionQuery().toLowerCase();
		return members()
			.map((m) => m.identity || m.name)
			.filter((name, index, arr) => name && arr.indexOf(name) === index)
			.filter((name) => !q || name.toLowerCase().includes(q))
			.slice(0, 8);
	};

	function resetTextareaHeight() {
		const el = textareaRef;
		if (!el) return;
		el.style.height = "auto";
		el.style.height = `${Math.min(el.scrollHeight, 72)}px`;
	}

	function resetAfterSend() {
		setContent("");
		setMentions([]);
		setMentionOpen(false);
		props.onCancelReply?.();
		requestAnimationFrame(() => {
			if (textareaRef) textareaRef.style.height = "auto";
		});
	}

	function handleSend() {
		const text = content().trim();
		if (!text || uploading()) return;

		const opts: { reply_to?: string; mentions?: string[] } = {};
		if (props.replyTo) opts.reply_to = props.replyTo;
		if (mentions().length > 0) opts.mentions = mentions();

		chatStore.send(text, opts);
		resetAfterSend();
	}

	function applyMention(name: string) {
		const el = textareaRef;
		if (!el) return;
		const value = el.value;
		const caret = el.selectionStart ?? value.length;
		const before = value.slice(0, caret);
		const at = before.lastIndexOf("@");
		const prefix = at >= 0 ? before.slice(0, at) : before;
		const after = value.slice(caret);
		const next = `${prefix}@${name} ${after}`;
		setContent(next);
		setMentions((prev) => (prev.includes(name) ? prev : [...prev, name]));
		setMentionOpen(false);
		requestAnimationFrame(() => {
			el.focus();
			const pos = prefix.length + name.length + 2;
			el.setSelectionRange(pos, pos);
		});
	}

	function handleInput() {
		const el = textareaRef;
		if (!el) return;
		setContent(el.value);
		resetTextareaHeight();

		const caret = el.selectionStart ?? el.value.length;
		const before = el.value.slice(0, caret);
		const at = before.lastIndexOf("@");
		const hasTrigger = at >= 0 && (at === 0 || /[\s@]/.test(before[at - 1]));
		if (hasTrigger && before.slice(at + 1).length < 40) {
			setMentionQuery(before.slice(at + 1).replace(/^@/, ""));
			setMentionIndex(0);
			setMentionOpen(true);
		} else {
			setMentionOpen(false);
		}
	}

	function handleKeyDown(e: KeyboardEvent) {
		const candidates = mentionCandidates();
		if (mentionOpen() && candidates.length > 0) {
			if (e.key === "ArrowDown") {
				e.preventDefault();
				setMentionIndex((i) => (i + 1) % candidates.length);
				return;
			}
			if (e.key === "ArrowUp") {
				e.preventDefault();
				setMentionIndex((i) => (i - 1 + candidates.length) % candidates.length);
				return;
			}
			if (e.key === "Enter" || e.key === "Tab") {
				e.preventDefault();
				applyMention(candidates[mentionIndex()] || candidates[0]);
				return;
			}
		}
		if (e.key === "Enter" && !e.shiftKey) {
			e.preventDefault();
			handleSend();
		}
	}

	async function handleFile(file: File | undefined) {
		if (!file) return;
		try {
			const result = await upload(file);
			const token = file.type.startsWith("image/")
				? `![${file.name}](${result.public_url})`
				: `[${file.name}](${result.public_url})`;
			setContent((prev) => `${prev.trimEnd()}\n${token}\n`);
			requestAnimationFrame(() => {
				if (textareaRef) {
					textareaRef.focus();
					resetTextareaHeight();
				}
			});
		} catch (err) {
			showToast(err instanceof Error ? err.message : "上传失败", {
				type: "error",
			});
		} finally {
			if (fileInputRef) fileInputRef.value = "";
		}
	}

	return (
		<div class="border-t border-base-300 p-2 sm:p-3 safe-bottom relative">
			<Show when={mentionOpen() && mentionCandidates().length > 0}>
				<div class="absolute bottom-full left-3 mb-2 w-56 rounded-lg border border-base-300 bg-base-100 shadow-xl z-20 overflow-hidden">
					<For each={mentionCandidates()}>
						{(name, index) => (
							<button
								type="button"
								class="block w-full text-left px-3 py-2 text-sm hover:bg-base-200"
								classList={{ "bg-base-200": index() === mentionIndex() }}
								onMouseDown={(e) => e.preventDefault()}
								onClick={() => applyMention(name)}
							>
								@{name}
							</button>
						)}
					</For>
				</div>
			</Show>

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
					placeholder="输入消息，支持 Markdown、附件和 @提及..."
					class="textarea textarea-bordered flex-1 min-h-[40px] max-h-[96px] resize-none text-base sm:text-sm leading-relaxed py-[8px] sm:py-[6px]"
					rows={1}
				/>
				<input
					ref={(el) => (fileInputRef = el)}
					type="file"
					class="hidden"
					accept="image/jpeg,image/png,image/gif,image/webp,application/pdf,text/plain"
					onChange={(e) => void handleFile(e.currentTarget.files?.[0])}
				/>
				<button
					type="button"
					class="btn btn-ghost btn-square h-10 min-h-10 shrink-0"
					title="上传附件"
					disabled={uploading()}
					onClick={() => fileInputRef?.click()}
				>
					{uploading() ? (
						<span class="loading loading-spinner loading-sm" />
					) : (
						<Paperclip class="size-4" />
					)}
				</button>
				<button
					type="button"
					class="btn btn-primary h-10 min-h-10 shrink-0 px-4"
					disabled={!content().trim() || uploading()}
					onClick={handleSend}
				>
					发送
				</button>
			</div>
		</div>
	);
}
