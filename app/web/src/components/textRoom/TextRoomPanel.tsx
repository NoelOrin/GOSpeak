import Search from "lucide-solid/icons/search";
import X from "lucide-solid/icons/x";
import { createSignal, Show } from "solid-js";
import { searchMessages } from "@/api/message";
import { chatStore } from "@/stores/chatStore";
import type { MessageDTO } from "@/types/message";
import MessageInput from "./MessageInput";
import MessageList from "./MessageList";

export default function TextRoomPanel() {
	const [replyTo, setReplyTo] = createSignal<string | null>(null);
	const [threadParent, setThreadParent] = createSignal<string | null>(null);
	const [query, setQuery] = createSignal("");
	const [searchResults, setSearchResults] = createSignal<MessageDTO[]>([]);
	const [searchActive, setSearchActive] = createSignal(false);
	const [searching, setSearching] = createSignal(false);

	async function runSearch(e?: Event) {
		e?.preventDefault();
		const room = chatStore.textRoom();
		const q = query().trim();
		if (!room || !q) {
			setSearchActive(false);
			setSearchResults([]);
			return;
		}
		setSearching(true);
		try {
			setSearchResults(await searchMessages(room, q));
			setSearchActive(true);
		} finally {
			setSearching(false);
		}
	}

	function clearSearch() {
		setQuery("");
		setSearchActive(false);
		setSearchResults([]);
	}

	return (
		<Show
			when={chatStore.textRoom()}
			fallback={
				<div class="flex items-center justify-center h-full text-base-content/40 text-sm select-none">
					选择文字房间开始聊天
				</div>
			}
		>
			<div class="flex flex-col h-full">
				<div class="flex items-center gap-2 border-b border-base-300 px-3 py-2 shrink-0">
					<span class="font-semibold text-sm truncate">
						# {chatStore.textRoomName()}
					</span>
					<Show when={threadParent()}>
						<span class="badge badge-sm badge-outline shrink-0">线程视图</span>
					</Show>
					<form
						class="ml-auto flex items-center gap-1 min-w-0"
						onSubmit={runSearch}
					>
						<div class="relative min-w-[120px] sm:w-64">
							<Search class="absolute left-2 top-1/2 -translate-y-1/2 size-3.5 text-base-content/40" />
							<input
								type="search"
								value={query()}
								placeholder="搜索消息"
								class="input input-sm input-bordered w-full pl-7 pr-7 text-sm"
								onInput={(e) => setQuery(e.currentTarget.value)}
							/>
							<Show when={searchActive()}>
								<button
									type="button"
									class="absolute right-1 top-1/2 -translate-y-1/2 btn btn-xs btn-ghost btn-square"
									onClick={clearSearch}
								>
									<X class="size-3" />
								</button>
							</Show>
						</div>
						<button
							type="submit"
							class="btn btn-sm btn-ghost"
							disabled={searching()}
						>
							{searching() ? "搜索中..." : "搜索"}
						</button>
					</form>
					<Show when={threadParent()}>
						<button
							type="button"
							class="btn btn-sm btn-ghost shrink-0"
							onClick={() => setThreadParent(null)}
						>
							退出线程
						</button>
					</Show>
				</div>

				<Show when={searchActive()}>
					<div class="px-3 py-1.5 text-xs text-base-content/60 bg-base-200/60 border-b border-base-300">
						找到 {searchResults().length} 条匹配消息
					</div>
				</Show>

				<MessageList
					messages={searchActive() ? searchResults() : undefined}
					searchActive={searchActive()}
					threadParent={threadParent()}
					onReply={(uuid) => {
						setThreadParent(null);
						setReplyTo(uuid);
					}}
					onOpenThread={(uuid) => {
						setSearchActive(false);
						setThreadParent(uuid);
					}}
				/>
				<MessageInput
					replyTo={replyTo()}
					onCancelReply={() => setReplyTo(null)}
				/>
			</div>
		</Show>
	);
}
