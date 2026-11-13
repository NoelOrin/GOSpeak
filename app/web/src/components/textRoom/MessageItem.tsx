import { CornerDownRight, MessageSquarePlus, Trash2 } from "lucide-react";
import { useState } from "react";
import Avatar from "@/components/common/avatar";
import ConfirmModal from "@/components/common/ConfirmModal";
import { chatStore, useChatStore } from "@/stores/chatStore";
import { useUserStore } from "@/stores/userStore";
import type { MessageDTO } from "@/types/message";
import { formatMessageTime } from "@/utils/chatTime";
import MarkdownText from "./MarkdownText";
import ReactionBar from "./ReactionBar";

interface MessageItemProps {
	msg: MessageDTO;
	onReply?: (uuid: string) => void;
	onOpenThread?: (uuid: string) => void;
}

/** Discord 风格消息行：左侧 40px 头像、用户名+时间戳、hover 浮出操作条 */
export default function MessageItem({
	msg,
	onReply,
	onOpenThread,
}: MessageItemProps) {
	const user = useUserStore((s) => s.user);
	const [confirmDelete, setConfirmDelete] = useState(false);

	const isOwn =
		!!user &&
		(msg.author_id === user.name || msg.author_id === user.display_name);
	const displayName = msg.author_name || msg.author_id || "?";
	const avatarUrl = msg.author_avatar || "";

	const replyMessage = msg.reply_to
		? chatStore.messages().find((m) => m.uuid === msg.reply_to)
		: undefined;

	const time = formatMessageTime(msg.created_at);

	if (msg.deleted) {
		return (
			<div className="text-muted-foreground px-4 py-1 text-sm italic select-none">
				[消息已删除]
			</div>
		);
	}

	const isPending = msg._status === "pending";
	const isFailed = msg._status === "failed";

	return (
		<div
			className={`hover:bg-muted group relative py-1 pr-4 pl-[72px] leading-[1.375rem] ${
				isFailed ? "bg-destructive/5" : ""
			}`}
		>
			{replyMessage && (
				<button
					type="button"
					className="text-muted-foreground hover:text-foreground mb-0.5 flex h-5 w-full cursor-pointer items-center gap-1.5 truncate text-left text-xs"
					onClick={() => onOpenThread?.(replyMessage.uuid)}
				>
					<CornerDownRight className="size-3 shrink-0" />
					<span className="text-foreground shrink-0 font-semibold">
						{replyMessage.author_name || replyMessage.author_id}
					</span>
					<span className="truncate">
						{replyMessage.deleted ? "[消息已删除]" : replyMessage.content}
					</span>
				</button>
			)}

			<Avatar
				src={avatarUrl || undefined}
				name={displayName}
				alt={displayName}
				className="absolute top-1 left-4 size-10"
				textClass="text-sm"
			/>

			<div className="flex items-baseline gap-2">
				<span className="text-foreground hover:underline cursor-pointer text-sm font-semibold">
					{displayName}
				</span>
				<span className="text-muted-foreground text-2xs">{time}</span>
				{isPending && (
					<span className="text-muted-foreground text-2xs">发送中…</span>
				)}
				{msg.edited_at && (
					<span className="text-muted-foreground text-2xs italic">
						(已编辑)
					</span>
				)}
			</div>

			<div className={isPending ? "opacity-60" : undefined}>
				<MarkdownText
					text={msg.content}
					className="ds-markdown"
					selfName={user?.name}
				/>
			</div>

			{isFailed && (
				<div className="text-destructive flex items-center gap-2 pt-0.5 text-xs">
					<span>{msg._error || "发送失败"}</span>
					<button
						type="button"
						className="text-primary cursor-pointer underline underline-offset-2"
						onClick={() => chatStore.retryMessage(msg.uuid)}
					>
						重试
					</button>
					<button
						type="button"
						className="text-muted-foreground hover:text-foreground cursor-pointer underline underline-offset-2"
						onClick={() =>
							useChatStore.setState((prev) => ({
								messages: prev.messages.filter((m) => m.uuid !== msg.uuid),
							}))
						}
					>
						删除
					</button>
				</div>
			)}

			<div className="bg-card border-border invisible absolute -top-4 right-4 z-10 flex items-center gap-0.5 rounded-sm border p-0.5 shadow-md group-hover:visible">
				<ReactionBar roomUuid={msg.room_uuid} messageUuid={msg.uuid} />
				<button
					type="button"
					className="ds-icon-btn size-7"
					onClick={() => onReply?.(msg.uuid)}
					title="回复"
				>
					<MessageSquarePlus size={16} />
				</button>
				<button
					type="button"
					className="ds-icon-btn size-7"
					onClick={() => onOpenThread?.(msg.uuid)}
					title="打开线程"
				>
					<CornerDownRight size={16} />
				</button>
				{isOwn && (
					<button
						type="button"
						className="ds-icon-btn hover:text-destructive size-7"
						onClick={() => setConfirmDelete(true)}
						title="删除消息"
					>
						<Trash2 size={14} />
					</button>
				)}
			</div>

			<ConfirmModal
				open={confirmDelete}
				title="删除消息"
				message="确认删除这条消息？删除后其他成员也将无法看到，且不能撤销。"
				confirmText="删除"
				onClose={() => setConfirmDelete(false)}
				onConfirm={() => {
					setConfirmDelete(false);
					chatStore.deleteMessage(msg.uuid);
				}}
			/>
		</div>
	);
}
