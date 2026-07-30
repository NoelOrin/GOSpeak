import ChatWindow from "./chatWindow";
import ConversationList from "./conversationList";
import MemberSidebar from "./memberSidebar";

export default function ChatPage() {
	return (
		<div class="flex flex-row w-full h-full">
			<div class="w-56 shrink-0 border-r border-base-300 hidden sm:block">
				<ConversationList />
			</div>
			<div class="flex-1 min-w-0">
				<ChatWindow />
			</div>
			<div class="w-48 shrink-0 border-l border-base-300 hidden sm:block">
				<MemberSidebar />
			</div>
		</div>
	);
}
