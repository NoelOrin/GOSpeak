import { Show } from "solid-js";
import { chatStore } from "@/stores/chatStore";
import userStore from "@/stores/userStore";
import type { MessageDTO } from "@/types/message";
import ReactionBar from "./ReactionBar";

interface MessageItemProps {
	msg: MessageDTO;
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

	// Render deleted message placeholder
	if (msg().deleted) {
		return (
			<div class="px-3 py-2 text-sm italic text-base-content/40 select-none">
				[消息已删除]
			</div>
		);
	}

	return (
		<div class="group px-3 py-1.5 rounded-lg hover:bg-base-200 transition-colors">
			{/* Reply indicator */}
			<Show when={msg().reply_to}>
				<div class="flex items-center gap-1 mb-0.5">
					<div class="w-3 h-3 shrink-0 border-l-2 border-b-2 border-base-300 rounded-bl" />
					<span class="text-[11px] text-base-content/40 truncate">
						回复了一条消息
					</span>
				</div>
			</Show>

			{/* Header: avatar placeholder + name + timestamp */}
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

			{/* Content */}
			<div class="text-sm text-base-content break-words whitespace-pre-wrap pl-8">
				{msg().content}
			</div>

			{/* Hover actions */}
			<Show when={!msg().deleted}>
				<div class="opacity-0 group-hover:opacity-100 transition-opacity flex items-center gap-1 mt-0.5 pl-8">
					<ReactionBar roomUuid={msg().room_uuid} messageUuid={msg().uuid} />
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
			</Show>
		</div>
	);
}
