import { toastActionError } from "@/utils/toast";
import {
	CornerDownLeft,
	CornerDownRight,
	CirclePlus,
	Maximize2,
	Smile,
	X,
} from "lucide-react";
import { useRef, useState } from "react";
import { useTypingReporter } from "@/hooks/useTypingReporter";
import { useUpload } from "@/hooks/useUpload";
import { chatStore, useChatStore } from "@/stores/chatStore";
import { useSocketStore } from "@/stores/socketStore";
import { useTextRoomUiStore } from "@/stores/textRoomUiStore";
import { useWorkspaceUiStore } from "@/stores/workspaceUiStore";

interface UploadingFile {
	uploadId: string;
	name: string;
}

export default function MessageInput() {
	const [content, setContent] = useState("");
	const [mentions, setMentions] = useState<string[]>([]);
	const [mentionOpen, setMentionOpen] = useState(false);
	const [mentionQuery, setMentionQuery] = useState("");
	const [mentionIndex, setMentionIndex] = useState(0);
	const [uploadingFiles, setUploadingFiles] = useState<UploadingFile[]>([]);
	const { upload, uploading, progress } = useUpload("chat");
	const textareaRef = useRef<HTMLTextAreaElement>(null);
	const fileInputRef = useRef<HTMLInputElement>(null);

	const replyTo = useTextRoomUiStore((s) => s.replyTo);
	const threadParent = useTextRoomUiStore((s) => s.threadParent);
	const setReplyTo = useTextRoomUiStore((s) => s.setReplyTo);
	const roomName = useChatStore((s) => s.textRoomName);
	const domainUUID = useChatStore((s) => s.textRoomDomainUUID);
	const chatFullscreen = useWorkspaceUiStore((s) => s.chatFullscreen);
	const toggleChatFullscreen = useWorkspaceUiStore(
		(s) => s.toggleChatFullscreen,
	);
	const { notify, stop } = useTypingReporter(roomName, domainUUID);

	const members = (() => {
		const room = useSocketStore
			.getState()
			.rooms.find((r) => r.name === roomName);
		return room?.members ?? [];
	})();

	const mentionCandidates = () => {
		const q = mentionQuery.toLowerCase();
		return members
			.map((m) => m.identity || m.name)
			.filter((name, index, arr) => name && arr.indexOf(name) === index)
			.filter((name) => !q || name.toLowerCase().includes(q))
			.slice(0, 8);
	};

	const replyTarget = threadParent || replyTo;

	function resetTextareaHeight() {
		const el = textareaRef.current;
		if (!el) return;
		el.style.height = "auto";
		el.style.height = `${Math.min(el.scrollHeight, 144)}px`;
	}

	function resetAfterSend() {
		setContent("");
		setMentions([]);
		setMentionOpen(false);
		setReplyTo(null);
		requestAnimationFrame(() => {
			if (textareaRef.current) textareaRef.current.style.height = "auto";
		});
	}

	function handleSend() {
		const text = content.trim();
		if (!text || uploading) return;

		const opts: { reply_to?: string; mentions?: string[] } = {};
		if (replyTarget) opts.reply_to = replyTarget;
		if (mentions.length > 0) opts.mentions = mentions;

		chatStore.send(text, opts);
		resetAfterSend();
		stop();
	}

	function applyMention(name: string) {
		const el = textareaRef.current;
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
		const el = textareaRef.current;
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

		// 只节流上报，不节流 setContent（否则会丢字）
		if (el.value.trim()) notify();
		else stop();
	}

	function handleKeyDown(e: React.KeyboardEvent) {
		const candidates = mentionCandidates();
		if (mentionOpen && candidates.length > 0) {
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
				applyMention(candidates[mentionIndex] || candidates[0]);
				return;
			}
		}
		if (e.key === "Enter" && !e.shiftKey) {
			e.preventDefault();
			handleSend();
		}
	}

	async function handleFiles(files: FileList | File[] | undefined) {
		const list = files ? Array.from(files) : [];
		if (list.length === 0) return;

		setUploadingFiles((prev) => [
			...prev,
			...list.map((file) => ({
				uploadId: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
				name: file.name,
			})),
		]);

		for (const file of list) {
			try {
				const result = await upload(file);
				const token = file.type.startsWith("image/")
					? `![${file.name}](${result.public_url})`
					: `[${file.name}](${result.public_url})`;
				setContent((prev) => `${prev.trimEnd()}\n${token}\n`);
			} catch (err) {
				toastActionError(err, "上传失败");
			} finally {
				setUploadingFiles((prev) => prev.slice(1));
			}
		}

		requestAnimationFrame(() => {
			if (textareaRef.current) {
				textareaRef.current.focus();
				resetTextareaHeight();
			}
		});
		if (fileInputRef.current) fileInputRef.current.value = "";
	}

	const iconBtn =
		"ds-icon-btn size-8 text-muted-foreground hover:text-foreground disabled:pointer-events-none disabled:opacity-40";

	return (
		<div
			role="region"
			className="border-border bg-card relative shrink-0 border-t px-4 pt-2 pb-3"
			onDragOver={(e) => e.preventDefault()}
			onDrop={(e) => {
				e.preventDefault();
				if (e.dataTransfer?.files) void handleFiles(e.dataTransfer.files);
			}}
		>
			{mentionOpen && mentionCandidates().length > 0 && (
				<div className="bg-popover border-border absolute bottom-full left-4 z-20 mb-2 w-56 overflow-hidden rounded-lg border shadow-xl">
					{mentionCandidates().map((name, index) => (
						<button
							key={name}
							type="button"
							className={`text-foreground hover:bg-muted block w-full cursor-pointer px-3 py-2 text-left text-sm transition-colors ${index === mentionIndex ? "bg-selected" : ""}`}
							onMouseDown={(e) => e.preventDefault()}
							onClick={() => applyMention(name)}
						>
							@{name}
						</button>
					))}
				</div>
			)}

			{uploadingFiles.length > 0 && (
				<div className="mb-1 space-y-1">
					{uploadingFiles.map((file) => (
						<div
							key={file.uploadId}
							className="text-muted-foreground flex items-center gap-2 px-1 py-1 text-xs"
						>
							<span className="border-border text-foreground size-3 animate-spin rounded-full border-2 border-t-transparent" />
							<span className="min-w-0 flex-1 truncate">{file.name}</span>
							<span>{progress}%</span>
						</div>
					))}
				</div>
			)}

			{replyTarget && (
				<div className="text-muted-foreground mb-1 flex items-center gap-2 px-1 text-xs">
					<CornerDownRight size={14} className="shrink-0" />
					<span className="truncate">
						{threadParent ? "回复到线程" : "回复中..."}
					</span>
					<button
						type="button"
						className="hover:text-foreground ml-auto shrink-0 cursor-pointer transition-colors"
						onClick={() => setReplyTo(null)}
						aria-label="取消回复"
					>
						<X size={14} />
					</button>
				</div>
			)}

			<div className="flex items-end gap-2">
				<textarea
					ref={textareaRef}
					value={content}
					onInput={handleInput}
					onKeyDown={handleKeyDown}
					onBlur={stop}
					placeholder={`发送至频道 ${roomName ?? "文字频道"}`}
					disabled={false}
					className="text-foreground placeholder:text-faint-foreground max-h-36 min-h-[36px] w-full flex-1 resize-none bg-transparent py-2 text-sm leading-5 outline-none disabled:cursor-not-allowed disabled:opacity-50"
					rows={1}
				/>

				<div className="flex shrink-0 items-center gap-0.5 pb-1">
					<button
						type="button"
						className={iconBtn}
						title="发送"
						disabled={!content.trim() || uploading}
						onClick={handleSend}
					>
						<CornerDownLeft size={20} />
					</button>
					<button
						type="button"
						className={iconBtn}
						title="插入表情"
						onClick={() => {
							setContent((prev) => `${prev}😊`);
							textareaRef.current?.focus();
						}}
					>
						<Smile size={20} />
					</button>
					<button
						type="button"
						className={iconBtn}
						title="上传附件"
						disabled={uploading}
						onClick={() => fileInputRef.current?.click()}
					>
						{uploading ? (
							<span className="border-border border-t-foreground size-4 animate-spin rounded-full border-2" />
						) : (
							<CirclePlus size={20} />
						)}
					</button>
					<button
						type="button"
						className={iconBtn}
						data-active={chatFullscreen}
						title={chatFullscreen ? "退出全屏" : "全屏"}
						aria-pressed={chatFullscreen}
						onClick={toggleChatFullscreen}
					>
						<Maximize2 size={20} />
					</button>
				</div>
			</div>

			<input
				ref={fileInputRef}
				type="file"
				className="hidden"
				multiple
				accept="image/jpeg,image/png,image/gif,image/webp,application/pdf,text/plain"
				onChange={(e) => void handleFiles(e.currentTarget.files ?? undefined)}
			/>
		</div>
	);
}
