import { useEffect, useState } from "react";
import { useChatStore } from "@/stores/chatStore";
import {
	activeTypingIdentities,
	roomTypingKey,
	useTypingStore,
} from "@/stores/typingStore";
import { useUserStore } from "@/stores/userStore";

/**
 * 当前文字房输入区上方的「谁正在输入」条。
 * 每秒 tick 一次只为让 TTL 过期的条目消失；状态本身由 typingStore 持有，
 * 不额外发任何网络请求。
 */
export default function TypingIndicator() {
	const byRoom = useTypingStore((s) => s.byRoom);
	const roomName = useChatStore((s) => s.textRoomName);
	const domainUUID = useChatStore((s) => s.textRoomDomainUUID);
	const self = useUserStore((s) => s.user?.name ?? "");
	const [now, setNow] = useState(() => Date.now());

	useEffect(() => {
		if (!roomName) return;
		const timer = window.setInterval(() => setNow(Date.now()), 1000);
		return () => window.clearInterval(timer);
	}, [roomName]);

	if (!roomName) return null;
	const names = activeTypingIdentities(
		byRoom,
		roomTypingKey(roomName, domainUUID),
		self,
		now,
	);
	if (names.length === 0) return null;

	const label =
		names.length === 1
			? `${names[0]} 正在输入…`
			: names.length <= 3
				? `${names.join("、")} 正在输入…`
				: `${names.length} 人正在输入…`;

	return (
		<div
			className="text-muted-foreground flex h-5 shrink-0 items-center gap-1.5 px-4 text-xs"
			aria-live="polite"
		>
			<span className="flex items-center gap-0.5" aria-hidden="true">
				<span className="bg-primary/70 size-1 animate-pulse rounded-full" />
				<span className="bg-primary/70 size-1 animate-pulse rounded-full [animation-delay:150ms]" />
				<span className="bg-primary/70 size-1 animate-pulse rounded-full [animation-delay:300ms]" />
			</span>
			<span className="truncate">{label}</span>
		</div>
	);
}
