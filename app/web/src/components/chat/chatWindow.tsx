import { useVirtualizer } from "@tanstack/react-virtual";
import { CornerDownLeft, MessageSquareText, Smile } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import type { PrivateMessageDTO } from "@/api/conversation";
import Avatar from "@/components/common/avatar";
import AsyncState from "@/components/common/AsyncState";
import { SkeletonListRows } from "@/components/common/Skeletons";
import { chatStore, useChatStore } from "@/stores/chatStore";
import { dayKey, formatDateLabel, formatMessageTime } from "@/utils/chatTime";

type Row =
	| { kind: "date"; key: string; label: string }
	| { kind: "msg"; key: string; msg: PrivateMessageDTO };

function buildRows(list: PrivateMessageDTO[]): Row[] {
	const rows: Row[] = [];
	let lastKey = "";
	for (const msg of list) {
		const key = dayKey(msg.created_at);
		if (key && key !== lastKey) {
			lastKey = key;
			rows.push({
				kind: "date",
				key: `date-${key}`,
				label: formatDateLabel(msg.created_at),
			});
		}
		rows.push({ kind: "msg", key: msg.uuid, msg });
	}
	return rows;
}

export default function ChatWindow() {
	const active = useChatStore((s) => s.activeConversationID);
	const conversations = useChatStore((s) => s.conversations);
	const pmMessages = useChatStore((s) => s.pmMessages);
	const loadingMessages = useChatStore((s) => s.loadingMessages);
	const messages: PrivateMessageDTO[] = active ? pmMessages[active] || [] : [];
	const isLoading = active ? !!loadingMessages[active] : false;

	const activeConversation = conversations.find(
		(c) => c.conversation_id === active,
	);
	const peerName =
		activeConversation?.other_display_name ||
		activeConversation?.other_identity ||
		"私聊";

	const rows = buildRows(messages);

	const scrollRef = useRef<HTMLDivElement>(null);
	const atBottomRef = useRef(true);
	const [content, setContent] = useState("");
	const textareaRef = useRef<HTMLTextAreaElement>(null);

	const virtualizer = useVirtualizer({
		count: rows.length,
		getScrollElement: () => scrollRef.current,
		estimateSize: (index) => (rows[index]?.kind === "date" ? 40 : 72),
		overscan: 10,
	});
	// effect 里只依赖消息数量/会话，避免把 virtualizer 实例放进依赖导致反复滚动
	const virtualizerRef = useRef(virtualizer);
	virtualizerRef.current = virtualizer;

	// 切换会话后一律回到最新消息
	useEffect(() => {
		atBottomRef.current = true;
	}, [active]);

	// biome-ignore lint/correctness/useExhaustiveDependencies: 仅按行数/会话触发贴底滚动，virtualizer 实例稳定
	useEffect(() => {
		if (rows.length === 0 || !atBottomRef.current) return;
		requestAnimationFrame(() => {
			virtualizerRef.current.scrollToIndex(rows.length - 1, {
				align: "end",
			});
		});
	}, [rows.length, active]);

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

	const items = virtualizer.getVirtualItems();

	const iconBtn =
		"ds-icon-btn size-8 text-muted-foreground hover:text-foreground disabled:pointer-events-none disabled:opacity-40";

	return (
		<div className="bg-card flex h-full min-w-0 flex-1 flex-col">
			{/* 面板标题栏：图标 + 对方名称 */}
			<header className="border-border flex h-12 shrink-0 items-center gap-2 border-b px-3">
				<span className="bg-muted text-foreground flex size-7 shrink-0 items-center justify-center rounded-sm">
					<MessageSquareText size={16} />
				</span>
				<span className="text-foreground truncate text-sm font-semibold">
					{peerName}
				</span>
			</header>

			<div
				ref={scrollRef}
				onScroll={handleScroll}
				className="ds-scroll min-h-0 flex-1 overflow-y-auto"
			>
				<AsyncState
					loading={isLoading && messages.length === 0}
					loadingText="正在加载消息…"
					skeleton={<SkeletonListRows rows={6} className="max-w-2xl" />}
					empty={messages.length === 0 && !isLoading}
					emptyText="发送一条消息开始对话"
				>
					{rows.length > 0 && (
						<div
							style={{
								height: `${virtualizer.getTotalSize()}px`,
								position: "relative",
							}}
						>
							{items.map((item) => {
								const row = rows[item.index];
								if (!row) return null;
								return (
									<div
										key={item.key}
										style={{
											position: "absolute",
											top: 0,
											left: 0,
											width: "100%",
											transform: `translateY(${item.start}px)`,
										}}
									>
										<div
											ref={virtualizer.measureElement}
											data-index={item.index}
										>
											{row.kind === "date" ? (
												<div className="text-faint-foreground flex items-center gap-1.5 px-4 pt-4 pb-1 text-xs font-medium select-none">
													<span>{row.label}</span>
												</div>
											) : (
												<div className="hover:bg-muted flex gap-3 px-4 py-1">
													<Avatar
														name={row.msg.author_id}
														alt={row.msg.author_id}
														className="mt-0.5 size-9 shrink-0"
														textClass="text-xs"
													/>
													<div className="min-w-0 flex-1">
														<div className="flex items-baseline gap-2">
															<span className="text-foreground truncate text-sm font-semibold">
																{row.msg.author_id || "?"}
															</span>
															<span className="text-faint-foreground shrink-0 text-2xs">
																{formatMessageTime(row.msg.created_at)}
															</span>
														</div>
														<div className="text-foreground text-sm break-words">
															{row.msg.deleted
																? "[消息已删除]"
																: row.msg.content}
														</div>
													</div>
												</div>
											)}
										</div>
									</div>
								);
							})}
						</div>
					)}
				</AsyncState>
			</div>

			{active && (
				<div className="bg-card shrink-0 px-4 pt-1 pb-3">
					<div className="flex items-end gap-2">
						<textarea
							ref={textareaRef}
							value={content}
							onInput={handleInput}
							onKeyDown={handleKeyDown}
							placeholder={`发送至 ${peerName}`}
							className="text-foreground placeholder:text-faint-foreground max-h-24 min-h-[36px] w-full flex-1 resize-none bg-transparent py-2 text-sm leading-5 outline-none"
							rows={1}
						/>
						<div className="flex shrink-0 items-center gap-0.5 pb-1">
							<button
								type="button"
								className={iconBtn}
								title="发送"
								disabled={!content.trim()}
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
						</div>
					</div>
				</div>
			)}
		</div>
	);
}
