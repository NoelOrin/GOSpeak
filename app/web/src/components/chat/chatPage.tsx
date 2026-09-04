import { Show } from "solid-js";
import ChatWindow from "@/components/chat/chatWindow";
import MemberSidebar from "@/components/chat/memberSidebar";
import { chatStore } from "@/stores/chatStore";

const ChatPage = () => {
	return (
		<Show
			when={true}
			fallback={
				<div class="flex justify-center items-center h-full text-sm text-base-content/40 select-none">
					加载中...
				</div>
			}
		>
			<div class="flex flex-row w-full h-full">
				<ChatWindow />
				<MemberSidebar
					onStartChat={(identity, _displayName) => {
						chatStore.startConversation(identity);
					}}
				/>
			</div>
		</Show>
	);
};

export default ChatPage;
