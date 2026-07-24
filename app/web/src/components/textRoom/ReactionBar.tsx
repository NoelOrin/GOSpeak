import { For } from "solid-js";
import { reactMessage } from "@/api/message";

const EMOJIS = ["👍", "❤️", "😂", "😮", "😢", "🙏"];

interface ReactionBarProps {
	roomUuid: string;
	messageUuid: string;
}

export default function ReactionBar(props: ReactionBarProps) {
	async function handleReact(emoji: string) {
		try {
			await reactMessage(props.roomUuid, props.messageUuid, emoji);
		} catch {
			// Reaction failure is non-critical, silently ignore
		}
	}

	return (
		<div class="flex items-center gap-0.5">
			<For each={EMOJIS}>
				{(emoji) => (
					<button
						type="button"
						class="btn btn-ghost btn-xs p-0 min-w-[22px] h-[22px] text-sm leading-none hover:bg-base-300 rounded"
						onClick={() => handleReact(emoji)}
						title={emoji}
					>
						{emoji}
					</button>
				)}
			</For>
		</div>
	);
}
