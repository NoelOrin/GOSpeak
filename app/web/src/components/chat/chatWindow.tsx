import { useVirtualizer } from "@tanstack/react-virtual";
import { useEffect, useRef, useState } from "react";
import type { PrivateMessageDTO } from "@/api/conversation";
import AsyncState from "@/components/common/AsyncState";
import { chatStore, useChatStore } from "@/stores/chatStore";
import { useUserStore } from "@/stores/userStore";

export default function ChatWindow() {
	const active = useChatStore((s) => s.activeConversationID);
	const pmMessages = useChatStore((s) => s.pmMessages);
	const loadingMessages = useChatStore((s) => s.loadingMessages);
	const messages: PrivateMessageDTO[] = active ? pmMessages[active] || [] : [];
	const isLoading = active ? !!loadingMessages[active] : false;

	const scrollRef = useRef<HTMLDivElement>(null);
	const atBottomRef = useRef(true);
	const [content, setContent] = useState("");
	const textareaRef = useRef<HTMLTextAreaElement>(null);

	const virtualizer = useVirtualizer({
		count: messages.length,
		getScrollElement: () => scrollRef.current,
		estimateSize: () => 64,
		overscan: 10,
	});
	// effect 里只依赖消息数量/会话，避免把 virtualizer 实例放进依赖导致反复滚动
	const virtualizerRef = useRef(virtualizer);
	virtualizerRef.current = virtualizer;

	// 切换会话后一律回到最新消息
	useEffect(() => {
		atBottomRef.current = true;
	}, [active]);

	useEffect(() => {
		if (messages.length === 0 || !atBottomRef.current) return;
		requestAnimationFrame(() => {
			virtualizerRef.current.scrollToIndex(messages.length - 1, {
				align: "end",
			});
		});
	}, [messages.length, active]);

	function handleScroll() {
		const el = scrollRef.current;
		if (!el) return;
		atBottomRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 60;
	}

	function handleSend() {
		const text = content.trim();
		const id = useChatStore.getState().activeConversationID;
		if (!text || !id) return;
		chatStore.sendDirect(id, text);
		setContent("");
		atBottomRef.current = true;
		if (textareaRef.current) textareaRef.current.style.height = "auto";
	}

	function handleKeyDown(e: React.KeyboardEvent) {
		if (e.key === "Enter" && !e.shiftKey) {
			e.preventDefault();
			handleSend();
		}
	}

	function handleInput() {
		const el = textareaRef.current;
		if (!el) return;
		setContent(el.value);
		el.style.height = "auto";
		el.style.height = `${Math.min(el.scrollHeight, 96)}px`;
	}

	const isOwn = (msg: PrivateMessageDTO) => {
		const u = useUserStore.getState().user;
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

	const items = virtualizer.getVirtualItems();

	return (
		<div className="flex h-full min-w-0 flex-1 flex-col">
			<div
				ref={scrollRef}
				onScroll={handleScroll}
				className="ds-scroll flex-1 overflow-y-auto px-4 py-2"
			>
				<AsyncState
					loading={isLoading && messages.length === 0}
					loadingText="正在加载消息…"
					empty={messages.length === 0 && !isLoading}
					emptyText="发送一条消息开始对话"
				>
					<div
						style={{
							height: `${virtualizer.getTotalSize()}px`,
							position: "relative",
						}}
					>
						{items.map((item) => {
							const msg = messages[item.index];
							if (!msg) return null;
							return (
								<div
									key={item.key}
									data-index={item.index}
									ref={virtualizer.measureElement}
									style={{
										position: "absolute",
										top: 0,
										left: 0,
										width: "100%",
										transform: `translateY(${item.start}px)`,
									}}
								>
									<div
										className={`mb-2 flex flex-col ${isOwn(msg) ? "items-end" : "items-start"}`}
									>
										<div
											className={`max-w-[70%] rounded-2xl px-3 py-1.5 text-sm break-words ${
												isOwn(msg)
													? "bg-primary text-primary-foreground"
													: "bg-muted"
											}`}
										>
											{msg.deleted ? "[消息已删除]" : msg.content}
										</div>
										<span className="text-faint-foreground mt-0.5 text-2xs">
											{formatTime(msg.created_at)}
										</span>
									</div>
								</div>
							);
						})}
					</div>
				</AsyncState>
			</div>

			{active && (
				<div className="border-border shrink-0 border-t p-2">
					<div className="flex items-end gap-2">
						<textarea
							ref={textareaRef}
							value={content}
							onInput={handleInput}
							onKeyDown={handleKeyDown}
							placeholder="输入消息..."
							className="border-input focus-visible:ring-ring placeholder:text-muted-foreground max-h-[96px] min-h-[40px] w-full flex-1 resize-none rounded-sm border bg-transparent px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2"
							rows={1}
						/>
						<button
							type="button"
							className="bg-primary text-primary-foreground hover:opacity-90 inline-flex h-9 shrink-0 cursor-pointer items-center rounded-sm px-3 text-sm font-medium transition-opacity disabled:pointer-events-none disabled:opacity-50"
							disabled={!content.trim()}
							onClick={handleSend}
						>
							发送
						</button>
					</div>
				</div>
			)}
		</div>
	);
}
