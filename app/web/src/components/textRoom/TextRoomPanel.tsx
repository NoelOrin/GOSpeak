import { Show } from "solid-js";
import { chatStore } from "@/stores/chatStore";
import MessageInput from "./MessageInput";
import MessageList from "./MessageList";

export default function TextRoomPanel() {
	const room = chatStore.textRoom;

	return (
		<Show
			when={room()}
			fallback={
				<div class="flex items-center justify-center h-full text-base-content/40 text-sm select-none">
					选择文字房间开始聊天
				</div>
			}
		>
			<div class="flex flex-col h-full">
				<MessageList />
				<MessageInput />
			</div>
		</Show>
	);
}
