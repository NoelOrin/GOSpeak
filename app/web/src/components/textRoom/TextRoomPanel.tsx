import { CornerDownRight, Hash } from "lucide-react";
import { useChatStore } from "@/stores/chatStore";
import { useTextRoomUiStore } from "@/stores/textRoomUiStore";
import MessageInput from "./MessageInput";
import MessageList from "./MessageList";
import TypingIndicator from "./TypingIndicator";

/**
 * 文字频道面板（Discord 版）：标题栏由 ChannelHeader 统一托管，
 * 这里只负责频道欢迎区、消息列表与输入框，因此主舞台和聊天侧栏都能复用。
 */
export default function TextRoomPanel() {
	const textRoom = useChatStore((s) => s.textRoom);
	const textRoomName = useChatStore((s) => s.textRoomName);
	const hasMore = useChatStore((s) => s.hasMore);
	const messageCount = useChatStore((s) => s.messages.length);

	const searchActive = useTextRoomUiStore((s) => s.searchActive);
	const searchResults = useTextRoomUiStore((s) => s.searchResults);
	const threadParent = useTextRoomUiStore((s) => s.threadParent);
	const setReplyTo = useTextRoomUiStore((s) => s.setReplyTo);
	const openThread = useTextRoomUiStore((s) => s.openThread);
	const exitThread = useTextRoomUiStore((s) => s.exitThread);

	if (!textRoom) {
		return (
			<div className="text-muted-foreground flex h-full flex-col items-center justify-center gap-2 text-sm select-none">
				<Hash size={32} strokeWidth={1.5} />
				<span>从左侧选择一个文字频道开始聊天</span>
			</div>
		);
	}

	const showIntro =
		!hasMore && !searchActive && !threadParent && messageCount <= 20;

	return (
		<div className="bg-card flex h-full min-h-0 flex-col">
			{searchActive && (
				<div className="border-border text-muted-foreground border-b px-4 py-1.5 text-xs">
					找到 {searchResults.length} 条匹配消息
				</div>
			)}
			{threadParent && (
				<div className="border-border text-muted-foreground flex shrink-0 items-center gap-2 border-b px-4 py-1.5 text-xs">
					<CornerDownRight size={14} />
					<span>线程视图</span>
					<button
						type="button"
						className="text-foreground hover:underline ml-auto cursor-pointer"
						onClick={exitThread}
					>
						退出线程
					</button>
				</div>
			)}

			{showIntro && (
				<div className="shrink-0 px-4 pt-4 pb-2">
					<div className="bg-muted mb-2 flex size-11 items-center justify-center rounded-full">
						<Hash size={24} strokeWidth={2} className="text-foreground" />
					</div>
					<h1 className="text-foreground text-xl leading-tight font-bold">
						欢迎来到 #{textRoomName}！
					</h1>
					<p className="text-muted-foreground mt-0.5 text-sm">
						这是{" "}
						<strong className="text-foreground font-semibold">
							#{textRoomName}
						</strong>{" "}
						频道的开始。
					</p>
				</div>
			)}

			<MessageList
				messages={searchActive ? searchResults : undefined}
				searchActive={searchActive}
				threadParent={threadParent}
				onReply={(uuid) => setReplyTo(threadParent ?? uuid)}
				onOpenThread={openThread}
			/>

			<TypingIndicator />

			<MessageInput />
		</div>
	);
}
