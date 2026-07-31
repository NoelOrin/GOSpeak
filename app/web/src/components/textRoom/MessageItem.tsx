import CornerDownRight from "lucide-solid/icons/corner-down-right";
import MessageSquarePlus from "lucide-solid/icons/message-square-plus";
import { Show } from "solid-js";
import { chatStore } from "@/stores/chatStore";
import userStore from "@/stores/userStore";
import type { MessageDTO } from "@/types/message";
import MarkdownText from "./MarkdownText";
import ReactionBar from "./ReactionBar";

interface MessageItemProps {
	msg: MessageDTO;
	onReply?: (uuid: string) => void;
	onOpenThread?: (uuid: string) => void;
}

export default function MessageItem(props: MessageItemProps) {
	const msg = () => props.msg;

	const isOwn = () => {
		const u = userStore.user();
		if (!u) return false;
		return msg().author_id === u.name || msg().author_id === u.display_name;
	};

	const displayName = () => {
		const short =
			msg().author_id.length > 8
				? `${msg().author_id.slice(0, 8)}...`
				: msg().author_id;
		return short || "?";
	};

	const replyMessage = () =>
		msg().reply_to
			? chatStore.messages().find((m) => m.uuid === msg().reply_to)
			: undefined;

	const time = () => {
		try {
			const d = new Date(msg().created_at);
			return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
		} catch {
			return "";
		}
	};

	async function handleDelete() {
		chatStore.deleteMessage(msg().uuid);
	}

	if (msg().deleted) {
		return (
			<div class="px-3 py-2 text-sm italic text-base-content/40 select-none">
				[消息已删除]
			</div>
		);
	}

	return (
		<div class="group px-3 py-1.5 rounded-lg hover:bg-base-200 transition-colors">
			<Show when={replyMessage()}>
				{(reply) => (
					<button
						type="button"
						class="flex items-start gap-1 mb-1 w-full text-left border-l-2 border-base-300 pl-2 hover:bg-base-200/70 rounded-r-lg"
						onClick={() => props.onOpenThread?.(reply().uuid)}
					>
						<CornerDownRight class="size-3 shrink-0 mt-0.5 text-base-content/40" />
						<span class="text-[11px] text-base-content/50 truncate">
							<Show when={!reply().deleted}>
								<strong class="font-semibold">{reply().author_id}</strong>:{" "}
								{reply().content}
							</Show>
							<Show when={reply().deleted}>[消息已删除]</Show>
						</span>
					</button>
				)}
			</Show>

			<div class="flex items-center gap-2 mb-0.5">
				<div class="size-6 rounded-full bg-primary text-primary-content text-[11px] flex items-center justify-center font-bold shrink-0">
					{displayName().charAt(0).toUpperCase()}
				</div>
				<span class="text-sm font-semibold text-base-content leading-tight">
					{displayName()}
				</span>
				<span class="text-[11px] text-base-content/40">{time()}</span>
				<Show when={msg().edited_at}>
					<span class="text-[11px] text-base-content/40 italic">(edited)</span>
				</Show>
			</div>

			<div class="pl-8">
				<MarkdownText text={msg().content} />
			</div>

			<div class="opacity-0 group-hover:opacity-100 transition-opacity flex items-center gap-1 mt-0.5 pl-8">
				<ReactionBar roomUuid={msg().room_uuid} messageUuid={msg().uuid} />
				<button
					type="button"
					class="btn btn-ghost btn-xs px-1 min-h-0 h-[22px] text-[11px] text-base-content/50"
					onClick={() => props.onReply?.(msg().uuid)}
				>
					<MessageSquarePlus class="size-3" />
					回复
				</button>
				<button
					type="button"
					class="btn btn-ghost btn-xs px-1 min-h-0 h-[22px] text-[11px] text-base-content/50"
					onClick={() => props.onOpenThread?.(msg().uuid)}
				>
					线程
				</button>
				<Show when={isOwn()}>
					<button
						type="button"
						class="btn btn-ghost btn-xs px-1 min-h-0 h-[22px] text-[11px] text-base-content/50 hover:text-error"
						onClick={handleDelete}
					>
						删除
					</button>
				</Show>
			</div>
		</div>
	);
}
