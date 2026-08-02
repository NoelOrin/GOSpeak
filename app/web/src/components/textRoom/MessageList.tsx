import { createVirtualizer } from "@tanstack/solid-virtual";
import { createEffect, createMemo, For, Show } from "solid-js";
import { chatStore } from "@/stores/chatStore";
import type { MessageDTO } from "@/types/message";
import MessageItem from "./MessageItem";

interface MessageListProps {
	messages?: MessageDTO[];
	searchActive?: boolean;
	threadParent?: string | null;
	onReply?: (uuid: string) => void;
	onOpenThread?: (uuid: string) => void;
}

export default function MessageList(props: MessageListProps) {
	let scrollRef!: HTMLDivElement;
	let prevScrollHeight = 0;

	const visibleMessages = createMemo<MessageDTO[]>(() => {
		if (props.searchActive && props.messages) return props.messages;
		if (props.threadParent) {
			const root = props.threadParent;
			const messages = chatStore.messages();
			const byParent = new Map<string, MessageDTO[]>();
			for (const m of messages) {
				if (!m.reply_to) continue;
				const children = byParent.get(m.reply_to) || [];
				children.push(m);
				byParent.set(m.reply_to, children);
			}
			const out: MessageDTO[] = [];
			const seen = new Set<string>();
			const append = (uuid: string) => {
				if (seen.has(uuid)) return;
				const message = messages.find((m) => m.uuid === uuid);
				if (!message) return;
				seen.add(uuid);
				out.push(message);
				for (const child of byParent.get(uuid) || []) append(child.uuid);
			};
			append(root);
			return out.sort((a, b) => a.created_at.localeCompare(b.created_at));
		}
		return chatStore.messages();
	});

	const virtualizer = createVirtualizer({
		get count() {
			return visibleMessages().length;
		},
		getScrollElement: () => scrollRef,
		estimateSize: () => 72,
		overscan: 12,
	});

	createEffect(() => {
		const msgs = visibleMessages();
		const len = msgs.length;
		if (len > 0 && chatStore.isAtBottom()) {
			requestAnimationFrame(() => {
				virtualizer.scrollToIndex(len - 1, { align: "end" });
			});
		}
	});

	function handleScroll() {
		const el = scrollRef;
		if (!el) return;

		const scrollBottom = el.scrollHeight - el.scrollTop - el.clientHeight;
		chatStore.setIsAtBottom(scrollBottom < 50);

		if (
			el.scrollTop < 50 &&
			chatStore.hasMore() &&
			!chatStore.loading() &&
			!props.searchActive &&
			!props.threadParent
		) {
			prevScrollHeight = el.scrollHeight;
			chatStore.loadMore().then(() => {
				if (prevScrollHeight > 0) {
					requestAnimationFrame(() => {
						el.scrollTop = el.scrollHeight - prevScrollHeight;
						prevScrollHeight = 0;
					});
				}
			});
		}
	}

	const items = () => virtualizer.getVirtualItems();

	return (
		<div ref={scrollRef} onScroll={handleScroll} class="flex-1 overflow-y-auto">
			<Show
				when={visibleMessages().length > 0}
				fallback={
					<div class="flex items-center justify-center h-full text-sm text-base-content/40 select-none">
						{props.searchActive ? "没有匹配消息" : "暂无消息"}
					</div>
				}
			>
				<div
					style={{
						height: `${virtualizer.getTotalSize()}px`,
						position: "relative",
					}}
				>
					<For each={items()}>
						{(item) => (
							<Show when={visibleMessages()[item.index]}>
								<div
									style={{
										position: "absolute",
										top: 0,
										left: 0,
										width: "100%",
										transform: `translateY(${item.start}px)`,
									}}
								>
									<div
										ref={(el) => {
											if (el) virtualizer.measureElement(el);
										}}
										data-index={item.index}
									>
										<MessageItem
											msg={visibleMessages()[item.index]}
											onReply={props.onReply}
											onOpenThread={props.onOpenThread}
										/>
									</div>
								</div>
							</Show>
						)}
					</For>
				</div>
			</Show>
		</div>
	);
}
