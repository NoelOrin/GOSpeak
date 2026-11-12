import { createFileRoute } from "@tanstack/react-router";
import { useEffect } from "react";
import ChatWindow from "@/components/chat/chatWindow";
import MemberSidebar from "@/components/chat/memberSidebar";
import { chatStore } from "@/stores/chatStore";

export const Route = createFileRoute("/(app)/chat")({
	component: ChatPage,
	staticData: { title: "聊天" },
});

export function ChatPage() {
	// 挂载时接入信令以获取服务器成员列表
	useEffect(() => {
		import("@/stores/socketStore").then(({ socketStore }) => {
			socketStore.connect();
		});
	}, []);

	return (
		<div className="flex h-full w-full flex-row">
			<ChatWindow />
			<MemberSidebar
				onStartChat={(identity) => {
					void chatStore.startConversation(identity);
				}}
			/>
		</div>
	);
}
