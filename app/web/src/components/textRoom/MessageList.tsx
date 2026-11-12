import { Button } from "@gospeak/components";
import { useVirtualizer } from "@tanstack/react-virtual";
import { Loader2 } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import AsyncState from "@/components/common/AsyncState";
import { SkeletonListRows } from "@/components/common/Skeletons";
import PasswordInput from "@/components/common/passwordInput";
import { chatStore, useChatStore } from "@/stores/chatStore";
import type { MessageDTO } from "@/types/message";
import { dayKey, formatDateLabel } from "@/utils/chatTime";
import MessageItem from "./MessageItem";

interface MessageListProps {
	messages?: MessageDTO[];
	searchActive?: boolean;
	threadParent?: string | null;
	onReply?: (uuid: string) => void;
	onOpenThread?: (uuid: string) => void;
}

type Row =
	| { kind: "date"; key: string; label: string }
	| { kind: "msg"; key: string; msg: MessageDTO };

/** 在消息序列前按天插入日期分隔条；搜索结果不插。 */
function buildRows(list: MessageDTO[], withDates: boolean): Row[] {
	const rows: Row[] = [];
	let lastKey = "";
	for (const msg of list) {
		if (withDates) {
			const key = dayKey(msg.created_at);
			if (key && key !== lastKey) {
				lastKey = key;
				rows.push({
					kind: "date",
					key: `date-${key}`,
					label: formatDateLabel(msg.created_at),
				});
			}
		}
		rows.push({ kind: "msg", key: msg.uuid, msg });
	}
	return rows;
}

export default function MessageList({
	messages,
	searchActive = false,
	threadParent,
	onReply,
	onOpenThread,
}: MessageListProps) {
	const scrollRef = useRef<HTMLDivElement>(null);
	const prevScrollHeightRef = useRef(0);

	const visibleMessages: MessageDTO[] = (() => {
		if (searchActive && messages) return messages;
		if (threadParent) {
			const root = threadParent;
			const all = useChatStore.getState().messages;
			const byParent = new Map<string, MessageDTO[]>();
			for (const m of all) {
				if (!m.reply_to) continue;
				const children = byParent.get(m.reply_to) || [];
				children.push(m);
				byParent.set(m.reply_to, children);
			}
			const out: MessageDTO[] = [];
			const seen = new Set<string>();
			const append = (uuid: string) => {
				if (seen.has(uuid)) return;
				const message = all.find((m) => m.uuid === uuid);
				if (!message) return;
				seen.add(uuid);
				out.push(message);
				for (const child of byParent.get(uuid) || []) append(child.uuid);
			};
			append(root);
			return out.sort((a, b) => a.created_at.localeCompare(b.created_at));
		}
		return useChatStore.getState().messages;
	})();

	const rows = buildRows(visibleMessages, !searchActive);

	const virtualizer = useVirtualizer({
		count: rows.length,
		getScrollElement: () => scrollRef.current,
		estimateSize: (index) => (rows[index]?.kind === "date" ? 40 : 76),
		overscan: 12,
	});

	const isAtBottom = useChatStore((s) => s.isAtBottom);
	const hasMore = useChatStore((s) => s.hasMore);
	const loading = useChatStore((s) => s.loading);
	const textRoomError = useChatStore((s) => s.textRoomError);
	const needsPassword = useChatStore((s) => s.textRoomNeedsPassword);
	const [password, setPassword] = useState("");
	const [retrying, setRetrying] = useState(false);

	// biome-ignore lint/correctness/useExhaustiveDependencies: 仅按行数与是否贴底触发滚动；virtualizer/ref 稳定，加入依赖会造成滚动抖动
	useEffect(() => {
		const len = rows.length;
		if (len > 0 && isAtBottom) {
			requestAnimationFrame(() => {
				virtualizer.scrollToIndex(len - 1, { align: "end" });
			});
		}
	}, [rows.length, isAtBottom]);

	function handleScroll() {
		const el = scrollRef.current;
		if (!el) return;

		const scrollBottom = el.scrollHeight - el.scrollTop - el.clientHeight;
		useChatStore.setState({ isAtBottom: scrollBottom < 50 });

		if (
			el.scrollTop < 50 &&
			hasMore &&
			!loading &&
			!searchActive &&
			!threadParent
		) {
			prevScrollHeightRef.current = el.scrollHeight;
			void chatStore.loadMore().then(() => {
				if (prevScrollHeightRef.current > 0) {
					requestAnimationFrame(() => {
						el.scrollTop = el.scrollHeight - prevScrollHeightRef.current;
						prevScrollHeightRef.current = 0;
					});
				}
			});
		}
	}

	const items = virtualizer.getVirtualItems();

	// 密码错误 / 缺失：给一个能重新输入的入口，而不是把用户晾在空房间里
	if (needsPassword && visibleMessages.length === 0) {
		return (
			<div className="flex h-full min-h-0 flex-1 flex-col items-center justify-center gap-3 px-6">
				<p className="text-sm font-medium">这个文字房需要密码</p>
				<p className="text-muted-foreground text-xs">{textRoomError}</p>
				<form
					className="flex w-full max-w-xs gap-2"
					onSubmit={async (e) => {
						e.preventDefault();
						if (!password || retrying) return;
						setRetrying(true);
						try {
							await chatStore.retryTextRoomWithPassword(password);
							setPassword("");
						} finally {
							setRetrying(false);
						}
					}}
				>
					<PasswordInput
						value={password}
						placeholder="房间密码"
						autoComplete="off"
						onChange={(e) => setPassword(e.target.value)}
					/>
					<Button type="submit" disabled={!password || retrying}>
						{retrying ? <Loader2 className="size-4 animate-spin" /> : "进入"}
					</Button>
				</form>
			</div>
		);
	}

	return (
		<div
			ref={scrollRef}
			onScroll={handleScroll}
			className="ds-scroll min-h-0 flex-1 overflow-y-auto"
		>
			<AsyncState
				loading={loading && visibleMessages.length === 0}
				skeleton={<SkeletonListRows rows={6} />}
				error={visibleMessages.length === 0 ? textRoomError : null}
				empty={visibleMessages.length === 0 && !searchActive}
				emptyText={searchActive ? "没有匹配消息" : "暂无消息"}
				onRetry={() => void chatStore.loadInitial()}
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
									<div ref={virtualizer.measureElement} data-index={item.index}>
										{row.kind === "date" ? (
											<div className="text-faint-foreground flex items-center gap-1.5 px-4 pt-4 pb-1 text-xs font-medium select-none">
												<span>{row.label}</span>
											</div>
										) : (
											<MessageItem
												msg={row.msg}
												onReply={onReply}
												onOpenThread={onOpenThread}
											/>
										)}
									</div>
								</div>
							);
						})}
					</div>
				)}
			</AsyncState>
		</div>
	);
}
