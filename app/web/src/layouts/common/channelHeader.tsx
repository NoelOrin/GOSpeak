import {
	Hash,
	Headphones,
	MessageSquare,
	Search,
	Users,
	Volume2,
	X,
} from "lucide-react";
import { useChatStore } from "@/stores/chatStore";
import { useTextRoomUiStore } from "@/stores/textRoomUiStore";
import { useWorkspaceUiStore } from "@/stores/workspaceUiStore";

interface ChannelHeaderProps {
	kind: "voice" | "text" | "empty";
	name: string;
	description?: string;
}

/** Discord 的 48px 频道标题栏：横跨主舞台与右侧侧栏 */
export default function ChannelHeader({
	kind,
	name,
	description,
}: ChannelHeaderProps) {
	const chatOpen = useWorkspaceUiStore((s) => s.chatOpen);
	const membersOpen = useWorkspaceUiStore((s) => s.membersOpen);
	const toggleChat = useWorkspaceUiStore((s) => s.toggleChat);
	const toggleMembers = useWorkspaceUiStore((s) => s.toggleMembers);
	const textRoom = useChatStore((s) => s.textRoom);
	const textRoomName = useChatStore((s) => s.textRoomName);
	const query = useTextRoomUiStore((s) => s.query);
	const setQuery = useTextRoomUiStore((s) => s.setQuery);
	const runSearch = useTextRoomUiStore((s) => s.runSearch);
	const clearSearch = useTextRoomUiStore((s) => s.clearSearch);
	const searchActive = useTextRoomUiStore((s) => s.searchActive);
	const searching = useTextRoomUiStore((s) => s.searching);
	const threadParent = useTextRoomUiStore((s) => s.threadParent);
	const exitThread = useTextRoomUiStore((s) => s.exitThread);

	const Icon = kind === "voice" ? Volume2 : kind === "text" ? Hash : Headphones;

	return (
		<header className="bg-card border-border relative z-10 flex h-10 shrink-0 items-center gap-2 border-b px-3">
			<Icon size={20} className="text-muted-foreground shrink-0" />
			<span className="text-foreground truncate text-sm font-semibold">
				{name}
			</span>
			{description && (
				<>
					<span className="bg-border mx-1 hidden h-5 w-px shrink-0 sm:block" />
					<span className="text-muted-foreground hidden truncate text-xs sm:block">
						{description}
					</span>
				</>
			)}

			<div className="ml-auto flex shrink-0 items-center gap-1">
				{threadParent && (
					<button
						type="button"
						className="bg-selected text-foreground mr-1 inline-flex h-7 cursor-pointer items-center gap-1 rounded-sm px-2 text-xs"
						onClick={exitThread}
						title="退出线程视图"
					>
						线程视图 <X size={12} />
					</button>
				)}

				{textRoom && (
					<form
						className="border border-border bg-muted focus-within:w-60 flex h-6 w-36 items-center gap-1 rounded-sm px-1.5 transition-all duration-150"
						onSubmit={(e) => {
							e.preventDefault();
							void runSearch();
						}}
					>
						<input
							type="search"
							value={query}
							placeholder="搜索"
							className="text-foreground placeholder:text-muted-foreground min-w-0 flex-1 bg-transparent text-xs outline-none"
							onChange={(e) => setQuery(e.currentTarget.value)}
							onKeyDown={(e) => {
								// Esc 清空并退出搜索结果，不必去点那个小叉
								if (e.key === "Escape") {
									e.preventDefault();
									setQuery("");
									clearSearch();
								}
							}}
						/>
						{searchActive ? (
							<button
								type="button"
								className="text-muted-foreground hover:text-foreground flex size-4 shrink-0 cursor-pointer items-center justify-center"
								onClick={clearSearch}
								aria-label="清除搜索"
							>
								{searching ? (
									<span className="border-border border-t text-foreground size-3 animate-spin rounded-full border-2" />
								) : (
									<X size={12} />
								)}
							</button>
						) : (
							<Search size={14} className="text-muted-foreground shrink-0" />
						)}
					</form>
				)}

				{textRoom && (
					<button
						type="button"
						className="ds-icon-btn"
						data-active={chatOpen}
						onClick={toggleChat}
						title="显示/隐藏文字聊天"
						aria-label="显示/隐藏文字聊天"
					>
						<MessageSquare size={20} />
					</button>
				)}
				<button
					type="button"
					className="ds-icon-btn"
					data-active={membersOpen}
					onClick={toggleMembers}
					title="显示成员列表"
					aria-label="显示成员列表"
				>
					<Users size={20} />
				</button>
			</div>
			{/* 供无障碍与后续扩展：标题栏当前频道标识 */}
			<span className="sr-only">{textRoomName ?? name}</span>
		</header>
	);
}
