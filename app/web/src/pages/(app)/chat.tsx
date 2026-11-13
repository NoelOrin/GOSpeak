import { createFileRoute } from "@tanstack/react-router";
import { useEffect } from "react";
import ChatWindow from "@/components/chat/chatWindow";
import MemberSidebar from "@/components/chat/memberSidebar";
import { useKeepAliveActive } from "@/components/common/keepAliveOutlet";
import { chatStore } from "@/stores/chatStore";

export const Route = createFileRoute("/(app)/chat")({
	component: ChatPage,
	staticData: { title: "聊天" },
});

export function ChatPage() {
	const isActive = useKeepAliveActive();

	// 挂载时接入信令以获取服务器成员列表。keep-alive 下隐藏页的 effect 常驻，
	// 只在页面可见时保持连接，切走后断开，避免后台占用信令通道。
	useEffect(() => {
		if (!isActive) return;
		let cancelled = false;
		void import("@/stores/socketStore").then(({ socketStore }) => {
			if (!cancelled) socketStore.connect();
		});
		return () => {
			cancelled = true;
		};
	}, [isActive]);

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
