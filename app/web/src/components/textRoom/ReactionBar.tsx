import { reactMessage } from "@/api/message";
import { toastActionError } from "@/utils/toast";

const EMOJIS = ["👍", "❤️", "😂", "😮", "😢", "🙏"];

interface ReactionBarProps {
	roomUuid: string;
	messageUuid: string;
}

export default function ReactionBar({
	roomUuid,
	messageUuid,
}: ReactionBarProps) {
	async function handleReact(emoji: string) {
		try {
			await reactMessage(roomUuid, messageUuid, emoji);
		} catch (error) {
			toastActionError(error, "回应发送失败");
		}
	}

	return (
		<div className="flex items-center gap-0.5">
			{EMOJIS.map((emoji) => (
				<button
					key={emoji}
					type="button"
					className="hover:bg-muted inline-flex h-[22px] min-w-[22px] cursor-pointer items-center justify-center rounded-sm p-0 text-sm leading-none transition-colors"
					onClick={() => handleReact(emoji)}
					title={emoji}
				>
					{emoji}
				</button>
			))}
		</div>
	);
}
