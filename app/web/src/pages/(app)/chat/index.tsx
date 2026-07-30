import { createFileRoute } from "@tanstack/solid-router";
import ChatPage from "@/components/chat/chatPage";

export const Route = createFileRoute("/(app)/chat/")({
	component: RouteComponent,
	staticData: { title: "聊天", icon: "message-square" },
});

function RouteComponent() {
	return <ChatPage />;
}
