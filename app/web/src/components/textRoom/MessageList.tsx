import { createVirtualizer } from "@tanstack/solid-virtual";
import { createEffect, For, Show } from "solid-js";
import { chatStore } from "@/stores/chatStore";
import MessageItem from "./MessageItem";

export default function MessageList() {
	let scrollRef!: HTMLDivElement;
	let prevScrollHeight = 0;

	const virtualizer = createVirtualizer({
		get count() {
			return chatStore.messages().length;
		},
		getScrollElement: () => scrollRef,
		estimateSize: () => 72,
		overscan: 12,
	});

	// Auto-scroll to bottom on new messages when user is at bottom
	createEffect(() => {
		const msgs = chatStore.messages();
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

		// Load more when near top
		if (el.scrollTop < 50 && chatStore.hasMore() && !chatStore.loading()) {
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
			<div
				style={{
					height: `${virtualizer.getTotalSize()}px`,
					position: "relative",
				}}
			>
				<For each={items()}>
					{(item) => (
						<Show when={chatStore.messages()[item.index]}>
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
									<MessageItem msg={chatStore.messages()[item.index]} />
								</div>
							</div>
						</Show>
					)}
				</For>
			</div>
		</div>
	);
}
